package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TripStatus string

const (
	TripStatusPending    TripStatus = "PENDING"
	TripStatusInProgress TripStatus = "IN_PROGRESS"
	TripStatusCompleted  TripStatus = "COMPLETED"
	TripStatusCancelled  TripStatus = "CANCELLED"
)

type ViolationType string

const (
	ViolationTypeRouteViolation ViolationType = "ROUTE_VIOLATION"
	ViolationTypeMismatchPlate  ViolationType = "MISMATCH_PLATE"
	ViolationTypeNoAssignment   ViolationType = "NO_ASSIGNMENT"
	ViolationTypeSuspiciousVolume ViolationType = "SUSPICIOUS_VOLUME"
)

type Trip struct {
	ID                  uuid.UUID     `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TicketID            *uuid.UUID    `gorm:"type:uuid;index" json:"ticket_id"`
	TicketAssignmentID  *uuid.UUID    `gorm:"type:uuid;index" json:"ticket_assignment_id"`
	CameraID            *uuid.UUID    `gorm:"type:uuid" json:"camera_id"`
	PolygonID           *uuid.UUID    `gorm:"type:uuid" json:"polygon_id"`
	VehiclePlateNumber  string        `gorm:"type:varchar(32)" json:"vehicle_plate_number"`
	DetectedPlateNumber string        `gorm:"type:varchar(32)" json:"detected_plate_number"`
	DetectedVolumeEntry *float64      `json:"detected_volume_entry"`
	DetectedVolumeExit  *float64      `json:"detected_volume_exit"`
	EntryAt             time.Time     `gorm:"not null" json:"entry_at"`
	ExitAt              *time.Time    `json:"exit_at"`
	Status              TripStatus    `gorm:"type:trip_status;not null;default:PENDING" json:"status"`
	HasViolations       bool          `gorm:"not null;default:false" json:"has_violations"`
	ViolationType       *ViolationType `gorm:"type:violation_type" json:"violation_type"`
	CreatedAt           time.Time     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time     `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Trip) TableName() string {
	return "trips"
}

func (t *Trip) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

