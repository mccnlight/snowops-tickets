package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TripStatus string

const (
	TripStatusOK                TripStatus = "OK"
	TripStatusRouteViolation    TripStatus = "ROUTE_VIOLATION"
	TripStatusMismatchPlate     TripStatus = "MISMATCH_PLATE"
	TripStatusNoAssignment      TripStatus = "NO_ASSIGNMENT"
	TripStatusSuspiciousVolume  TripStatus = "SUSPICIOUS_VOLUME"
)

type Trip struct {
	ID                  uuid.UUID    `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TicketID            *uuid.UUID   `gorm:"type:uuid;index" json:"ticket_id"`
	TicketAssignmentID  *uuid.UUID   `gorm:"type:uuid;index" json:"ticket_assignment_id"`
	DriverID            *uuid.UUID   `gorm:"type:uuid;index" json:"driver_id"`
	VehicleID           *uuid.UUID   `gorm:"type:uuid;index" json:"vehicle_id"`
	CameraID            *uuid.UUID   `gorm:"type:uuid" json:"camera_id"`
	PolygonID           *uuid.UUID   `gorm:"type:uuid" json:"polygon_id"`
	VehiclePlateNumber  string       `gorm:"type:varchar(32)" json:"vehicle_plate_number"`
	DetectedPlateNumber string       `gorm:"type:varchar(32)" json:"detected_plate_number"`
	EntryLprEventID     *uuid.UUID   `gorm:"type:uuid" json:"entry_lpr_event_id"`
	ExitLprEventID      *uuid.UUID   `gorm:"type:uuid" json:"exit_lpr_event_id"`
	EntryVolumeEventID  *uuid.UUID   `gorm:"type:uuid" json:"entry_volume_event_id"`
	ExitVolumeEventID   *uuid.UUID   `gorm:"type:uuid" json:"exit_volume_event_id"`
	DetectedVolumeEntry *float64     `json:"detected_volume_entry"`
	DetectedVolumeExit  *float64     `json:"detected_volume_exit"`
	EntryAt             time.Time    `gorm:"not null" json:"entry_at"`
	ExitAt              *time.Time   `json:"exit_at"`
	Status              TripStatus   `gorm:"type:trip_status;not null;default:OK" json:"status"`
	CreatedAt           time.Time    `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time    `gorm:"autoUpdateTime" json:"updated_at"`
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

