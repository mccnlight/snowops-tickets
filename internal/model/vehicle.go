package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Vehicle struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	ContractorID        uuid.UUID  `gorm:"type:uuid;not null;index" json:"contractor_id"`
	PlateNumber         string     `gorm:"type:varchar(32);uniqueIndex;not null" json:"plate_number"`
	BodyVolumeM3        *float64   `json:"body_volume_m3"`
	DefaultCleaningArea *uuid.UUID `gorm:"type:uuid" json:"default_cleaning_area"`
	DefaultPolygonID    *uuid.UUID `gorm:"type:uuid" json:"default_polygon_id"`
	DefaultDriverID     *uuid.UUID `gorm:"type:uuid" json:"default_driver_id"`
	CreatedAt           time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Vehicle) TableName() string {
	return "vehicles"
}

func (v *Vehicle) BeforeCreate(tx *gorm.DB) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	return nil
}
