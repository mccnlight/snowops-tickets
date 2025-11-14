package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LprEvent struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CameraID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"camera_id"`
	PolygonID   *uuid.UUID `gorm:"type:uuid" json:"polygon_id"`
	PlateNumber string     `gorm:"type:varchar(32);not null;index" json:"plate_number"`
	DetectedAt  time.Time  `gorm:"not null;index" json:"detected_at"`
	Direction   *string    `gorm:"type:varchar(20)" json:"direction"`
	Confidence  *float64   `json:"confidence"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (LprEvent) TableName() string {
	return "lpr_events"
}

func (e *LprEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

type VolumeEvent struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CameraID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"camera_id"`
	PolygonID     *uuid.UUID `gorm:"type:uuid" json:"polygon_id"`
	DetectedVolume float64   `gorm:"not null" json:"detected_volume"`
	DetectedAt    time.Time  `gorm:"not null;index" json:"detected_at"`
	Direction     *string    `gorm:"type:varchar(20)" json:"direction"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (VolumeEvent) TableName() string {
	return "volume_events"
}

func (e *VolumeEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

