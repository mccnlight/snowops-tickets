package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DriverMarkStatus string

const (
	DriverMarkStatusNotStarted DriverMarkStatus = "NOT_STARTED"
	DriverMarkStatusInWork     DriverMarkStatus = "IN_WORK"
	DriverMarkStatusCompleted  DriverMarkStatus = "COMPLETED"
)

type TicketAssignment struct {
	ID               uuid.UUID         `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TicketID         uuid.UUID         `gorm:"type:uuid;not null;index" json:"ticket_id"`
	DriverID         uuid.UUID         `gorm:"type:uuid;not null;index" json:"driver_id"`
	VehicleID        uuid.UUID         `gorm:"type:uuid;not null;index" json:"vehicle_id"`
	DriverMarkStatus DriverMarkStatus  `gorm:"type:driver_mark_status;not null;default:NOT_STARTED" json:"driver_mark_status"`
	AssignedAt       time.Time         `gorm:"not null;default:now()" json:"assigned_at"`
	UnassignedAt     *time.Time        `json:"unassigned_at"`
	IsActive         bool              `gorm:"not null;default:true" json:"is_active"`
	CreatedAt        time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
}

func (TicketAssignment) TableName() string {
	return "ticket_assignments"
}

func (ta *TicketAssignment) BeforeCreate(tx *gorm.DB) error {
	if ta.ID == uuid.Nil {
		ta.ID = uuid.New()
	}
	if ta.AssignedAt.IsZero() {
		ta.AssignedAt = time.Now()
	}
	return nil
}

