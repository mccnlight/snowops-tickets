package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TicketStatus string

const (
	TicketStatusPlanned    TicketStatus = "PLANNED"
	TicketStatusInProgress TicketStatus = "IN_PROGRESS"
	TicketStatusCompleted  TicketStatus = "COMPLETED"
	TicketStatusClosed     TicketStatus = "CLOSED"
	TicketStatusCancelled  TicketStatus = "CANCELLED"
)

type Ticket struct {
	ID             uuid.UUID    `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	CleaningAreaID uuid.UUID    `gorm:"type:uuid;not null;index" json:"cleaning_area_id"`
	ContractorID   uuid.UUID    `gorm:"type:uuid;not null;index" json:"contractor_id"`
	CreatedByOrgID uuid.UUID    `gorm:"type:uuid;not null;index" json:"created_by_org_id"`
	Status         TicketStatus `gorm:"type:ticket_status;not null;default:PLANNED" json:"status"`
	PlannedStartAt time.Time    `gorm:"not null" json:"planned_start_at"`
	PlannedEndAt   time.Time    `gorm:"not null" json:"planned_end_at"`
	FactStartAt    *time.Time   `json:"fact_start_at"`
	FactEndAt      *time.Time   `json:"fact_end_at"`
	Description    string       `gorm:"type:text" json:"description"`
	PhotoURL       *string      `gorm:"type:text" json:"photo_url"`
	Latitude       *float64     `json:"latitude"`
	Longitude      *float64     `json:"longitude"`
	CreatedAt      time.Time    `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time    `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Ticket) TableName() string {
	return "tickets"
}

func (t *Ticket) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

