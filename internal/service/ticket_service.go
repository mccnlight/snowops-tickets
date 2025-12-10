package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
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
	ticketRepo     *repository.TicketRepository
	tripRepo       *repository.TripRepository
	assignmentRepo *repository.AssignmentRepository
	appealRepo     *repository.AppealRepository
	areaAccessRepo *repository.CleaningAreaAccessRepository
	log            zerolog.Logger
}

func NewTicketService(
	ticketRepo *repository.TicketRepository,
	tripRepo *repository.TripRepository,
	assignmentRepo *repository.AssignmentRepository,
	appealRepo *repository.AppealRepository,
	areaAccessRepo *repository.CleaningAreaAccessRepository,
	log zerolog.Logger,
) *TicketService {
	return &TicketService{
		ticketRepo:     ticketRepo,
		tripRepo:       tripRepo,
		assignmentRepo: assignmentRepo,
		appealRepo:     appealRepo,
		areaAccessRepo: areaAccessRepo,
		log:            log,
	}
}

func (s *TicketService) Create(ctx context.Context, principal model.Principal, input CreateTicketInput) (*model.Ticket, error) {
	// Только KGU ZKH может создавать тикеты
	if !principal.IsKgu() {
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

	if input.ContractID == "" {
		return nil, ErrInvalidInput
	}
	contractID, err := uuid.Parse(input.ContractID)
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
		ContractID:     contractID,
		CreatedByOrgID: principal.OrgID,
		Status:         model.TicketStatusPlanned,
		PlannedStartAt: plannedStartAt,
		PlannedEndAt:   plannedEndAt,
		Description:    input.Description,
	}

	if err := s.ticketRepo.Create(ctx, ticket); err != nil {
		return nil, err
	}

	// Automatically grant access to cleaning area for contractor
	// This is best-effort: if it fails, we log but don't fail ticket creation
	if err := s.areaAccessRepo.Grant(ctx, cleaningAreaID, contractorID, "TICKET"); err != nil {
		s.log.Warn().
			Err(err).
			Str("cleaning_area_id", cleaningAreaID.String()).
			Str("contractor_id", contractorID.String()).
			Str("ticket_id", ticket.ID.String()).
			Msg("failed to grant cleaning area access for contractor (ticket created successfully)")
	} else {
		s.log.Info().
			Str("cleaning_area_id", cleaningAreaID.String()).
			Str("contractor_id", contractorID.String()).
			Str("ticket_id", ticket.ID.String()).
			Msg("automatically granted cleaning area access for contractor")
	}

	return ticket, nil
}

type CreateTicketInput struct {
	CleaningAreaID string
	ContractorID   string
	ContractID     string
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

	ok, err := s.canAccessTicket(ctx, principal, ticket)
	if err != nil {
		return nil, err
	}
	if !ok {
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
	} else {
		return nil, ErrPermissionDenied
	}

	return s.ticketRepo.List(ctx, filter)
}

func (s *TicketService) Cancel(ctx context.Context, principal model.Principal, id string) error {
	// Только KGU ZKH может отменять тикеты
	if !principal.IsKgu() {
		return ErrPermissionDenied
	}

	ticket, err := s.ticketRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	if ticket.CreatedByOrgID != principal.OrgID {
		return ErrPermissionDenied
	}

	// Можно отменить только если нет фактов (нет рейсов и fact_start_at пустой)
	if ticket.FactStartAt != nil {
		return ErrConflict
	}

	// Проверяем, есть ли рейсы
	tripCount, err := s.ticketRepo.CountTripsByTicketID(ctx, ticket.ID)
	if err != nil {
		return err
	}

	if tripCount > 0 {
		return ErrConflict
	}

	ticket.Status = model.TicketStatusCancelled
	return s.ticketRepo.Update(ctx, ticket)
}

func (s *TicketService) Close(ctx context.Context, principal model.Principal, id string) error {
	// KGU ZKH может закрывать тикеты после проверки
	if !principal.IsKgu() {
		return ErrPermissionDenied
	}

	ticket, err := s.ticketRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	if ticket.CreatedByOrgID != principal.OrgID {
		return ErrPermissionDenied
	}

	// Можно закрыть только если тикет в статусе COMPLETED
	if ticket.Status != model.TicketStatusCompleted {
		return ErrConflict
	}

	ticket.Status = model.TicketStatusClosed
	return s.ticketRepo.Update(ctx, ticket)
}

func (s *TicketService) Complete(ctx context.Context, principal model.Principal, id string) error {
	// Подрядчик может завершить тикет
	if !principal.IsContractor() {
		return ErrPermissionDenied
	}

	ticket, err := s.ticketRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	if ticket.ContractorID != principal.OrgID {
		return ErrPermissionDenied
	}

	// Проверяем, что все рейсы закрыты (есть exit события и кузов пустой)
	incompleteTrips, err := s.ticketRepo.CountIncompleteTripsByTicketID(ctx, ticket.ID)
	if err != nil {
		return err
	}

	if incompleteTrips > 0 {
		return ErrConflict
	}

	// Проверяем, что все водители отметили "Завершено"
	incompleteAssignments, err := s.ticketRepo.CountIncompleteAssignmentsByTicketID(ctx, ticket.ID)
	if err != nil {
		return err
	}

	if incompleteAssignments > 0 {
		return ErrConflict
	}

	now := time.Now()
	ticket.Status = model.TicketStatusCompleted
	if ticket.FactEndAt == nil {
		ticket.FactEndAt = &now
	}
	return s.ticketRepo.Update(ctx, ticket)
}

// TicketDetails содержит полную информацию о тикете
type TicketDetails struct {
	Ticket      *model.Ticket             `json:"ticket"`
	Metrics     *repository.TicketMetrics `json:"metrics"`
	Assignments []model.TicketAssignment  `json:"assignments"`
	Trips       []model.Trip              `json:"trips"`
	Appeals     []model.Appeal            `json:"appeals"`
}

func (s *TicketService) GetDetails(ctx context.Context, principal model.Principal, id string) (*TicketDetails, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	ok, err := s.canAccessTicket(ctx, principal, ticket)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrPermissionDenied
	}

	// Получаем метрики
	metrics, err := s.ticketRepo.GetTicketMetrics(ctx, ticket.ID)
	if err != nil {
		return nil, err
	}

	// Получаем назначения
	assignments, err := s.ticketRepo.GetAssignmentsByTicketID(ctx, ticket.ID)
	if err != nil {
		return nil, err
	}

	// Фильтруем назначения для водителя
	if principal.IsDriver() && principal.DriverID != nil {
		var filteredAssignments []model.TicketAssignment
		for _, a := range assignments {
			if a.DriverID == *principal.DriverID {
				filteredAssignments = append(filteredAssignments, a)
			}
		}
		assignments = filteredAssignments
	}

	// Получаем рейсы
	trips, err := s.ticketRepo.GetTripsByTicketID(ctx, ticket.ID)
	if err != nil {
		return nil, err
	}

	// Фильтруем рейсы для водителя
	if principal.IsDriver() && principal.DriverID != nil {
		var filteredTrips []model.Trip
		for _, t := range trips {
			if t.DriverID != nil && *t.DriverID == *principal.DriverID {
				filteredTrips = append(filteredTrips, t)
			}
		}
		trips = filteredTrips
	}

	// Получаем обжалования
	appeals, err := s.ticketRepo.GetAppealsByTicketID(ctx, ticket.ID)
	if err != nil {
		return nil, err
	}

	// Фильтруем обжалования для водителя
	if principal.IsDriver() && principal.DriverID != nil {
		var filteredAppeals []model.Appeal
		for _, a := range appeals {
			if a.CreatedByUserID == principal.UserID {
				filteredAppeals = append(filteredAppeals, a)
			}
		}
		appeals = filteredAppeals
	}

	return &TicketDetails{
		Ticket:      ticket,
		Metrics:     metrics,
		Assignments: assignments,
		Trips:       trips,
		Appeals:     appeals,
	}, nil
}

// OnTripCreated вызывается при создании нового рейса для автоматического перехода статусов
func (s *TicketService) OnTripCreated(ctx context.Context, ticketID uuid.UUID) error {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID.String())
	if err != nil {
		return err
	}

	// Если тикет в статусе PLANNED и это первый рейс, переводим в IN_PROGRESS
	if ticket.Status == model.TicketStatusPlanned && ticket.FactStartAt == nil {
		// Проверяем, это ли первый рейс
		firstTrip, err := s.tripRepo.GetFirstTripByTicketID(ctx, ticketID)
		if err != nil {
			return err
		}

		if firstTrip != nil {
			now := time.Now()
			ticket.Status = model.TicketStatusInProgress
			ticket.FactStartAt = &now
			return s.ticketRepo.Update(ctx, ticket)
		}
	}

	return nil
}

// TryAutoComplete переводит тикет в COMPLETED, если выполнены все условия
func (s *TicketService) TryAutoComplete(ctx context.Context, ticketID uuid.UUID) error {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID.String())
	if err != nil {
		return err
	}

	if ticket.Status != model.TicketStatusInProgress {
		return nil
	}

	incompleteTrips, err := s.ticketRepo.CountIncompleteTripsByTicketID(ctx, ticket.ID)
	if err != nil {
		return err
	}
	if incompleteTrips > 0 {
		return nil
	}

	incompleteAssignments, err := s.ticketRepo.CountIncompleteAssignmentsByTicketID(ctx, ticket.ID)
	if err != nil {
		return err
	}
	if incompleteAssignments > 0 {
		return nil
	}

	now := time.Now()
	ticket.Status = model.TicketStatusCompleted
	if ticket.FactEndAt == nil {
		ticket.FactEndAt = &now
	}

	return s.ticketRepo.Update(ctx, ticket)
}

func (s *TicketService) canAccessTicket(ctx context.Context, principal model.Principal, ticket *model.Ticket) (bool, error) {
	if principal.IsAkimat() {
		return true, nil
	}
	if principal.IsKgu() {
		return ticket.CreatedByOrgID == principal.OrgID, nil
	}
	if principal.IsContractor() {
		return ticket.ContractorID == principal.OrgID, nil
	}
	if principal.IsDriver() && principal.DriverID != nil {
		has, err := s.assignmentRepo.HasActiveAssignment(ctx, ticket.ID, *principal.DriverID)
		if err != nil {
			return false, err
		}
		return has, nil
	}
	// Драйвер без assignment и другие роли — нет доступа
	return false, nil
}

func (s *TicketService) Delete(ctx context.Context, principal model.Principal, id string) error {
	// Only KGU can delete tickets
	if !principal.IsKgu() {
		return ErrPermissionDenied
	}

	ticket, err := s.ticketRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	// Verify ticket was created by the principal's organization
	if ticket.CreatedByOrgID != principal.OrgID {
		return ErrPermissionDenied
	}

	// Delete ticket (cascades to assignments and appeals)
	// trips.ticket_id will be set to NULL automatically via ON DELETE SET NULL
	return s.ticketRepo.Delete(ctx, id)
}
