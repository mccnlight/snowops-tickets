package main

import (
	"fmt"
	"os"

	"ticket-service/internal/auth"
	"ticket-service/internal/client"
	"ticket-service/internal/config"
	"ticket-service/internal/db"
	httphandler "ticket-service/internal/http"
	"ticket-service/internal/http/middleware"
	"ticket-service/internal/logger"
	"ticket-service/internal/repository"
	"ticket-service/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	appLogger := logger.New(cfg.Environment)

	database, err := db.New(cfg, appLogger)
	if err != nil {
		appLogger.Fatal().Err(err).Msg("failed to connect database")
	}

	// Repositories
	ticketRepo := repository.NewTicketRepository(database)
	assignmentRepo := repository.NewAssignmentRepository(database)
	tripRepo := repository.NewTripRepository(database)
	appealRepo := repository.NewAppealRepository(database)

	// Clients
	anprClient := client.NewANPRClient(cfg)

	// Services (нужно создать TripService до AssignmentService, т.к. AssignmentService зависит от TripService)
	ticketService := service.NewTicketService(ticketRepo, tripRepo, assignmentRepo, appealRepo)
	tripService := service.NewTripService(tripRepo, ticketRepo, assignmentRepo, ticketService, anprClient, appLogger)
	assignmentService := service.NewAssignmentService(assignmentRepo, ticketRepo, ticketService, tripService)
	appealService := service.NewAppealService(appealRepo, tripRepo, ticketRepo, assignmentRepo)

	tokenParser := auth.NewParser(cfg.Auth.AccessSecret)

	handler := httphandler.NewHandler(ticketService, assignmentService, tripService, appealService, appLogger)
	authMiddleware := middleware.Auth(tokenParser)
	router := httphandler.NewRouter(handler, authMiddleware, cfg.Environment)

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	appLogger.Info().Str("addr", addr).Msg("starting ticket service")

	if err := router.Run(addr); err != nil {
		appLogger.Error().Err(err).Msg("failed to start server")
		os.Exit(1)
	}
}
