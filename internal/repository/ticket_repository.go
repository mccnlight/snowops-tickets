package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ticket-service/internal/model"
)

type TicketRepository struct {
	db *gorm.DB
}

func NewTicketRepository(db *gorm.DB) *TicketRepository {
	return &TicketRepository{db: db}
}

func (r *TicketRepository) Create(ctx context.Context, ticket *model.Ticket) error {
	return r.db.WithContext(ctx).Create(ticket).Error
}

func (r *TicketRepository) GetByID(ctx context.Context, id string) (*model.Ticket, error) {
	var ticket model.Ticket
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&ticket).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &ticket, nil
}

func (r *TicketRepository) Update(ctx context.Context, ticket *model.Ticket) error {
	return r.db.WithContext(ctx).Save(ticket).Error
}

func (r *TicketRepository) CountTripsByTicketID(ctx context.Context, ticketID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Trip{}).
		Where("ticket_id = ?", ticketID).Count(&count).Error
	return count, err
}

func (r *TicketRepository) CountIncompleteTripsByTicketID(ctx context.Context, ticketID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Trip{}).
		Where("ticket_id = ? AND exit_at IS NULL", ticketID).
		Count(&count).Error
	return count, err
}

func (r *TicketRepository) CountIncompleteAssignmentsByTicketID(ctx context.Context, ticketID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.TicketAssignment{}).
		Where("ticket_id = ? AND is_active = ? AND driver_mark_status != ?",
			ticketID, true, model.DriverMarkStatusCompleted).
		Count(&count).Error
	return count, err
}

func (r *TicketRepository) CountTripsWithInvalidExitVolume(ctx context.Context, ticketID uuid.UUID, tolerance float64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Trip{}).
		Where("ticket_id = ? AND exit_volume_event_id IS NOT NULL", ticketID).
		Where("(detected_volume_exit IS NULL OR ABS(detected_volume_exit) > ?)", tolerance).
		Count(&count).Error
	return count, err
}

type TicketListFilter struct {
	Status           *model.TicketStatus
	ContractorID     *string
	CleaningAreaID   *string
	ContractID       *string
	CreatedByOrgID   *string
	DriverID         *string
	PlannedStartFrom *string
	PlannedStartTo   *string
	PlannedEndFrom   *string
	PlannedEndTo     *string
	FactStartFrom    *string
	FactStartTo      *string
	FactEndFrom      *string
	FactEndTo        *string
}

func (r *TicketRepository) List(ctx context.Context, filter TicketListFilter) ([]model.Ticket, error) {
	var tickets []model.Ticket
	query := r.db.WithContext(ctx).Model(&model.Ticket{})

	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.ContractorID != nil {
		query = query.Where("contractor_id = ?", *filter.ContractorID)
	}
	if filter.CleaningAreaID != nil {
		query = query.Where("cleaning_area_id = ?", *filter.CleaningAreaID)
	}
	if filter.ContractID != nil {
		query = query.Where("contract_id = ?", *filter.ContractID)
	}
	if filter.CreatedByOrgID != nil {
		query = query.Where("created_by_org_id = ?", *filter.CreatedByOrgID)
	}
	if filter.DriverID != nil {
		query = query.Joins("JOIN ticket_assignments ta ON ta.ticket_id = tickets.id").
			Where("ta.driver_id = ? AND ta.is_active = ?", *filter.DriverID, true)
	}
	if filter.PlannedStartFrom != nil {
		query = query.Where("planned_start_at >= ?", *filter.PlannedStartFrom)
	}
	if filter.PlannedStartTo != nil {
		query = query.Where("planned_start_at <= ?", *filter.PlannedStartTo)
	}
	if filter.PlannedEndFrom != nil {
		query = query.Where("planned_end_at >= ?", *filter.PlannedEndFrom)
	}
	if filter.PlannedEndTo != nil {
		query = query.Where("planned_end_at <= ?", *filter.PlannedEndTo)
	}
	if filter.FactStartFrom != nil {
		query = query.Where("fact_start_at >= ?", *filter.FactStartFrom)
	}
	if filter.FactStartTo != nil {
		query = query.Where("fact_start_at <= ?", *filter.FactStartTo)
	}
	if filter.FactEndFrom != nil {
		query = query.Where("fact_end_at >= ?", *filter.FactEndFrom)
	}
	if filter.FactEndTo != nil {
		query = query.Where("fact_end_at <= ?", *filter.FactEndTo)
	}

	if err := query.Order("created_at DESC").Find(&tickets).Error; err != nil {
		return nil, err
	}

	return tickets, nil
}

// TicketMetrics stores aggregated ticket data
type TicketMetrics struct {
	TotalTrips    int64   `json:"total_trips"`
	TotalVolumeM3 float64 `json:"total_volume_m3"`
	HasViolations bool    `json:"has_violations"`
}

// GetTicketMetrics calculates metrics for a single ticket
func (r *TicketRepository) GetTicketMetrics(ctx context.Context, ticketID uuid.UUID) (*TicketMetrics, error) {
	var metrics TicketMetrics

	// Count trips
	if err := r.db.WithContext(ctx).Model(&model.Trip{}).
		Where("ticket_id = ?", ticketID).Count(&metrics.TotalTrips).Error; err != nil {
		return nil, err
	}

	// Calculate total exported volume (sum of detected_volume_entry)
	var totalVolume *float64
	if err := r.db.WithContext(ctx).Model(&model.Trip{}).
		Select("COALESCE(SUM(detected_volume_entry), 0)").
		Where("ticket_id = ?", ticketID).
		Scan(&totalVolume).Error; err != nil {
		return nil, err
	}
	if totalVolume != nil {
		metrics.TotalVolumeM3 = *totalVolume
	}

	// Determine if there are any non-OK trips
	var violationsCount int64
	if err := r.db.WithContext(ctx).Model(&model.Trip{}).
		Where("ticket_id = ? AND status != ?", ticketID, model.TripStatusOK).
		Count(&violationsCount).Error; err != nil {
		return nil, err
	}
	metrics.HasViolations = violationsCount > 0

	return &metrics, nil
}

func (r *TicketRepository) GetMetricsForTickets(ctx context.Context, ticketIDs []uuid.UUID) (map[uuid.UUID]*TicketMetrics, error) {
	result := make(map[uuid.UUID]*TicketMetrics, len(ticketIDs))
	if len(ticketIDs) == 0 {
		return result, nil
	}

	type row struct {
		TicketID       uuid.UUID
		TotalTrips     int64
		TotalVolumeM3  float64
		ViolationCount int64
	}

	var rows []row
	err := r.db.WithContext(ctx).Model(&model.Trip{}).
		Select("ticket_id, COUNT(*) AS total_trips, COALESCE(SUM(detected_volume_entry), 0) AS total_volume_m3, SUM(CASE WHEN status != ? THEN 1 ELSE 0 END) AS violation_count", model.TripStatusOK).
		Where("ticket_id IN ?", ticketIDs).
		Group("ticket_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	for _, data := range rows {
		result[data.TicketID] = &TicketMetrics{
			TotalTrips:    data.TotalTrips,
			TotalVolumeM3: data.TotalVolumeM3,
			HasViolations: data.ViolationCount > 0,
		}
	}

	return result, nil
}

// GetAssignmentsByTicketID returns active assignments
func (r *TicketRepository) GetAssignmentsByTicketID(ctx context.Context, ticketID uuid.UUID) ([]model.TicketAssignment, error) {
	var assignments []model.TicketAssignment
	err := r.db.WithContext(ctx).
		Where("ticket_id = ? AND is_active = ?", ticketID, true).
		Order("assigned_at DESC").
		Find(&assignments).Error
	return assignments, err
}

// GetTripsByTicketID returns trips for a ticket
func (r *TicketRepository) GetTripsByTicketID(ctx context.Context, ticketID uuid.UUID) ([]model.Trip, error) {
	var trips []model.Trip
	err := r.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("entry_at DESC").
		Find(&trips).Error
	return trips, err
}

// GetAppealsByTicketID returns appeals for a ticket
func (r *TicketRepository) GetAppealsByTicketID(ctx context.Context, ticketID uuid.UUID) ([]model.Appeal, error) {
	var appeals []model.Appeal
	err := r.db.WithContext(ctx).
		Where("ticket_id = ?", ticketID).
		Order("created_at DESC").
		Find(&appeals).Error
	return appeals, err
}

func (r *TicketRepository) FindActiveTicketForVehicle(ctx context.Context, contractorID uuid.UUID, cleaningAreaID *uuid.UUID) (*model.Ticket, error) {
	var ticket model.Ticket
	query := r.db.WithContext(ctx).
		Where("contractor_id = ? AND status IN ?", contractorID, []model.TicketStatus{
			model.TicketStatusPlanned,
			model.TicketStatusInProgress,
		}).
		Order("planned_start_at ASC")

	if cleaningAreaID != nil {
		query = query.Where("cleaning_area_id = ?", *cleaningAreaID)
	}

	err := query.First(&ticket).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &ticket, nil
}
