package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ticket-service/internal/model"
	"ticket-service/internal/repository"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrPermissionDenied = errors.New("permission denied")
	ErrInvalidInput     = errors.New("invalid input")
	ErrConflict         = errors.New("conflict")
)

type TicketService struct {
	ticketRepo *repository.TicketRepository
}

func NewTicketService(ticketRepo *repository.TicketRepository) *TicketService {
	return &TicketService{
		ticketRepo: ticketRepo,
	}
}

func (s *TicketService) Create(ctx context.Context, principal model.Principal, input CreateTicketInput) (*model.Ticket, error) {
	if !principal.IsAkimat() && !principal.IsKgu() {
		return nil, ErrPermissionDenied
	}

	cleaningAreaID, err := uuid.Parse(input.CleaningAreaID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	contractorID, err := uuid.Parse(input.ContractorID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	plannedStartAt, err := time.Parse(time.RFC3339, input.PlannedStartAt)
	if err != nil {
		return nil, ErrInvalidInput
	}

	plannedEndAt, err := time.Parse(time.RFC3339, input.PlannedEndAt)
	if err != nil {
		return nil, ErrInvalidInput
	}

	if plannedEndAt.Before(plannedStartAt) || plannedEndAt.Equal(plannedStartAt) {
		return nil, ErrInvalidInput
	}

	ticket := &model.Ticket{
		CleaningAreaID: cleaningAreaID,
		ContractorID:   contractorID,
		CreatedByOrgID: principal.OrgID,
		Status:         model.TicketStatusPlanned,
		PlannedStartAt: plannedStartAt,
		PlannedEndAt:   plannedEndAt,
		Description:    input.Description,
	}

	if err := s.ticketRepo.Create(ctx, ticket); err != nil {
		return nil, err
	}

	return ticket, nil
}

type CreateTicketInput struct {
	CleaningAreaID string
	ContractorID   string
	PlannedStartAt string
	PlannedEndAt   string
	Description    string
}

func (s *TicketService) Get(ctx context.Context, principal model.Principal, id string) (*model.Ticket, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if !s.canAccessTicket(principal, ticket) {
		return nil, ErrPermissionDenied
	}

	return ticket, nil
}

func (s *TicketService) List(ctx context.Context, principal model.Principal, filter repository.TicketListFilter) ([]model.Ticket, error) {
	if principal.IsAkimat() {
		// Акимат видит все
	} else if principal.IsKgu() {
		orgID := principal.OrgID.String()
		filter.CreatedByOrgID = &orgID
	} else if principal.IsContractor() {
		contractorID := principal.OrgID.String()
		filter.ContractorID = &contractorID
	} else if principal.IsDriver() {
		if principal.DriverID == nil {
			return nil, ErrPermissionDenied
		}
		driverID := principal.DriverID.String()
		filter.DriverID = &driverID
	}

	return s.ticketRepo.List(ctx, filter)
}

func (s *TicketService) canAccessTicket(principal model.Principal, ticket *model.Ticket) bool {
	if principal.IsAkimat() {
		return true
	}
	if principal.IsKgu() {
		return ticket.CreatedByOrgID == principal.OrgID
	}
	if principal.IsContractor() {
		return ticket.ContractorID == principal.OrgID
	}
	if principal.IsDriver() && principal.DriverID != nil {
		// Проверка через назначения будет в репозитории
		return true
	}
	return false
}
