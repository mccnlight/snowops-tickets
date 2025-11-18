package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CleaningArea struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	Name           string    `gorm:"type:varchar(255)" json:"name"`
	DefaultPolygon uuid.UUID `gorm:"type:uuid" json:"default_polygon"`
	EntryLat       *float64  `json:"entry_lat"`
	EntryLong      *float64  `json:"entry_long"`
	ExitLat        *float64  `json:"exit_lat"`
	ExitLong       *float64  `json:"exit_long"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (CleaningArea) TableName() string {
	return "cleaning_areas"
}

func (c *CleaningArea) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
