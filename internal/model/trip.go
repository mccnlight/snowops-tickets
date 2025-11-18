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
	TripStatusForeignArea       TripStatus = "FOREIGN_AREA"
	TripStatusMismatchPlate     TripStatus = "MISMATCH_PLATE"
	TripStatusOverCapacity      TripStatus = "OVER_CAPACITY"
	TripStatusNoAreaWork        TripStatus = "NO_AREA_WORK"
	TripStatusNoAssignment      TripStatus = "NO_ASSIGNMENT"
	TripStatusSuspiciousVolume  TripStatus = "SUSPICIOUS_VOLUME"
	TripStatusNoExitCamera      TripStatus = "NO_EXIT_CAMERA"
	TripStatusOverContractLimit TripStatus = "OVER_CONTRACT_LIMIT"
)

type Trip struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TicketID            *uuid.UUID `gorm:"type:uuid;index" json:"ticket_id"`
	TicketAssignmentID  *uuid.UUID `gorm:"type:uuid;index" json:"ticket_assignment_id"`
	DriverID            *uuid.UUID `gorm:"type:uuid;index" json:"driver_id"`
	VehicleID           *uuid.UUID `gorm:"type:uuid;index" json:"vehicle_id"`
	VehicleBodyVolumeM3 *float64   `json:"vehicle_body_volume_m3"`
	CameraID            *uuid.UUID `gorm:"type:uuid" json:"camera_id"`
	PolygonID           *uuid.UUID `gorm:"type:uuid" json:"polygon_id"`
	VehiclePlateNumber  string     `gorm:"type:varchar(32)" json:"vehicle_plate_number"`
	DetectedPlateNumber string     `gorm:"type:varchar(32)" json:"detected_plate_number"`
	EntryLprEventID     *uuid.UUID `gorm:"type:uuid" json:"entry_lpr_event_id"`
	ExitLprEventID      *uuid.UUID `gorm:"type:uuid" json:"exit_lpr_event_id"`
	EntryVolumeEventID  *uuid.UUID `gorm:"type:uuid" json:"entry_volume_event_id"`
	ExitVolumeEventID   *uuid.UUID `gorm:"type:uuid" json:"exit_volume_event_id"`
	DetectedVolumeEntry *float64   `json:"detected_volume_entry"`
	DetectedVolumeExit  *float64   `json:"detected_volume_exit"`
	EntryAt             time.Time  `gorm:"not null" json:"entry_at"`
	ExitAt              *time.Time `json:"exit_at"`
	Status              TripStatus `gorm:"type:trip_status;not null;default:OK" json:"status"`
	ViolationReason     *string    `gorm:"column:violation_reason" json:"violation_reason,omitempty"`
	CreatedAt           time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
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
