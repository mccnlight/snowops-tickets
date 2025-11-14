package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AppealStatus string

const (
	AppealStatusNeedInfo   AppealStatus = "NEED_INFO"
	AppealStatusUnderReview AppealStatus = "UNDER_REVIEW"
	AppealStatusApproved   AppealStatus = "APPROVED"
	AppealStatusRejected   AppealStatus = "REJECTED"
	AppealStatusClosed     AppealStatus = "CLOSED"
)

type Appeal struct {
	ID             uuid.UUID    `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TripID         *uuid.UUID   `gorm:"type:uuid;index" json:"trip_id"`
	TicketID       *uuid.UUID   `gorm:"type:uuid;index" json:"ticket_id"`
	CreatedByUserID uuid.UUID  `gorm:"type:uuid;not null" json:"created_by_user_id"`
	Status         AppealStatus `gorm:"type:appeal_status;not null;default:NEED_INFO" json:"status"`
	Reason         string       `gorm:"type:text;not null" json:"reason"`
	AdminResponse  *string      `gorm:"type:text" json:"admin_response"`
	ResolvedAt     *time.Time   `json:"resolved_at"`
	CreatedAt      time.Time    `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time    `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Appeal) TableName() string {
	return "appeals"
}

func (a *Appeal) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

type AppealComment struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	AppealID       uuid.UUID `gorm:"type:uuid;not null;index" json:"appeal_id"`
	CreatedByUserID uuid.UUID `gorm:"type:uuid;not null" json:"created_by_user_id"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (AppealComment) TableName() string {
	return "appeal_comments"
}

func (ac *AppealComment) BeforeCreate(tx *gorm.DB) error {
	if ac.ID == uuid.Nil {
		ac.ID = uuid.New()
	}
	return nil
}

