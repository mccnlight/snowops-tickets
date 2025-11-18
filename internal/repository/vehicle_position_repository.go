package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ticket-service/internal/model"
)

type VehiclePositionRepository struct {
	db *gorm.DB
}

func NewVehiclePositionRepository(db *gorm.DB) *VehiclePositionRepository {
	return &VehiclePositionRepository{db: db}
}

func (r *VehiclePositionRepository) GetLastByVehicleID(ctx context.Context, vehicleID uuid.UUID) (*model.VehiclePosition, error) {
	var pos model.VehiclePosition
	err := r.db.WithContext(ctx).
		Where("vehicle_id = ?", vehicleID).
		Order("recorded_at DESC").
		First(&pos).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &pos, nil
}
