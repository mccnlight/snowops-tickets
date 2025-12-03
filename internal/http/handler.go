package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"ticket-service/internal/http/middleware"
	"ticket-service/internal/model"
	"ticket-service/internal/repository"
	"ticket-service/internal/service"
)

type Handler struct {
	ticketService     *service.TicketService
	assignmentService *service.AssignmentService
	tripService       *service.TripService
	appealService     *service.AppealService
	log               zerolog.Logger
}

func NewHandler(
	ticketService *service.TicketService,
	assignmentService *service.AssignmentService,
	tripService *service.TripService,
	appealService *service.AppealService,
	log zerolog.Logger,
) *Handler {
	return &Handler{
		ticketService:     ticketService,
		assignmentService: assignmentService,
		tripService:       tripService,
		appealService:     appealService,
		log:               log,
	}
}

func (h *Handler) Register(r *gin.Engine, authMiddleware gin.HandlerFunc) {
	protected := r.Group("/")
	protected.Use(authMiddleware)

	akimat := protected.Group("/akimat")
	{
		akimat.GET("/tickets", h.listTickets)
		akimat.GET("/tickets/:id", h.getTicketDetails)
	}

	// KGU ZKH (TOO) - создание и управление тикетами
	kgu := protected.Group("/kgu")
	{
		kgu.GET("/tickets", h.listTickets)
		kgu.POST("/tickets", h.createTicket)
		kgu.GET("/tickets/:id", h.getTicketDetails)
		kgu.PUT("/tickets/:id/cancel", h.cancelTicket)
		kgu.PUT("/tickets/:id/close", h.closeTicket)
		kgu.DELETE("/tickets/:id", h.deleteTicket)
	}

	contractor := protected.Group("/contractor")
	{
		contractor.GET("/tickets", h.listTickets)
		contractor.GET("/tickets/:id", h.getTicketDetails)
		contractor.PUT("/tickets/:id/complete", h.completeTicket)
		// Назначения
		contractor.POST("/tickets/:id/assignments", h.createAssignment)
		contractor.DELETE("/assignments/:id", h.deleteAssignment)
		contractor.GET("/tickets/:id/assignments", h.listAssignments)
	}

	driver := protected.Group("/driver")
	{
		driver.GET("/tickets", h.listTickets)
		driver.GET("/tickets/:id", h.getTicketDetails)
		// Обновление статуса водителя
		driver.PUT("/assignments/:id/mark-in-work", h.markAssignmentInWork)
		driver.PUT("/assignments/:id/mark-completed", h.markAssignmentCompleted)
		// Обжалования
		driver.POST("/appeals", h.createAppeal)
		driver.GET("/appeals", h.listMyAppeals)
		driver.GET("/appeals/:id", h.getAppeal)
		driver.POST("/appeals/:id/comments", h.addAppealComment)
		driver.GET("/appeals/:id/comments", h.getAppealComments)
	}

	// LANDFILL - журнал приёма снега
	landfill := protected.Group("/landfill")
	{
		landfill.GET("/reception-journal", h.getReceptionJournal)
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
		ContractID     string `json:"contract_id" binding:"required"`
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
		ContractID:     req.ContractID,
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

func (h *Handler) getTicketDetails(c *gin.Context) {
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

	details, err := h.ticketService.GetDetails(c.Request.Context(), principal, id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(details))
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

	contractID := strings.TrimSpace(c.Query("contract_id"))
	if contractID != "" {
		filter.ContractID = &contractID
	}

	plannedStartFrom := strings.TrimSpace(c.Query("planned_start_from"))
	if plannedStartFrom != "" {
		filter.PlannedStartFrom = &plannedStartFrom
	}

	plannedStartTo := strings.TrimSpace(c.Query("planned_start_to"))
	if plannedStartTo != "" {
		filter.PlannedStartTo = &plannedStartTo
	}

	plannedEndFrom := strings.TrimSpace(c.Query("planned_end_from"))
	if plannedEndFrom != "" {
		filter.PlannedEndFrom = &plannedEndFrom
	}

	plannedEndTo := strings.TrimSpace(c.Query("planned_end_to"))
	if plannedEndTo != "" {
		filter.PlannedEndTo = &plannedEndTo
	}

	factStartFrom := strings.TrimSpace(c.Query("fact_start_from"))
	if factStartFrom != "" {
		filter.FactStartFrom = &factStartFrom
	}

	factStartTo := strings.TrimSpace(c.Query("fact_start_to"))
	if factStartTo != "" {
		filter.FactStartTo = &factStartTo
	}

	factEndFrom := strings.TrimSpace(c.Query("fact_end_from"))
	if factEndFrom != "" {
		filter.FactEndFrom = &factEndFrom
	}

	factEndTo := strings.TrimSpace(c.Query("fact_end_to"))
	if factEndTo != "" {
		filter.FactEndTo = &factEndTo
	}

	tickets, err := h.ticketService.List(c.Request.Context(), principal, filter)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(tickets))
}

func (h *Handler) cancelTicket(c *gin.Context) {
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

	if err := h.ticketService.Cancel(c.Request.Context(), principal, id); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(gin.H{"message": "ticket cancelled"}))
}

func (h *Handler) closeTicket(c *gin.Context) {
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

	if err := h.ticketService.Close(c.Request.Context(), principal, id); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(gin.H{"message": "ticket closed"}))
}

func (h *Handler) deleteTicket(c *gin.Context) {
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

	if err := h.ticketService.Delete(c.Request.Context(), principal, id); err != nil {
		h.handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) completeTicket(c *gin.Context) {
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

	if err := h.ticketService.Complete(c.Request.Context(), principal, id); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(gin.H{"message": "ticket completed"}))
}

// Assignment handlers
func (h *Handler) createAssignment(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	ticketID := strings.TrimSpace(c.Param("id"))
	if ticketID == "" {
		c.JSON(http.StatusBadRequest, errorResponse("invalid ticket id"))
		return
	}

	var req struct {
		DriverID  string `json:"driver_id" binding:"required"`
		VehicleID string `json:"vehicle_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	assignment, err := h.assignmentService.Create(c.Request.Context(), principal, service.CreateAssignmentInput{
		TicketID:  ticketID,
		DriverID:  req.DriverID,
		VehicleID: req.VehicleID,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, successResponse(assignment))
}

func (h *Handler) deleteAssignment(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, errorResponse("invalid assignment id"))
		return
	}

	if err := h.assignmentService.Delete(c.Request.Context(), principal, id); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(gin.H{"message": "assignment deleted"}))
}

func (h *Handler) listAssignments(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	ticketID := strings.TrimSpace(c.Param("id"))
	if ticketID == "" {
		c.JSON(http.StatusBadRequest, errorResponse("invalid ticket id"))
		return
	}

	assignments, err := h.assignmentService.ListByTicketID(c.Request.Context(), principal, ticketID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(assignments))
}

func (h *Handler) markAssignmentInWork(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, errorResponse("invalid assignment id"))
		return
	}

	if err := h.assignmentService.UpdateDriverMarkStatus(c.Request.Context(), principal, id, model.DriverMarkStatusInWork); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(gin.H{"message": "marked as in work"}))
}

func (h *Handler) markAssignmentCompleted(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, errorResponse("invalid assignment id"))
		return
	}

	if err := h.assignmentService.UpdateDriverMarkStatus(c.Request.Context(), principal, id, model.DriverMarkStatusCompleted); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(gin.H{"message": "marked as completed"}))
}

// Appeal handlers
func (h *Handler) createAppeal(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	var req struct {
		TripID           string `json:"trip_id" binding:"required"`
		AppealReasonType string `json:"appeal_reason_type" binding:"required"`
		Comment          string `json:"comment" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	appeal, err := h.appealService.Create(c.Request.Context(), principal, service.CreateAppealInput{
		TripID:           req.TripID,
		AppealReasonType: req.AppealReasonType,
		Comment:          req.Comment,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, successResponse(appeal))
}

func (h *Handler) listMyAppeals(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	ticketIDParam := strings.TrimSpace(c.Query("ticket_id"))
	var ticketID *string
	if ticketIDParam != "" {
		ticketID = &ticketIDParam
	}

	appeals, err := h.appealService.ListDriverAppeals(c.Request.Context(), principal, ticketID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(appeals))
}

func (h *Handler) getAppeal(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, errorResponse("invalid appeal id"))
		return
	}

	appeal, err := h.appealService.GetByID(c.Request.Context(), principal, id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(appeal))
}

func (h *Handler) addAppealComment(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, errorResponse("invalid appeal id"))
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	if err := h.appealService.AddComment(c.Request.Context(), principal, id, req.Content); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, successResponse(gin.H{"message": "comment added"}))
}

func (h *Handler) getAppealComments(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, errorResponse("invalid appeal id"))
		return
	}

	comments, err := h.appealService.GetComments(c.Request.Context(), principal, id)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(comments))
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

func (h *Handler) getReceptionJournal(c *gin.Context) {
	principal, ok := middleware.MustPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse("missing principal"))
		return
	}

	// Парсинг polygon_ids из query параметра (может быть несколько)
	var polygonIDs []uuid.UUID
	if raw := c.Query("polygon_ids"); raw != "" {
		ids := strings.Split(raw, ",")
		for _, idStr := range ids {
			id, err := uuid.Parse(strings.TrimSpace(idStr))
			if err == nil {
				polygonIDs = append(polygonIDs, id)
			}
		}
	}

	// Парсинг дат
	var dateFrom *time.Time
	if raw := c.Query("date_from"); raw != "" {
		t, err := parseTime(raw)
		if err == nil {
			dateFrom = &t
		}
	}

	var dateTo *time.Time
	if raw := c.Query("date_to"); raw != "" {
		t, err := parseTime(raw)
		if err == nil {
			dateTo = &t
		}
	}

	// Парсинг contractor_id
	var contractorID *uuid.UUID
	if raw := c.Query("contractor_id"); raw != "" {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err == nil {
			contractorID = &id
		}
	}

	// Парсинг status
	var status *model.TripStatus
	if raw := c.Query("status"); raw != "" {
		s := model.TripStatus(strings.ToUpper(strings.TrimSpace(raw)))
		status = &s
	}

	result, err := h.tripService.GetReceptionJournal(c.Request.Context(), principal, service.ReceptionJournalInput{
		PolygonIDs:   polygonIDs,
		DateFrom:     dateFrom,
		DateTo:       dateTo,
		ContractorID: contractorID,
		Status:       status,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, successResponse(result))
}

func parseTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("invalid time format")
}
