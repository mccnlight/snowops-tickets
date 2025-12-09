package repository

import (
	"context"
	"errors"
	"fmt"
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

// UpdateTripStartedAt обновляет время начала рейса и статус
func (r *AssignmentRepository) UpdateTripStartedAt(ctx context.Context, id string, startedAt time.Time, status model.DriverMarkStatus) error {
	return r.db.WithContext(ctx).Model(&model.TicketAssignment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"trip_started_at":    startedAt,
			"driver_mark_status": status,
		}).Error
}

// UpdateTripFinishedAt обновляет время окончания рейса и статус
func (r *AssignmentRepository) UpdateTripFinishedAt(ctx context.Context, id string, finishedAt time.Time, status model.DriverMarkStatus) error {
	return r.db.WithContext(ctx).Model(&model.TicketAssignment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"trip_finished_at":   finishedAt,
			"driver_mark_status": status,
		}).Error
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

// GetVehiclePlateNumber получает номер машины по vehicle_id из таблицы vehicles
// Таблица vehicles может быть в общей БД или в другом сервисе
func (r *AssignmentRepository) GetVehiclePlateNumber(ctx context.Context, vehicleID uuid.UUID) (string, error) {
	var result struct {
		PlateNumber string `gorm:"column:plate_number"`
	}

	err := r.db.WithContext(ctx).
		Table("vehicles").
		Select("plate_number").
		Where("id = ?", vehicleID).
		First(&result).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("vehicle with id %s not found", vehicleID)
		}
		return "", fmt.Errorf("failed to get vehicle plate number: %w", err)
	}

	return result.PlateNumber, nil
}
