package main

import (
	"fmt"
	"os"

	"ticket-service/internal/auth"
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

	ticketRepo := repository.NewTicketRepository(database)
	ticketService := service.NewTicketService(ticketRepo)

	tokenParser := auth.NewParser(cfg.Auth.AccessSecret)

	handler := httphandler.NewHandler(ticketService, appLogger)
	authMiddleware := middleware.Auth(tokenParser)
	router := httphandler.NewRouter(handler, authMiddleware, cfg.Environment)

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	appLogger.Info().Str("addr", addr).Msg("starting ticket service")

	if err := router.Run(addr); err != nil {
		appLogger.Error().Err(err).Msg("failed to start server")
		os.Exit(1)
	}
}
