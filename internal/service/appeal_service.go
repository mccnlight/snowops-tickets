package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"ticket-service/internal/model"
	"ticket-service/internal/repository"
)

type AppealService struct {
	appealRepo     *repository.AppealRepository
	tripRepo       *repository.TripRepository
	ticketRepo     *repository.TicketRepository
	assignmentRepo *repository.AssignmentRepository
}

func NewAppealService(
	appealRepo *repository.AppealRepository,
	tripRepo *repository.TripRepository,
	ticketRepo *repository.TicketRepository,
	assignmentRepo *repository.AssignmentRepository,
) *AppealService {
	return &AppealService{
		appealRepo:     appealRepo,
		tripRepo:       tripRepo,
		ticketRepo:     ticketRepo,
		assignmentRepo: assignmentRepo,
	}
}

type CreateAppealInput struct {
	TripID           string
	AppealReasonType string
	Comment          string
}

func (s *AppealService) Create(ctx context.Context, principal model.Principal, input CreateAppealInput) (*model.Appeal, error) {
	// Только водитель может создавать обжалования
	if !principal.IsDriver() {
		return nil, ErrPermissionDenied
	}

	// Валидация типа основания
	validReasonTypes := map[string]bool{
		string(model.AppealReasonTypeCameraError):     true,
		string(model.AppealReasonTypeTransit):         true,
		string(model.AppealReasonTypeOtherAssignment): true,
		string(model.AppealReasonTypeOther):           true,
	}
	if !validReasonTypes[input.AppealReasonType] {
		return nil, ErrInvalidInput
	}

	tripID, err := uuid.Parse(input.TripID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	// Проверяем, что рейс существует и принадлежит водителю
	trip, err := s.tripRepo.GetByID(ctx, input.TripID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if trip.DriverID == nil || *trip.DriverID != *principal.DriverID {
		return nil, ErrPermissionDenied
	}

	// Можно обжаловать только рейсы с нарушениями
	if trip.Status == model.TripStatusOK {
		return nil, ErrConflict
	}

	var ticketID *uuid.UUID
	if trip.TicketID != nil {
		ticketID = trip.TicketID
	}

	appealReasonType := input.AppealReasonType
	appeal := &model.Appeal{
		TripID:           &tripID,
		TicketID:         ticketID,
		CreatedByUserID:  principal.UserID,
		Status:           model.AppealStatusSubmitted,
		Reason:           string(trip.Status), // Нарушение из статуса рейса
		AppealReasonType: &appealReasonType,
		Comment:          input.Comment,
	}

	if err := s.appealRepo.Create(ctx, appeal); err != nil {
		return nil, err
	}

	return appeal, nil
}

func (s *AppealService) ListByTicketID(ctx context.Context, principal model.Principal, ticketID string) ([]model.Appeal, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Проверяем права доступа
	if principal.IsAkimat() {
		// Акимат видит все
	} else if principal.IsKgu() {
		if ticket.CreatedByOrgID != principal.OrgID {
			return nil, ErrPermissionDenied
		}
	} else if principal.IsContractor() {
		if ticket.ContractorID != principal.OrgID {
			return nil, ErrPermissionDenied
		}
	} else if principal.IsDriver() {
		// Водитель видит только свои обжалования
		return s.appealRepo.ListByUserID(ctx, principal.UserID)
	} else {
		return nil, ErrPermissionDenied
	}

	return s.appealRepo.ListByTicketID(ctx, ticket.ID)
}

func (s *AppealService) ListDriverAppeals(ctx context.Context, principal model.Principal, ticketID *string) ([]model.Appeal, error) {
	if !principal.IsDriver() {
		return nil, ErrPermissionDenied
	}

	if ticketID != nil && *ticketID != "" {
		if principal.DriverID == nil {
			return nil, ErrPermissionDenied
		}
		tid, err := uuid.Parse(*ticketID)
		if err != nil {
			return nil, ErrInvalidInput
		}

		has, err := s.assignmentRepo.HasActiveAssignment(ctx, tid, *principal.DriverID)
		if err != nil {
			return nil, err
		}
		if !has {
			return nil, ErrPermissionDenied
		}

		appeals, err := s.appealRepo.ListByTicketID(ctx, tid)
		if err != nil {
			return nil, err
		}

		var filtered []model.Appeal
		for _, a := range appeals {
			if a.CreatedByUserID == principal.UserID {
				filtered = append(filtered, a)
			}
		}
		return filtered, nil
	}

	return s.appealRepo.ListByUserID(ctx, principal.UserID)
}

func (s *AppealService) GetByID(ctx context.Context, principal model.Principal, id string) (*model.Appeal, error) {
	appeal, err := s.appealRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Проверяем права доступа
	if principal.IsAkimat() {
		// Акимат видит все
	} else if principal.IsKgu() {
		if appeal.TicketID != nil {
			ticket, err := s.ticketRepo.GetByID(ctx, appeal.TicketID.String())
			if err != nil {
				return nil, err
			}
			if ticket.CreatedByOrgID != principal.OrgID {
				return nil, ErrPermissionDenied
			}
		}
	} else if principal.IsContractor() {
		if appeal.TicketID != nil {
			ticket, err := s.ticketRepo.GetByID(ctx, appeal.TicketID.String())
			if err != nil {
				return nil, err
			}
			if ticket.ContractorID != principal.OrgID {
				return nil, ErrPermissionDenied
			}
		}
	} else if principal.IsDriver() {
		if appeal.CreatedByUserID != principal.UserID {
			return nil, ErrPermissionDenied
		}
	} else {
		return nil, ErrPermissionDenied
	}

	return appeal, nil
}

func (s *AppealService) UpdateStatus(ctx context.Context, principal model.Principal, id string, status model.AppealStatus, adminResponse *string) error {
	// Только KGU ZKH и Акимат могут обновлять статус обжалования
	if !principal.IsKgu() && !principal.IsAkimat() {
		return ErrPermissionDenied
	}

	appeal, err := s.appealRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	// Проверяем права доступа
	if principal.IsKgu() && appeal.TicketID != nil {
		ticket, err := s.ticketRepo.GetByID(ctx, appeal.TicketID.String())
		if err != nil {
			return err
		}
		if ticket.CreatedByOrgID != principal.OrgID {
			return ErrPermissionDenied
		}
	}

	appeal.Status = status
	if adminResponse != nil {
		appeal.AdminResponse = adminResponse
	}

	return s.appealRepo.Update(ctx, appeal)
}

func (s *AppealService) AddComment(ctx context.Context, principal model.Principal, appealID string, content string) error {
	appeal, err := s.appealRepo.GetByID(ctx, appealID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	// Проверяем права доступа
	if principal.IsDriver() {
		if appeal.CreatedByUserID != principal.UserID {
			return ErrPermissionDenied
		}
	} else if principal.IsKgu() || principal.IsAkimat() {
		// KGU ZKH и Акимат могут комментировать
	} else if principal.IsContractor() {
		if appeal.TicketID != nil {
			ticket, err := s.ticketRepo.GetByID(ctx, appeal.TicketID.String())
			if err != nil {
				return err
			}
			if ticket.ContractorID != principal.OrgID {
				return ErrPermissionDenied
			}
		}
	} else {
		return ErrPermissionDenied
	}

	comment := &model.AppealComment{
		AppealID:        appeal.ID,
		CreatedByUserID: principal.UserID,
		Content:         content,
	}

	return s.appealRepo.AddComment(ctx, comment)
}

func (s *AppealService) GetComments(ctx context.Context, principal model.Principal, appealID string) ([]model.AppealComment, error) {
	appeal, err := s.appealRepo.GetByID(ctx, appealID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Проверяем права доступа (та же логика, что и в GetByID)
	if principal.IsDriver() {
		if appeal.CreatedByUserID != principal.UserID {
			return nil, ErrPermissionDenied
		}
	} else if principal.IsKgu() || principal.IsAkimat() {
		// KGU ZKH и Акимат могут видеть
	} else if principal.IsContractor() {
		if appeal.TicketID != nil {
			ticket, err := s.ticketRepo.GetByID(ctx, appeal.TicketID.String())
			if err != nil {
				return nil, err
			}
			if ticket.ContractorID != principal.OrgID {
				return nil, ErrPermissionDenied
			}
		}
	} else {
		return nil, ErrPermissionDenied
	}

	return s.appealRepo.GetCommentsByAppealID(ctx, appeal.ID)
}
