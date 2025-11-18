package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ticket-service/internal/model"
)

type VehicleRepository struct {
	db *gorm.DB
}

func NewVehicleRepository(db *gorm.DB) *VehicleRepository {
	return &VehicleRepository{db: db}
}

func (r *VehicleRepository) GetByPlate(ctx context.Context, plate string) (*model.Vehicle, error) {
	if plate == "" {
		return nil, nil
	}
	var vehicle model.Vehicle
	err := r.db.WithContext(ctx).
		Where("plate_number = ?", plate).
		First(&vehicle).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &vehicle, nil
}

func (r *VehicleRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Vehicle, error) {
	var vehicle model.Vehicle
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&vehicle).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &vehicle, nil
}
