package repository

import (
	"context"
	"errors"

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

