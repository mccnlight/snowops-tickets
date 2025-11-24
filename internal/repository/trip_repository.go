package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ticket-service/internal/model"
)

type TripRepository struct {
	db *gorm.DB
}

func NewTripRepository(db *gorm.DB) *TripRepository {
	return &TripRepository{db: db}
}

func (r *TripRepository) Create(ctx context.Context, trip *model.Trip) error {
	return r.db.WithContext(ctx).Create(trip).Error
}

func (r *TripRepository) GetByID(ctx context.Context, id string) (*model.Trip, error) {
	var trip model.Trip
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&trip).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &trip, nil
}

func (r *TripRepository) Update(ctx context.Context, trip *model.Trip) error {
	return r.db.WithContext(ctx).Save(trip).Error
}

func (r *TripRepository) ListByTicketID(ctx context.Context, ticketID uuid.UUID) ([]model.Trip, error) {
	var trips []model.Trip
	err := r.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("entry_at DESC").
		Find(&trips).Error
	return trips, err
}

func (r *TripRepository) ListByDriverID(ctx context.Context, driverID uuid.UUID, ticketID *uuid.UUID) ([]model.Trip, error) {
	var trips []model.Trip
	query := r.db.WithContext(ctx).Where("driver_id = ?", driverID)
	if ticketID != nil {
		query = query.Where("ticket_id = ?", *ticketID)
	}
	err := query.Order("entry_at DESC").Find(&trips).Error
	return trips, err
}

func (r *TripRepository) GetFirstTripByTicketID(ctx context.Context, ticketID uuid.UUID) (*model.Trip, error) {
	var trip model.Trip
	err := r.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("entry_at ASC").
		First(&trip).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &trip, nil
}

// ReceptionJournalEntry представляет запись в журнале приёма снега
type ReceptionJournalEntry struct {
	TripID              uuid.UUID  `json:"trip_id"`
	EntryAt             time.Time  `json:"entry_at"`
	ExitAt              *time.Time `json:"exit_at"`
	PolygonID           *uuid.UUID `json:"polygon_id"`
	PolygonName         *string    `json:"polygon_name"`
	VehiclePlateNumber  string     `json:"vehicle_plate_number"`
	DetectedPlateNumber string     `json:"detected_plate_number"`
	ContractorID        *uuid.UUID `json:"contractor_id"`
	ContractorName      *string    `json:"contractor_name"`
	DetectedVolumeEntry *float64   `json:"detected_volume_entry"`
	DetectedVolumeExit  *float64   `json:"detected_volume_exit"`
	NetVolumeM3         float64    `json:"net_volume_m3"`
	Status              string     `json:"status"`
}

// ReceptionJournalFilter фильтры для журнала приёма
type ReceptionJournalFilter struct {
	PolygonIDs   []uuid.UUID
	DateFrom     *time.Time
	DateTo       *time.Time
	ContractorID *uuid.UUID
	Status       *model.TripStatus
}

// ListReceptionJournal возвращает список рейсов для журнала приёма снега
func (r *TripRepository) ListReceptionJournal(ctx context.Context, filter ReceptionJournalFilter) ([]ReceptionJournalEntry, error) {
	query := r.db.WithContext(ctx).Table("trips tr").
		Select(`
			tr.id AS trip_id,
			tr.entry_at,
			tr.exit_at,
			tr.polygon_id,
			p.name AS polygon_name,
			tr.vehicle_plate_number,
			tr.detected_plate_number,
			t.contractor_id,
			contractor.name AS contractor_name,
			tr.detected_volume_entry,
			tr.detected_volume_exit,
			COALESCE(tr.detected_volume_entry, 0) - COALESCE(tr.detected_volume_exit, 0) AS net_volume_m3,
			tr.status::text AS status
		`).
		Joins("LEFT JOIN polygons p ON p.id = tr.polygon_id").
		Joins("LEFT JOIN tickets t ON t.id = tr.ticket_id").
		Joins("LEFT JOIN organizations contractor ON contractor.id = t.contractor_id").
		Where("tr.polygon_id IS NOT NULL")

	if len(filter.PolygonIDs) > 0 {
		query = query.Where("tr.polygon_id IN ?", filter.PolygonIDs)
	}

	if filter.DateFrom != nil {
		query = query.Where("tr.entry_at >= ?", *filter.DateFrom)
	}

	if filter.DateTo != nil {
		query = query.Where("tr.entry_at <= ?", *filter.DateTo)
	}

	if filter.ContractorID != nil {
		query = query.Where("t.contractor_id = ?", *filter.ContractorID)
	}

	if filter.Status != nil {
		query = query.Where("tr.status = ?", *filter.Status)
	}

	query = query.Order("tr.entry_at DESC")

	var entries []ReceptionJournalEntry
	if err := query.Scan(&entries).Error; err != nil {
		return nil, err
	}

	return entries, nil
}
