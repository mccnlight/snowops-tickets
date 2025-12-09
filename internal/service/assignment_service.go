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

type AssignmentService struct {
	assignmentRepo *repository.AssignmentRepository
	ticketRepo     *repository.TicketRepository
	ticketService  *TicketService
	tripService    *TripService
}

func NewAssignmentService(
	assignmentRepo *repository.AssignmentRepository,
	ticketRepo *repository.TicketRepository,
	ticketService *TicketService,
	tripService *TripService,
) *AssignmentService {
	return &AssignmentService{
		assignmentRepo: assignmentRepo,
		ticketRepo:     ticketRepo,
		ticketService:  ticketService,
		tripService:    tripService,
	}
}

type CreateAssignmentInput struct {
	TicketID  string
	DriverID  string
	VehicleID string
}

func (s *AssignmentService) Create(ctx context.Context, principal model.Principal, input CreateAssignmentInput) (*model.TicketAssignment, error) {
	// Только подрядчик может создавать назначения
	if !principal.IsContractor() {
		return nil, ErrPermissionDenied
	}

	ticketID, err := uuid.Parse(input.TicketID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	driverID, err := uuid.Parse(input.DriverID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	vehicleID, err := uuid.Parse(input.VehicleID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	// Проверяем, что тикет принадлежит подрядчику
	ticket, err := s.ticketRepo.GetByID(ctx, input.TicketID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if ticket.ContractorID != principal.OrgID {
		return nil, ErrPermissionDenied
	}
	if !isTicketMutableForAssignments(ticket.Status) {
		return nil, ErrConflict
	}

	assignment := &model.TicketAssignment{
		TicketID:         ticketID,
		DriverID:         driverID,
		VehicleID:        vehicleID,
		DriverMarkStatus: model.DriverMarkStatusNotStarted,
		IsActive:         true,
	}

	if err := s.assignmentRepo.Create(ctx, assignment); err != nil {
		return nil, err
	}

	return assignment, nil
}

func (s *AssignmentService) Delete(ctx context.Context, principal model.Principal, id string) error {
	// Только подрядчик может удалять назначения
	if !principal.IsContractor() {
		return ErrPermissionDenied
	}

	assignment, err := s.assignmentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	// Проверяем, что тикет принадлежит подрядчику
	ticket, err := s.ticketRepo.GetByID(ctx, assignment.TicketID.String())
	if err != nil {
		return err
	}

	if ticket.ContractorID != principal.OrgID {
		return ErrPermissionDenied
	}
	if !isTicketMutableForAssignments(ticket.Status) {
		return ErrConflict
	}

	return s.assignmentRepo.Delete(ctx, id)
}

func (s *AssignmentService) UpdateDriverMarkStatus(ctx context.Context, principal model.Principal, id string, status model.DriverMarkStatus) error {
	// Только водитель может обновлять свой статус
	if !principal.IsDriver() || principal.DriverID == nil {
		return ErrPermissionDenied
	}

	assignment, err := s.assignmentRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	// Проверяем, что это назначение принадлежит водителю
	if assignment.DriverID != *principal.DriverID {
		return ErrPermissionDenied
	}

	// Проверяем статус тикета
	ticket, err := s.ticketRepo.GetByID(ctx, assignment.TicketID.String())
	if err != nil {
		return err
	}
	if ticket.Status == model.TicketStatusCancelled || ticket.Status == model.TicketStatusClosed || ticket.Status == model.TicketStatusCompleted {
		return ErrConflict
	}

	// Обновляем статус и временные метки
	now := time.Now()
	if status == model.DriverMarkStatusInWork {
		// Проверяем, что рейс еще не начат
		if assignment.TripStartedAt != nil {
			return ErrConflict // рейс уже начат
		}
		// При начале работы фиксируем время начала рейса и обновляем статус
		if err := s.assignmentRepo.UpdateTripStartedAt(ctx, id, now, status); err != nil {
			return err
		}
		assignment.TripStartedAt = &now
	} else if status == model.DriverMarkStatusCompleted {
		// Проверяем, что рейс был начат
		if assignment.TripStartedAt == nil {
			return ErrConflict // нельзя завершить рейс, который не был начат
		}
		// Проверяем, что рейс еще не завершен
		if assignment.TripFinishedAt != nil {
			return ErrConflict // рейс уже завершен
		}
		// При завершении фиксируем время окончания рейса и обновляем статус
		if err := s.assignmentRepo.UpdateTripFinishedAt(ctx, id, now, status); err != nil {
			return err
		}
		assignment.TripFinishedAt = &now
	} else {
		// Для других статусов просто обновляем статус
		if err := s.assignmentRepo.UpdateDriverMarkStatus(ctx, id, status); err != nil {
			return err
		}
	}

	// Если водитель отметил "В работе", проверяем, нужно ли перевести тикет в IN_PROGRESS
	if status == model.DriverMarkStatusInWork {
		if ticket.Status == model.TicketStatusPlanned && ticket.FactStartAt == nil {
			now := time.Now()
			ticket.Status = model.TicketStatusInProgress
			ticket.FactStartAt = &now
			if err := s.ticketRepo.Update(ctx, ticket); err != nil {
				return err
			}
		}
	}

	if status == model.DriverMarkStatusCompleted {
		// Рассчитываем объем и создаем/обновляем trip
		// Если расчет объема или создание trip не удались, рейс все равно считается завершенным
		// (assignment уже обновлен с trip_finished_at и COMPLETED статусом)
		if s.tripService != nil {
			assignmentUUID, err := uuid.Parse(id)
			if err == nil {
				_, err := s.tripService.CompleteTripAndCalculateVolume(ctx, assignmentUUID)
				if err != nil {
					// Логируем ошибку, но НЕ прерываем выполнение
					// Рейс должен быть завершен даже если расчет объема или создание trip не удались
					// Согласно ТЗ, лучше завершить рейс с объемом 0, чем вернуть ошибку пользователю
					// Ошибка уже залогирована в CompleteTripAndCalculateVolume
				}
			}
		}

		if err := s.ticketService.TryAutoComplete(ctx, assignment.TicketID); err != nil {
			return err
		}
	}

	return nil
}

func (s *AssignmentService) ListByTicketID(ctx context.Context, principal model.Principal, ticketID string) ([]model.TicketAssignment, error) {
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
		if principal.DriverID == nil {
			return nil, ErrPermissionDenied
		}
		has, err := s.assignmentRepo.HasActiveAssignment(ctx, ticket.ID, *principal.DriverID)
		if err != nil {
			return nil, err
		}
		if !has {
			return nil, ErrPermissionDenied
		}
		// Водитель видит только свои назначения
		assignments, err := s.assignmentRepo.ListByTicketID(ctx, ticket.ID)
		if err != nil {
			return nil, err
		}
		// Фильтруем по driver_id
		var result []model.TicketAssignment
		for _, a := range assignments {
			if a.DriverID == *principal.DriverID {
				result = append(result, a)
			}
		}
		return result, nil
	} else {
		return nil, ErrPermissionDenied
	}

	return s.assignmentRepo.ListByTicketID(ctx, ticket.ID)
}

func isTicketMutableForAssignments(status model.TicketStatus) bool {
	switch status {
	case model.TicketStatusPlanned, model.TicketStatusInProgress:
		return true
	default:
		return false
	}
}
