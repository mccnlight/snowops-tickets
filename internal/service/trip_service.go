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

type TripService struct {
	tripRepo       *repository.TripRepository
	ticketRepo     *repository.TicketRepository
	assignmentRepo *repository.AssignmentRepository
	ticketService  *TicketService
}

func NewTripService(
	tripRepo *repository.TripRepository,
	ticketRepo *repository.TicketRepository,
	assignmentRepo *repository.AssignmentRepository,
	ticketService *TicketService,
) *TripService {
	return &TripService{
		tripRepo:       tripRepo,
		ticketRepo:     ticketRepo,
		assignmentRepo: assignmentRepo,
		ticketService:  ticketService,
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
	PolygonEntryTime    *string
	PolygonExitTime     *string
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

	var polygonEntryTime *time.Time
	if input.PolygonEntryTime != nil {
		parsed, err := time.Parse(time.RFC3339, *input.PolygonEntryTime)
		if err != nil {
			return nil, ErrInvalidInput
		}
		polygonEntryTime = &parsed
	}

	var polygonExitTime *time.Time
	if input.PolygonExitTime != nil {
		parsed, err := time.Parse(time.RFC3339, *input.PolygonExitTime)
		if err != nil {
			return nil, ErrInvalidInput
		}
		polygonExitTime = &parsed
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
		PolygonEntryTime:    polygonEntryTime,
		PolygonExitTime:     polygonExitTime,
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

// ProcessCameraEvent обрабатывает событие камеры (LPR или Volume) и создает/обновляет рейс
type ProcessCameraEventInput struct {
	EventType      string // "LPR_ENTRY", "LPR_EXIT", "VOLUME_ENTRY", "VOLUME_EXIT"
	EventID        string
	CameraID       string
	PolygonID      *string
	PlateNumber    *string  // для LPR событий
	DetectedVolume *float64 // для Volume событий
	DetectedAt     string
	Direction      *string
	PhotoURL       *string
	Confidence     *float64 // для LPR событий
}

func (s *TripService) ProcessCameraEvent(ctx context.Context, input ProcessCameraEventInput) (*model.Trip, error) {
	// Валидация входных данных
	if _, err := uuid.Parse(input.EventID); err != nil {
		return nil, ErrInvalidInput
	}

	if _, err := uuid.Parse(input.CameraID); err != nil {
		return nil, ErrInvalidInput
	}

	if _, err := time.Parse(time.RFC3339, input.DetectedAt); err != nil {
		return nil, ErrInvalidInput
	}

	var polygonID *uuid.UUID
	if input.PolygonID != nil {
		parsed, err := uuid.Parse(*input.PolygonID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		polygonID = &parsed
	}

	// Определяем, это вход или выход
	isEntry := input.EventType == "LPR_ENTRY" || input.EventType == "VOLUME_ENTRY"
	isExit := input.EventType == "LPR_EXIT" || input.EventType == "VOLUME_EXIT"

	if !isEntry && !isExit {
		return nil, ErrInvalidInput
	}

	// Ищем открытый рейс для этой машины и полигона
	var openTrip *model.Trip
	if polygonID != nil {
		// Ищем по vehicle_id и polygon_id, где exit_at IS NULL
		openTrips, err := s.tripRepo.FindOpenTripsByVehicleAndPolygon(ctx, nil, polygonID)
		if err == nil && len(openTrips) > 0 {
			// Берем последний открытый рейс
			openTrip = &openTrips[len(openTrips)-1]
		}
	}

	// Если это входное событие и нет открытого рейса, создаем новый
	if isEntry && openTrip == nil {
		var vehicleID *uuid.UUID
		var driverID *uuid.UUID
		var ticketID *uuid.UUID
		var assignmentID *uuid.UUID

		// Пытаемся найти vehicle по plate_number
		// TODO: Интеграция с сервисом vehicles для поиска по номеру
		// После реализации интеграции раскомментировать:
		// if input.PlateNumber != nil {
		// 	vehicleID = findVehicleByPlateNumber(input.PlateNumber)
		// }

		// Пытаемся найти активное назначение
		// TODO: Раскомментировать после реализации интеграции с vehicles
		// if vehicleID != nil {
		// 	assignment, err := s.assignmentRepo.FindActiveByVehicle(ctx, *vehicleID)
		// 	if err == nil && assignment != nil {
		// 		ticketID = &assignment.TicketID
		// 		assignmentID = &assignment.ID
		// 		driverID = &assignment.DriverID
		// 	}
		// }

		// Создаем новый рейс
		createInput := CreateTripInput{
			VehicleID:           vehicleIDToString(vehicleID),
			DriverID:            uuidToString(driverID),
			CameraID:            &input.CameraID,
			PolygonID:           input.PolygonID,
			DetectedPlateNumber: stringPtrToString(input.PlateNumber),
			EntryAt:             input.DetectedAt,
			PolygonEntryTime:    &input.DetectedAt,
		}

		if input.EventType == "LPR_ENTRY" {
			createInput.EntryLprEventID = &input.EventID
		} else if input.EventType == "VOLUME_ENTRY" {
			createInput.EntryVolumeEventID = &input.EventID
			createInput.DetectedVolumeEntry = input.DetectedVolume
		}

		if ticketID != nil {
			tid := ticketID.String()
			createInput.TicketID = &tid
		}
		if assignmentID != nil {
			aid := assignmentID.String()
			createInput.TicketAssignmentID = &aid
		}

		return s.Create(ctx, createInput)
	}

	// Если это выходное событие и есть открытый рейс, обновляем его
	if isExit && openTrip != nil {
		updateInput := UpdateTripExitInput{
			TripID: openTrip.ID.String(),
		}

		if input.EventType == "LPR_EXIT" {
			updateInput.ExitLprEventID = &input.EventID
			updateInput.PolygonExitTime = &input.DetectedAt
		} else if input.EventType == "VOLUME_EXIT" {
			updateInput.ExitVolumeEventID = &input.EventID
			updateInput.DetectedVolumeExit = input.DetectedVolume
			if openTrip.PolygonExitTime == nil {
				updateInput.PolygonExitTime = &input.DetectedAt
			}
		}

		return s.UpdateTripExit(ctx, updateInput)
	}

	return nil, ErrConflict
}

// UpdateTripExitInput содержит данные для обновления рейса при выезде
type UpdateTripExitInput struct {
	TripID             string
	ExitLprEventID     *string
	ExitVolumeEventID  *string
	DetectedVolumeExit *float64
	PolygonExitTime    *string
	ExitAt             *string
}

// UpdateTripExit обновляет рейс при выезде с полигона
func (s *TripService) UpdateTripExit(ctx context.Context, input UpdateTripExitInput) (*model.Trip, error) {
	trip, err := s.tripRepo.GetByID(ctx, input.TripID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if trip.ExitAt != nil {
		// Рейс уже закрыт
		return nil, ErrConflict
	}

	var exitLprEventID *uuid.UUID
	if input.ExitLprEventID != nil {
		parsed, err := uuid.Parse(*input.ExitLprEventID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		exitLprEventID = &parsed
	}

	var exitVolumeEventID *uuid.UUID
	if input.ExitVolumeEventID != nil {
		parsed, err := uuid.Parse(*input.ExitVolumeEventID)
		if err != nil {
			return nil, ErrInvalidInput
		}
		exitVolumeEventID = &parsed
	}

	var polygonExitTime *time.Time
	if input.PolygonExitTime != nil {
		parsed, err := time.Parse(time.RFC3339, *input.PolygonExitTime)
		if err != nil {
			return nil, ErrInvalidInput
		}
		polygonExitTime = &parsed
	}

	var exitAt *time.Time
	if input.ExitAt != nil {
		parsed, err := time.Parse(time.RFC3339, *input.ExitAt)
		if err != nil {
			return nil, ErrInvalidInput
		}
		exitAt = &parsed
	} else if polygonExitTime != nil {
		exitAt = polygonExitTime
	}

	// Обновляем поля
	if exitLprEventID != nil {
		trip.ExitLprEventID = exitLprEventID
	}
	if exitVolumeEventID != nil {
		trip.ExitVolumeEventID = exitVolumeEventID
	}
	if input.DetectedVolumeExit != nil {
		trip.DetectedVolumeExit = input.DetectedVolumeExit
	}
	if polygonExitTime != nil {
		trip.PolygonExitTime = polygonExitTime
	}
	if exitAt != nil {
		trip.ExitAt = exitAt
	}

	if err := s.tripRepo.Update(ctx, trip); err != nil {
		return nil, err
	}

	// Проверяем, можно ли автоматически завершить тикет
	if trip.TicketID != nil && s.ticketService != nil {
		if err := s.ticketService.TryAutoComplete(ctx, *trip.TicketID); err != nil {
			// Логируем ошибку, но не прерываем выполнение
		}
	}

	return trip, nil
}

// CompleteTripByGPSInput содержит данные для завершения рейса по GPS
type CompleteTripByGPSInput struct {
	TripID          string
	PolygonExitTime string
	ExitAt          string
}

// CompleteTripByGPS завершает рейс по GPS (когда машина выехала из полигона по GPS, но нет событий камер)
func (s *TripService) CompleteTripByGPS(ctx context.Context, input CompleteTripByGPSInput) (*model.Trip, error) {
	trip, err := s.tripRepo.GetByID(ctx, input.TripID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if trip.ExitAt != nil {
		// Рейс уже закрыт
		return nil, ErrConflict
	}

	if trip.PolygonEntryTime == nil {
		// Нет времени входа на полигон
		return nil, ErrInvalidInput
	}

	polygonExitTime, err := time.Parse(time.RFC3339, input.PolygonExitTime)
	if err != nil {
		return nil, ErrInvalidInput
	}

	exitAt, err := time.Parse(time.RFC3339, input.ExitAt)
	if err != nil {
		return nil, ErrInvalidInput
	}

	// Обновляем рейс
	trip.PolygonExitTime = &polygonExitTime
	trip.ExitAt = &exitAt

	// Если нет exit_lpr_event и exit_volume_event, создаем нарушение NO_EXIT_CAMERA
	if trip.ExitLprEventID == nil && trip.ExitVolumeEventID == nil {
		trip.Status = model.TripStatusNoExitCamera
	}

	if err := s.tripRepo.Update(ctx, trip); err != nil {
		return nil, err
	}

	// Проверяем, можно ли автоматически завершить тикет
	if trip.TicketID != nil && s.ticketService != nil {
		if err := s.ticketService.TryAutoComplete(ctx, *trip.TicketID); err != nil {
			// Логируем ошибку, но не прерываем выполнение
		}
	}

	return trip, nil
}

// Helper functions
func vehicleIDToString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func uuidToString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func stringPtrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
