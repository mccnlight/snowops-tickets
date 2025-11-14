package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"ticket-service/internal/model"
)

type TicketRepository struct {
	db *gorm.DB
}

func NewTicketRepository(db *gorm.DB) *TicketRepository {
	return &TicketRepository{db: db}
}

func (r *TicketRepository) Create(ctx context.Context, ticket *model.Ticket) error {
	return r.db.WithContext(ctx).Create(ticket).Error
}

func (r *TicketRepository) GetByID(ctx context.Context, id string) (*model.Ticket, error) {
	var ticket model.Ticket
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&ticket).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return &ticket, nil
}

func (r *TicketRepository) Update(ctx context.Context, ticket *model.Ticket) error {
	return r.db.WithContext(ctx).Save(ticket).Error
}

type TicketListFilter struct {
	Status         *model.TicketStatus
	ContractorID   *string
	CleaningAreaID *string
	CreatedByOrgID *string
	DriverID       *string
}

func (r *TicketRepository) List(ctx context.Context, filter TicketListFilter) ([]model.Ticket, error) {
	var tickets []model.Ticket
	query := r.db.WithContext(ctx).Model(&model.Ticket{})

	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.ContractorID != nil {
		query = query.Where("contractor_id = ?", *filter.ContractorID)
	}
	if filter.CleaningAreaID != nil {
		query = query.Where("cleaning_area_id = ?", *filter.CleaningAreaID)
	}
	if filter.CreatedByOrgID != nil {
		query = query.Where("created_by_org_id = ?", *filter.CreatedByOrgID)
	}
	if filter.DriverID != nil {
		query = query.Joins("JOIN ticket_assignments ta ON ta.ticket_id = tickets.id").
			Where("ta.driver_id = ? AND ta.is_active = ?", *filter.DriverID, true)
	}

	if err := query.Order("created_at DESC").Find(&tickets).Error; err != nil {
		return nil, err
	}

	return tickets, nil
}
