package service

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"

	"ticket-service/internal/model"
	"ticket-service/internal/repository"
)

type NavigationService struct {
	assignmentRepo      *repository.AssignmentRepository
	vehicleRepo         *repository.VehicleRepository
	vehiclePositionRepo *repository.VehiclePositionRepository
	ticketRepo          *repository.TicketRepository
	cleaningAreaRepo    *repository.CleaningAreaRepository
	polygonRepo         *repository.PolygonRepository
}

func NewNavigationService(
	assignmentRepo *repository.AssignmentRepository,
	vehicleRepo *repository.VehicleRepository,
	vehiclePositionRepo *repository.VehiclePositionRepository,
	ticketRepo *repository.TicketRepository,
	cleaningAreaRepo *repository.CleaningAreaRepository,
	polygonRepo *repository.PolygonRepository,
) *NavigationService {
	return &NavigationService{
		assignmentRepo:      assignmentRepo,
		vehicleRepo:         vehicleRepo,
		vehiclePositionRepo: vehiclePositionRepo,
		ticketRepo:          ticketRepo,
		cleaningAreaRepo:    cleaningAreaRepo,
		polygonRepo:         polygonRepo,
	}
}

type RoutePoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type RouteOption struct {
	Label           string       `json:"label"`
	DistanceMeters  float64      `json:"distance_meters"`
	DurationSeconds int64        `json:"duration_seconds"`
	Points          []RoutePoint `json:"points"`
}

type NavigationHint struct {
	Mode         string        `json:"mode"`
	VehicleState string        `json:"vehicle_state"`
	Primary      RouteOption   `json:"primary_route"`
	Alternatives []RouteOption `json:"alternatives"`
}

func (s *NavigationService) BuildDriverHint(ctx context.Context, principal model.Principal, ticketID *string) (*NavigationHint, error) {
	if !principal.IsDriver() || principal.DriverID == nil {
		return nil, ErrPermissionDenied
	}

	assignment, err := s.assignmentRepo.FindActiveByDriver(ctx, *principal.DriverID)
	if err != nil {
		return nil, err
	}
	if assignment == nil {
		return nil, ErrNotFound
	}

	targetTicketID := assignment.TicketID

	if ticketID != nil && *ticketID != "" {
		parsed, err := uuid.Parse(*ticketID)
		if err == nil {
			targetTicketID = parsed
		}
	}

	ticket, err := s.ticketRepo.GetByID(ctx, targetTicketID.String())
	if err != nil {
		return nil, err
	}

	var targetArea *model.CleaningArea
	if ticket != nil {
		targetArea, err = s.cleaningAreaRepo.GetByID(ctx, ticket.CleaningAreaID)
		if err != nil {
			return nil, err
		}
	}

	vehicle, err := s.vehicleRepo.GetByID(ctx, assignment.VehicleID)
	if err != nil {
		return nil, err
	}
	if vehicle == nil {
		return nil, errors.New("vehicle not found")
	}

	position, err := s.vehiclePositionRepo.GetLastByVehicleID(ctx, vehicle.ID)
	if err != nil {
		return nil, err
	}
	if position == nil {
		return nil, errors.New("vehicle position unavailable")
	}

	goalPolygonID := vehicle.DefaultPolygonID
	if targetArea != nil {
		tmp := targetArea.DefaultPolygon
		goalPolygonID = &tmp
	}

	var goalPolygon *model.Polygon
	if goalPolygonID != nil {
		goalPolygon, _ = s.polygonRepo.GetByID(ctx, *goalPolygonID)
	}

	mode := "TO_CLEANING_AREA"
	if position.InsideCleaningArea {
		mode = "TO_POLYGON"
	}
	if position.InsidePolygon {
		mode = "ON_POLYGON"
	}

	state := "OUTSIDE"
	if position.InsideCleaningArea {
		state = "INSIDE_AREA"
	}
	if position.InsidePolygon {
		state = "INSIDE_POLYGON"
	}

	targetPoints := buildTargetPoints(position, targetArea, goalPolygon)
	routes := buildRoutes(position, targetPoints)

	hint := &NavigationHint{
		Mode:         mode,
		VehicleState: state,
		Primary:      routes[0],
	}
	if len(routes) > 1 {
		hint.Alternatives = routes[1:]
	}

	return hint, nil
}

func buildTargetPoints(pos *model.VehiclePosition, area *model.CleaningArea, polygon *model.Polygon) []RoutePoint {
	points := []RoutePoint{}
	if pos == nil {
		return points
	}

	if area != nil && area.EntryLat != nil && area.EntryLong != nil {
		points = append(points, RoutePoint{Latitude: *area.EntryLat, Longitude: *area.EntryLong})
	}
	if polygon != nil && polygon.CentroidLat != nil && polygon.CentroidLong != nil {
		points = append(points, RoutePoint{Latitude: *polygon.CentroidLat, Longitude: *polygon.CentroidLong})
	}

	if len(points) == 0 {
		points = append(points, RoutePoint{Latitude: pos.Latitude, Longitude: pos.Longitude})
	}
	return points
}

func buildRoutes(pos *model.VehiclePosition, targets []RoutePoint) []RouteOption {
	if len(targets) == 0 {
		return []RouteOption{{
			Label:           "Stay",
			DistanceMeters:  0,
			DurationSeconds: 0,
			Points: []RoutePoint{
				{Latitude: pos.Latitude, Longitude: pos.Longitude},
			},
		}}
	}

	var routes []RouteOption
	for i, target := range targets {
		distance := haversine(pos.Latitude, pos.Longitude, target.Latitude, target.Longitude)
		duration := int64(distance / 8.0 * 3.6) // assume 8 m/s average
		label := "Route"
		if i == 0 {
			label = "Primary"
		}
		routes = append(routes, RouteOption{
			Label:           label,
			DistanceMeters:  distance,
			DurationSeconds: duration,
			Points: []RoutePoint{
				{Latitude: pos.Latitude, Longitude: pos.Longitude},
				target,
			},
		})
	}

	return routes
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1Rad)*math.Cos(lat2Rad)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadius * c
}
