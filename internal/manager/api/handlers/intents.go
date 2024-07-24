package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/noelukwa/indexer/internal/manager"
	"github.com/noelukwa/indexer/internal/manager/models"
)

// Since is a custom type for handling date parsing
type Since time.Time

func (ct *Since) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*ct = Since(t)
	return nil
}

type IntentHandler struct {
	service   *manager.Service
	validator *validator.Validate
}

func NewIntentHandler(service *manager.Service) *IntentHandler {
	return &IntentHandler{
		service:   service,
		validator: validator.New(),
	}
}

// AddIntentRequest represents the request body for creating an intent
type AddIntentRequest struct {
	Repository string `json:"repository" validate:"required"`
	Since      Since  `json:"since" validate:"required"`
}

// CreateIntent godoc
// @Summary Create a new intent
// @Description Create a new intent for a repository
// @Tags intents
// @Accept json
// @Produce json
// @Param request body AddIntentRequest true "Intent creation request"
// @Success 201 {object} models.Intent
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /intents [post]
func (h *IntentHandler) CreateIntent(c echo.Context) error {
	var request AddIntentRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
	}

	if err := h.validator.Struct(request); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	intent, err := h.service.CreateIntent(
		c.Request().Context(),
		request.Repository,
		time.Time(request.Since),
	)
	if err != nil {
		if errors.Is(err, manager.ErrInvalidRepository) || errors.Is(err, manager.ErrExistingIntent) {
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		}
		log.Printf("Error creating intent: %s", err.Error())
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to add intent"})
	}

	return c.JSON(http.StatusCreated, intent)
}

// UpdateIntentRequest represents the request body for updating an intent
type UpdateIntentRequest struct {
	IsActive bool  `json:"is_active"`
	Since    Since `json:"since"`
}

// UpdateIntent godoc
// @Summary Update an existing intent
// @Description Update the details of an existing intent
// @Tags intents
// @Accept json
// @Produce json
// @Param id path string true "Intent ID"
// @Param request body UpdateIntentRequest true "Intent update request"
// @Success 200 {object} models.Intent
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /intents/{id} [put]
func (h *IntentHandler) UpdateIntent(c echo.Context) error {

	var request UpdateIntentRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
	}

	if err := h.validator.Struct(request); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, "Intent updated successfully")
}

// FetchIntent godoc
// @Summary Fetch a single intent
// @Description Get details of a specific intent by ID
// @Tags intents
// @Accept json
// @Produce json
// @Param id path string true "Intent ID"
// @Success 200 {object} models.Intent
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /intents/{id} [get]
func (h *IntentHandler) FetchIntent(c echo.Context) error {

	return c.JSON(http.StatusOK, "Intent details")
}

// FetchIntentsRequest represents the query parameters for fetching intents
type FetchIntentsRequest struct {
	IsActive       *bool                `query:"is_active" validate:"omitempty"`
	Status         *models.IntentStatus `query:"status" validate:"omitempty,oneof=pending active completed failed"`
	RepositoryName *string              `query:"repository_name" validate:"omitempty"`
	Page           int                  `query:"page" validate:"required,min=1"`
	PerPage        int                  `query:"per_page" validate:"required,min=1,max=100"`
}

// FetchIntents godoc
// @Summary Fetch multiple intents
// @Description Get a list of intents based on filter criteria
// @Tags intents
// @Accept json
// @Produce json
// @Param is_active query bool false "Filter by active status"
// @Param status query string false "Filter by intent status" Enums(pending, active, completed, failed)
// @Param repository_name query string false "Filter by repository name"
// @Param page query int true "Page number" minimum(1)
// @Param per_page query int true "Items per page" minimum(1) maximum(100)
// @Success 200 {object} PaginatedResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /intents [get]
func (h *IntentHandler) FetchIntents(c echo.Context) error {
	var request FetchIntentsRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid query parameters"})
	}

	if err := h.validator.Struct(request); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	filter := models.IntentFilter{
		IsActive:       request.IsActive,
		Status:         request.Status,
		RepositoryName: request.RepositoryName,
	}

	paginatedIntents, err := h.service.GetIntents(c.Request().Context(), filter, request.PerPage, request.Page)
	if err != nil {
		log.Printf("Error fetching intents: %v", err)
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch intents"})
	}

	response := PaginatedResponse{
		Data:       paginatedIntents.Data,
		TotalCount: paginatedIntents.TotalCount,
		Page:       paginatedIntents.Page,
		PerPage:    paginatedIntents.PerPage,
	}

	return c.JSON(http.StatusOK, response)
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	TotalCount int64       `json:"total_count"`
	Page       int         `json:"page"`
	PerPage    int         `json:"per_page"`
}
