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
	ticketRepo     *repository.TicketRepository
	tripRepo       *repository.TripRepository
	assignmentRepo *repository.AssignmentRepository
	appealRepo     *repository.AppealRepository
}

func NewTicketService(
	ticketRepo *repository.TicketRepository,
	tripRepo *repository.TripRepository,
	assignmentRepo *repository.AssignmentRepository,
	appealRepo *repository.AppealRepository,
) *TicketService {
	return &TicketService{
		ticketRepo:     ticketRepo,
		tripRepo:       tripRepo,
		assignmentRepo: assignmentRepo,
		appealRepo:     appealRepo,
	}
}

func (s *TicketService) Create(ctx context.Context, principal model.Principal, input CreateTicketInput) (*model.Ticket, error) {
	// Only KGU ZKH can create tickets
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

type TicketListItem struct {
	Ticket  model.Ticket             `json:"ticket"`
	Metrics repository.TicketMetrics `json:"metrics"`
}

func (s *TicketService) List(ctx context.Context, principal model.Principal, filter repository.TicketListFilter) ([]TicketListItem, error) {
	if principal.IsAkimat() {
		// Akimat has access to every ticket
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

	tickets, err := s.ticketRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(tickets) == 0 {
		return []TicketListItem{}, nil
	}

	ids := make([]uuid.UUID, 0, len(tickets))
	for _, t := range tickets {
		ids = append(ids, t.ID)
	}

	metricsMap, err := s.ticketRepo.GetMetricsForTickets(ctx, ids)
	if err != nil {
		return nil, err
	}

	items := make([]TicketListItem, 0, len(tickets))
	for _, t := range tickets {
		metrics := repository.TicketMetrics{}
		if m, ok := metricsMap[t.ID]; ok && m != nil {
			metrics = *m
		}
		items = append(items, TicketListItem{
			Ticket:  t,
			Metrics: metrics,
		})
	}

	return items, nil
}

func (s *TicketService) Cancel(ctx context.Context, principal model.Principal, id string) error {
	// Only KGU ZKH can cancel tickets
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

	// Cancellation is allowed only if there are no facts yet (no trips, no fact_start_at)
	if ticket.FactStartAt != nil {
		return ErrConflict
	}

	// Check whether any trips exist
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
	// KGU ZKH can close tickets after validation
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

	// Closing is allowed only for tickets in COMPLETED status
	if ticket.Status != model.TicketStatusCompleted {
		return ErrConflict
	}

	ticket.Status = model.TicketStatusClosed
	return s.ticketRepo.Update(ctx, ticket)
}

func (s *TicketService) Complete(ctx context.Context, principal model.Principal, id string) error {
	// Contractors can request completion
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

	if ticket.Status != model.TicketStatusInProgress {
		if ticket.Status == model.TicketStatusCompleted {
			return nil
		}
		return ErrConflict
	}

	// Ensure every trip is closed (exit events present and truck empty on exit)
	incompleteTrips, err := s.ticketRepo.CountIncompleteTripsByTicketID(ctx, ticket.ID)
	if err != nil {
		return err
	}

	if incompleteTrips > 0 {
		return ErrConflict
	}

	// Ensure each driver marked the assignment as completed
	incompleteAssignments, err := s.ticketRepo.CountIncompleteAssignmentsByTicketID(ctx, ticket.ID)
	if err != nil {
		return err
	}

	if incompleteAssignments > 0 {
		return ErrConflict
	}

	invalidExitVolumeTrips, err := s.ticketRepo.CountTripsWithInvalidExitVolume(ctx, ticket.ID, exitVolumeTolerance)
	if err != nil {
		return err
	}
	if invalidExitVolumeTrips > 0 {
		return ErrConflict
	}

	now := time.Now()
	ticket.Status = model.TicketStatusCompleted
	if ticket.FactEndAt == nil {
		ticket.FactEndAt = &now
	}
	return s.ticketRepo.Update(ctx, ticket)
}

// TicketDetails aggregates everything shown in the ticket card
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

	// Load aggregated metrics
	metrics, err := s.ticketRepo.GetTicketMetrics(ctx, ticket.ID)
	if err != nil {
		return nil, err
	}

	// Load assignments
	assignments, err := s.ticketRepo.GetAssignmentsByTicketID(ctx, ticket.ID)
	if err != nil {
		return nil, err
	}

	// Narrow assignments down to the current driver
	if principal.IsDriver() && principal.DriverID != nil {
		var filteredAssignments []model.TicketAssignment
		for _, a := range assignments {
			if a.DriverID == *principal.DriverID {
				filteredAssignments = append(filteredAssignments, a)
			}
		}
		assignments = filteredAssignments
	}

	// Load trips
	trips, err := s.ticketRepo.GetTripsByTicketID(ctx, ticket.ID)
	if err != nil {
		return nil, err
	}

	// Driver sees only his trips
	if principal.IsDriver() && principal.DriverID != nil {
		var filteredTrips []model.Trip
		for _, t := range trips {
			if t.DriverID != nil && *t.DriverID == *principal.DriverID {
				filteredTrips = append(filteredTrips, t)
			}
		}
		trips = filteredTrips
	}

	// Load appeals
	appeals, err := s.ticketRepo.GetAppealsByTicketID(ctx, ticket.ID)
	if err != nil {
		return nil, err
	}

	// Drivers see only their own appeals
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

// OnTripCreated translates ticket statuses based on new trips
func (s *TicketService) OnTripCreated(ctx context.Context, ticketID uuid.UUID) error {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID.String())
	if err != nil {
		return err
	}

	// First trip automatically moves PLANNED ticket to IN_PROGRESS
	if ticket.Status == model.TicketStatusPlanned && ticket.FactStartAt == nil {
		// Ensure this is the earliest trip
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

// TryAutoComplete moves ticket to COMPLETED when all conditions are met
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

	invalidExitVolumeTrips, err := s.ticketRepo.CountTripsWithInvalidExitVolume(ctx, ticket.ID, exitVolumeTolerance)
	if err != nil {
		return err
	}
	if invalidExitVolumeTrips > 0 {
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
	// Users without an active assignment cannot access the ticket
	return false, nil
}
