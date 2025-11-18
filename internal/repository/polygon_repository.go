package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ticket-service/internal/model"
)

type PolygonRepository struct {
	db *gorm.DB
}

func NewPolygonRepository(db *gorm.DB) *PolygonRepository {
	return &PolygonRepository{db: db}
}

func (r *PolygonRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Polygon, error) {
	var polygon model.Polygon
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&polygon).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &polygon, nil
}
