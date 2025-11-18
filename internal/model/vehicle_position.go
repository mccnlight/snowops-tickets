package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type VehiclePosition struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	VehicleID          uuid.UUID `gorm:"type:uuid;not null;index" json:"vehicle_id"`
	Latitude           float64   `json:"latitude"`
	Longitude          float64   `json:"longitude"`
	RecordedAt         time.Time `gorm:"index;not null" json:"recorded_at"`
	InsideCleaningArea bool      `json:"inside_cleaning_area"`
	InsidePolygon      bool      `json:"inside_polygon"`
}

func (VehiclePosition) TableName() string {
	return "vehicle_positions"
}

func (p *VehiclePosition) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
