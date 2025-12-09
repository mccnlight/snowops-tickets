package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"ticket-service/internal/client"
	"ticket-service/internal/model"
	"ticket-service/internal/repository"
	"ticket-service/internal/utils"
)

type TripService struct {
	tripRepo       *repository.TripRepository
	ticketRepo     *repository.TicketRepository
	assignmentRepo *repository.AssignmentRepository
	ticketService  *TicketService
	anprClient     *client.ANPRClient
	log            zerolog.Logger
}

func NewTripService(
	tripRepo *repository.TripRepository,
	ticketRepo *repository.TicketRepository,
	assignmentRepo *repository.AssignmentRepository,
	ticketService *TicketService,
	anprClient *client.ANPRClient,
	log zerolog.Logger,
) *TripService {
	return &TripService{
		tripRepo:       tripRepo,
		ticketRepo:     ticketRepo,
		assignmentRepo: assignmentRepo,
		ticketService:  ticketService,
		anprClient:     anprClient,
		log:            log,
	}
}

type CreateTripInput struct {
	TicketID            *string
	TicketAssignmentID  *string
	DriverID            *string
	VehicleID           *string
	CameraID            *string
	PolygonID           *string
	VehiclePlateNumber  string
	DetectedPlateNumber string
	EntryLprEventID     *string
	ExitLprEventID      *string
	EntryVolumeEventID  *string
	ExitVolumeEventID   *string
	DetectedVolumeEntry *float64
	DetectedVolumeExit  *float64
	EntryAt             string
	ExitAt              *string
	Status              model.TripStatus
}

func (s *TripService) Create(ctx context.Context, input CreateTripInput) (*model.Trip, error) {
	var ticketID *uuid.UUID
	if input.TicketID != nil {
		parsed, err := uuid.Parse(*input.TicketID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		ticketID = &parsed
	}

	var ticketAssignmentID *uuid.UUID
	if input.TicketAssignmentID != nil {
		parsed, err := uuid.Parse(*input.TicketAssignmentID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		ticketAssignmentID = &parsed
	}

	var driverID *uuid.UUID
	if input.DriverID != nil {
		parsed, err := uuid.Parse(*input.DriverID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		driverID = &parsed
	}

	var vehicleID *uuid.UUID
	if input.VehicleID != nil {
		parsed, err := uuid.Parse(*input.VehicleID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		vehicleID = &parsed
	}

	var cameraID *uuid.UUID
	if input.CameraID != nil {
		parsed, err := uuid.Parse(*input.CameraID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		cameraID = &parsed
	}

	var polygonID *uuid.UUID
	if input.PolygonID != nil {
		parsed, err := uuid.Parse(*input.PolygonID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		polygonID = &parsed
	}

	var entryLprEventID *uuid.UUID
	if input.EntryLprEventID != nil {
		parsed, err := uuid.Parse(*input.EntryLprEventID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		entryLprEventID = &parsed
	}

	var exitLprEventID *uuid.UUID
	if input.ExitLprEventID != nil {
		parsed, err := uuid.Parse(*input.ExitLprEventID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		exitLprEventID = &parsed
	}

	var entryVolumeEventID *uuid.UUID
	if input.EntryVolumeEventID != nil {
		parsed, err := uuid.Parse(*input.EntryVolumeEventID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		entryVolumeEventID = &parsed
	}

	var exitVolumeEventID *uuid.UUID
	if input.ExitVolumeEventID != nil {
		parsed, err := uuid.Parse(*input.ExitVolumeEventID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		exitVolumeEventID = &parsed
	}

	entryAt, err := time.Parse(time.RFC3339, input.EntryAt)
	if err != nil {
		return nil, ErrInvalidInput
	}

	var exitAt *time.Time
	if input.ExitAt != nil {
		parsed, err := time.Parse(time.RFC3339, *input.ExitAt)
		if err != nil {
			return nil, ErrInvalidInput
		}
		exitAt = &parsed
	}

	var assignment *model.TicketAssignment
	if ticketAssignmentID != nil {
		a, err := s.assignmentRepo.GetByID(ctx, ticketAssignmentID.String())
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		if !a.IsActive {
			return nil, ErrConflict
		}
		assignment = a
	}

	if assignment == nil && driverID != nil {
		a, err := s.assignmentRepo.FindActiveByDriver(ctx, *driverID)
		if err != nil {
			return nil, err
		}
		assignment = a
	}

	if assignment == nil && vehicleID != nil {
		a, err := s.assignmentRepo.FindActiveByVehicle(ctx, *vehicleID)
		if err != nil {
			return nil, err
		}
		assignment = a
	}

	if assignment != nil {
		resolvedTicketID := assignment.TicketID
		ticketID = &resolvedTicketID

		resolvedAssignmentID := assignment.ID
		ticketAssignmentID = &resolvedAssignmentID

		resolvedDriverID := assignment.DriverID
		driverID = &resolvedDriverID

		resolvedVehicleID := assignment.VehicleID
		vehicleID = &resolvedVehicleID
	}

	if ticketID != nil {
		t, err := s.ticketRepo.GetByID(ctx, ticketID.String())
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrNotFound
			}
			return nil, err
		}

		switch t.Status {
		case model.TicketStatusCancelled, model.TicketStatusClosed, model.TicketStatusCompleted:
			return nil, ErrConflict
		}
	}

	tripStatus := input.Status
	if ticketID == nil {
		tripStatus = model.TripStatusNoAssignment
	}

	trip := &model.Trip{
		TicketID:            ticketID,
		TicketAssignmentID:  ticketAssignmentID,
		DriverID:            driverID,
		VehicleID:           vehicleID,
		CameraID:            cameraID,
		PolygonID:           polygonID,
		VehiclePlateNumber:  input.VehiclePlateNumber,
		DetectedPlateNumber: input.DetectedPlateNumber,
		EntryLprEventID:     entryLprEventID,
		ExitLprEventID:      exitLprEventID,
		EntryVolumeEventID:  entryVolumeEventID,
		ExitVolumeEventID:   exitVolumeEventID,
		DetectedVolumeEntry: input.DetectedVolumeEntry,
		DetectedVolumeExit:  input.DetectedVolumeExit,
		EntryAt:             entryAt,
		ExitAt:              exitAt,
		Status:              tripStatus,
	}

	if err := s.tripRepo.Create(ctx, trip); err != nil {
		return nil, err
	}

	// Автоматический переход статуса тикета при создании первого рейса
	if ticketID != nil && s.ticketService != nil {
		if err := s.ticketService.OnTripCreated(ctx, *ticketID); err != nil {
			return nil, err
		}
	}

	return trip, nil
}

func (s *TripService) ListByTicketID(ctx context.Context, principal model.Principal, ticketID string) ([]model.Trip, error) {
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
		// Водитель видит только свои рейсы
		return s.tripRepo.ListByDriverID(ctx, *principal.DriverID, &ticket.ID)
	} else {
		return nil, ErrPermissionDenied
	}

	return s.tripRepo.ListByTicketID(ctx, ticket.ID)
}

func (s *TripService) GetByID(ctx context.Context, principal model.Principal, id string) (*model.Trip, error) {
	trip, err := s.tripRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Проверяем права доступа через тикет
	if trip.TicketID != nil {
		ticket, err := s.ticketRepo.GetByID(ctx, trip.TicketID.String())
		if err != nil {
			return nil, err
		}

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
			if principal.DriverID == nil || trip.DriverID == nil || *trip.DriverID != *principal.DriverID {
				return nil, ErrPermissionDenied
			}
		} else {
			return nil, ErrPermissionDenied
		}
	}

	return trip, nil
}

// ReceptionJournalInput входные данные для журнала приёма
type ReceptionJournalInput struct {
	PolygonIDs   []uuid.UUID
	DateFrom     *time.Time
	DateTo       *time.Time
	ContractorID *uuid.UUID
	Status       *model.TripStatus
}

// ReceptionJournalResult результат журнала приёма
type ReceptionJournalResult struct {
	Trips         []repository.ReceptionJournalEntry `json:"trips"`
	TotalVolumeM3 float64                            `json:"total_volume_m3"`
	TotalTrips    int                                `json:"total_trips"`
}

// GetReceptionJournal возвращает журнал приёма снега для LANDFILL
func (s *TripService) GetReceptionJournal(ctx context.Context, principal model.Principal, input ReceptionJournalInput) (*ReceptionJournalResult, error) {
	if !principal.IsLandfill() {
		return nil, ErrPermissionDenied
	}

	filter := repository.ReceptionJournalFilter{
		PolygonIDs:   input.PolygonIDs,
		DateFrom:     input.DateFrom,
		DateTo:       input.DateTo,
		ContractorID: input.ContractorID,
		Status:       input.Status,
	}

	entries, err := s.tripRepo.ListReceptionJournal(ctx, filter)
	if err != nil {
		return nil, err
	}

	var totalVolume float64
	for _, entry := range entries {
		totalVolume += entry.NetVolumeM3
	}

	return &ReceptionJournalResult{
		Trips:         entries,
		TotalVolumeM3: totalVolume,
		TotalTrips:    len(entries),
	}, nil
}

// CalculateVolumeForAssignment вычисляет объем перевезенного снега для назначения
// Получает события ANPR за период рейса и суммирует объемы всех событий въезда
// Принимает уже полученное assignment, чтобы избежать дублирования запросов
func (s *TripService) CalculateVolumeForAssignment(ctx context.Context, assignment *model.TicketAssignment) (float64, error) {
	// Проверяем, что есть временные метки
	if assignment.TripStartedAt == nil {
		return 0, fmt.Errorf("trip not started (trip_started_at is nil)")
	}
	if assignment.TripFinishedAt == nil {
		return 0, fmt.Errorf("trip not finished (trip_finished_at is nil)")
	}

	// Получаем номер машины
	plateNumber, err := s.assignmentRepo.GetVehiclePlateNumber(ctx, assignment.VehicleID)
	if err != nil {
		s.log.Warn().
			Err(err).
			Str("assignment_id", assignment.ID.String()).
			Str("vehicle_id", assignment.VehicleID.String()).
			Msg("failed to get vehicle plate number")
		return 0, fmt.Errorf("failed to get vehicle plate number: %w", err)
	}

	// Нормализуем номер
	normalizedPlate := utils.NormalizePlate(plateNumber)
	if normalizedPlate == "" {
		return 0, fmt.Errorf("invalid plate number format")
	}

	// Запрашиваем события ANPR за период рейса (только въезды)
	direction := "entry"
	events, err := s.anprClient.GetEventsByPlateAndTime(
		ctx,
		normalizedPlate,
		*assignment.TripStartedAt,
		*assignment.TripFinishedAt,
		&direction,
	)
	if err != nil {
		s.log.Error().
			Err(err).
			Str("assignment_id", assignment.ID.String()).
			Str("plate", normalizedPlate).
			Time("start", *assignment.TripStartedAt).
			Time("end", *assignment.TripFinishedAt).
			Msg("failed to get ANPR events")
		return 0, fmt.Errorf("failed to get ANPR events: %w", err)
	}

	// Суммируем объемы всех событий въезда
	var totalVolume float64
	eventCount := 0
	for _, event := range events {
		if event.SnowVolumeM3 != nil {
			totalVolume += *event.SnowVolumeM3
			eventCount++
		}
	}

	s.log.Info().
		Str("assignment_id", assignment.ID.String()).
		Str("plate", normalizedPlate).
		Float64("total_volume_m3", totalVolume).
		Int("events_count", eventCount).
		Int("total_events", len(events)).
		Msg("calculated volume for assignment")

	if totalVolume == 0 && len(events) > 0 {
		s.log.Warn().
			Str("assignment_id", assignment.ID.String()).
			Str("plate", normalizedPlate).
			Int("events_count", len(events)).
			Msg("trip completed with zero volume (all events have nil or zero volume)")
	}

	return totalVolume, nil
}

// CompleteTripAndCalculateVolume завершает рейс, рассчитывает объем и создает/обновляет запись trip
// Вызывается после того, как trip_finished_at уже установлен в AssignmentService
func (s *TripService) CompleteTripAndCalculateVolume(ctx context.Context, assignmentID uuid.UUID) (*model.Trip, error) {
	// Получаем назначение (оно уже обновлено с trip_finished_at)
	assignment, err := s.assignmentRepo.GetByID(ctx, assignmentID.String())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get assignment: %w", err)
	}

	// Проверяем, что рейс был начат и завершен
	if assignment.TripStartedAt == nil {
		return nil, fmt.Errorf("trip not started")
	}
	if assignment.TripFinishedAt == nil {
		return nil, fmt.Errorf("trip not finished (trip_finished_at is nil)")
	}

	// Рассчитываем объем (передаем уже полученное assignment)
	totalVolume, err := s.CalculateVolumeForAssignment(ctx, assignment)
	if err != nil {
		s.log.Error().
			Err(err).
			Str("assignment_id", assignmentID.String()).
			Msg("failed to calculate volume, completing trip with volume 0")
		// Продолжаем выполнение, но с объемом 0
		// Это нормальная ситуация - если ANPR недоступен или события отсутствуют
		totalVolume = 0
	}

	// Получаем номер машины для записи в trip
	plateNumber, err := s.assignmentRepo.GetVehiclePlateNumber(ctx, assignment.VehicleID)
	if err != nil {
		s.log.Warn().
			Err(err).
			Str("assignment_id", assignmentID.String()).
			Msg("failed to get vehicle plate number for trip, using empty string")
		plateNumber = ""
	}
	normalizedPlate := utils.NormalizePlate(plateNumber)

	// Ищем существующий trip для этого назначения
	existingTrip, err := s.tripRepo.FindByAssignmentID(ctx, assignmentID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to find existing trip: %w", err)
	}

	if existingTrip != nil {
		// Обновляем существующий trip
		existingTrip.ExitAt = assignment.TripFinishedAt
		existingTrip.TotalVolumeM3 = &totalVolume
		existingTrip.Status = model.TripStatusOK
		existingTrip.AutoCreated = true

		if err := s.tripRepo.Update(ctx, existingTrip); err != nil {
			return nil, fmt.Errorf("failed to update trip: %w", err)
		}

		s.log.Info().
			Str("trip_id", existingTrip.ID.String()).
			Str("assignment_id", assignmentID.String()).
			Float64("total_volume_m3", totalVolume).
			Msg("updated existing trip with calculated volume")

		return existingTrip, nil
	}

	// Создаем новый trip
	trip := &model.Trip{
		TicketID:           &assignment.TicketID,
		TicketAssignmentID: &assignmentID,
		DriverID:           &assignment.DriverID,
		VehicleID:          &assignment.VehicleID,
		VehiclePlateNumber: normalizedPlate,
		EntryAt:            *assignment.TripStartedAt,
		ExitAt:             assignment.TripFinishedAt,
		TotalVolumeM3:      &totalVolume,
		AutoCreated:        true,
		Status:             model.TripStatusOK,
	}

	if err := s.tripRepo.Create(ctx, trip); err != nil {
		return nil, fmt.Errorf("failed to create trip: %w", err)
	}

	s.log.Info().
		Str("trip_id", trip.ID.String()).
		Str("assignment_id", assignmentID.String()).
		Float64("total_volume_m3", totalVolume).
		Msg("created new trip with calculated volume")

	// Автоматический переход статуса тикета при создании рейса
	if s.ticketService != nil {
		if err := s.ticketService.OnTripCreated(ctx, assignment.TicketID); err != nil {
			s.log.Warn().
				Err(err).
				Str("ticket_id", assignment.TicketID.String()).
				Msg("failed to update ticket status on trip creation")
		}
	}

	return trip, nil
}
