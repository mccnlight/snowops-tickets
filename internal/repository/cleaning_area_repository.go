package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ticket-service/internal/model"
)

type CleaningAreaRepository struct {
	db *gorm.DB
}

func NewCleaningAreaRepository(db *gorm.DB) *CleaningAreaRepository {
	return &CleaningAreaRepository{db: db}
}

func (r *CleaningAreaRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.CleaningArea, error) {
	var area model.CleaningArea
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&area).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &area, nil
}
