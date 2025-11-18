package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ticket-service/internal/model"
)

type AssignmentRepository struct {
	db *gorm.DB
}

func NewAssignmentRepository(db *gorm.DB) *AssignmentRepository {
	return &AssignmentRepository{db: db}
}

func (r *AssignmentRepository) Create(ctx context.Context, assignment *model.TicketAssignment) error {
	return r.db.WithContext(ctx).Create(assignment).Error
}

func (r *AssignmentRepository) GetByID(ctx context.Context, id string) (*model.TicketAssignment, error) {
	var assignment model.TicketAssignment
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&assignment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &assignment, nil
}

func (r *AssignmentRepository) Update(ctx context.Context, assignment *model.TicketAssignment) error {
	return r.db.WithContext(ctx).Save(assignment).Error
}

func (r *AssignmentRepository) Delete(ctx context.Context, id string) error {
	// Мягкое удаление - помечаем как неактивное
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.TicketAssignment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_active":     false,
			"unassigned_at": now,
		}).Error
}

func (r *AssignmentRepository) ListByTicketID(ctx context.Context, ticketID uuid.UUID) ([]model.TicketAssignment, error) {
	var assignments []model.TicketAssignment
	err := r.db.WithContext(ctx).
		Where("ticket_id = ? AND is_active = ?", ticketID, true).
		Order("assigned_at DESC").
		Find(&assignments).Error
	return assignments, err
}

func (r *AssignmentRepository) UpdateDriverMarkStatus(ctx context.Context, id string, status model.DriverMarkStatus) error {
	return r.db.WithContext(ctx).Model(&model.TicketAssignment{}).
		Where("id = ?", id).
		Update("driver_mark_status", status).Error
}

func (r *AssignmentRepository) HasActiveAssignment(ctx context.Context, ticketID, driverID uuid.UUID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.TicketAssignment{}).
		Where("ticket_id = ? AND driver_id = ? AND is_active = ?", ticketID, driverID, true).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *AssignmentRepository) FindActiveByDriver(ctx context.Context, driverID uuid.UUID) (*model.TicketAssignment, error) {
	var assignment model.TicketAssignment
	err := r.db.WithContext(ctx).
		Where("driver_id = ? AND is_active = ?", driverID, true).
		Order("assigned_at DESC").
		First(&assignment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &assignment, nil
}

func (r *AssignmentRepository) FindActiveByVehicle(ctx context.Context, vehicleID uuid.UUID) (*model.TicketAssignment, error) {
	var assignment model.TicketAssignment
	err := r.db.WithContext(ctx).
		Where("vehicle_id = ? AND is_active = ?", vehicleID, true).
		Order("assigned_at DESC").
		First(&assignment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &assignment, nil
}
