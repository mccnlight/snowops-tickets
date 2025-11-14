package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"ticket-service/internal/http/middleware"
	"ticket-service/internal/model"
	"ticket-service/internal/repository"
	"ticket-service/internal/service"
)

type Handler struct {
	ticketService *service.TicketService
	log           zerolog.Logger
}

func NewHandler(ticketService *service.TicketService, log zerolog.Logger) *Handler {
	return &Handler{
		ticketService: ticketService,
		log:           log,
	}
}

func (h *Handler) Register(r *gin.Engine, authMiddleware gin.HandlerFunc) {
	protected := r.Group("/")
	protected.Use(authMiddleware)

	akimat := protected.Group("/akimat")
	{
		akimat.GET("/tickets", h.listTickets)
		akimat.POST("/tickets", h.createTicket)
		akimat.GET("/tickets/:id", h.getTicket)
	}

	kgu := protected.Group("/kgu")
	{
		kgu.GET("/tickets", h.listTickets)
		kgu.POST("/tickets", h.createTicket)
		kgu.GET("/tickets/:id", h.getTicket)
	}

	contractor := protected.Group("/contractor")
	{
		contractor.GET("/tickets", h.listTickets)
		contractor.GET("/tickets/:id", h.getTicket)
	}

	driver := protected.Group("/driver")
	{
		driver.GET("/tickets", h.listTickets)
		driver.GET("/tickets/:id", h.getTicket)
	}
}

func (h *Handler) createTicket(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	var req struct {
		CleaningAreaID string `json:"cleaning_area_id" binding:"required"`
		ContractorID   string `json:"contractor_id" binding:"required"`
		PlannedStartAt string `json:"planned_start_at" binding:"required"`
		PlannedEndAt   string `json:"planned_end_at" binding:"required"`
		Description    string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	ticket, err := h.ticketService.Create(c.Request.Context(), principal, service.CreateTicketInput{
		CleaningAreaID: req.CleaningAreaID,
		ContractorID:   req.ContractorID,
		PlannedStartAt: req.PlannedStartAt,
		PlannedEndAt:   req.PlannedEndAt,
		Description:    req.Description,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, successResponse(ticket))
}

func (h *Handler) getTicket(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, errorResponse("invalid ticket id"))
		return
	}

	ticket, err := h.ticketService.Get(c.Request.Context(), principal, id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(ticket))
}

func (h *Handler) listTickets(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	filter := repository.TicketListFilter{}

	status := strings.TrimSpace(c.Query("status"))
	if status != "" {
		ts := model.TicketStatus(strings.ToUpper(status))
		filter.Status = &ts
	}

	contractorID := strings.TrimSpace(c.Query("contractor_id"))
	if contractorID != "" {
		filter.ContractorID = &contractorID
	}

	cleaningAreaID := strings.TrimSpace(c.Query("cleaning_area_id"))
	if cleaningAreaID != "" {
		filter.CleaningAreaID = &cleaningAreaID
	}

	tickets, err := h.ticketService.List(c.Request.Context(), principal, filter)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(tickets))
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrPermissionDenied):
		c.JSON(http.StatusForbidden, errorResponse(err.Error()))
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, errorResponse(err.Error()))
	case errors.Is(err, service.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
	case errors.Is(err, service.ErrConflict):
		c.JSON(http.StatusConflict, errorResponse(err.Error()))
	default:
		h.log.Error().Err(err).Msg("handler error")
		c.JSON(http.StatusInternalServerError, errorResponse("internal error"))
	}
}

func successResponse(data interface{}) gin.H {
	return gin.H{
		"data": data,
	}
}

func errorResponse(message string) gin.H {
	return gin.H{
		"error": message,
	}
}
