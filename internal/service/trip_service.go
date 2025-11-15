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
