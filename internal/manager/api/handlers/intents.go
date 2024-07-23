package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/noelukwa/indexer/internal/manager"
	"github.com/noelukwa/indexer/internal/manager/models"
)

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

type AddIntentRequest struct {
	Repository string `json:"repository" validate:"required"`
	Since      Since  `json:"since" validate:"required"`
}

func (h *IntentHandler) CreateIntent(c echo.Context) error {
	var request AddIntentRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if err := h.validator.Struct(request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	intent, err := h.service.CreateIntent(
		c.Request().Context(),
		request.Repository,
		time.Time(request.Since),
	)
	if err != nil {
		if errors.Is(err, manager.ErrInvalidRepository) || errors.Is(err, manager.ErrExistingIntent) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		log.Printf("error: %s", err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to add intent"})
	}

	return c.JSON(http.StatusCreated, intent)
}

type UpdateIntentRequest struct {
	IsActive bool  `json:"is_active"`
	Since    Since `json:"since"`
}

func (h *IntentHandler) UpdateIntent(c echo.Context) error {
	// idParam := c.Param("id")
	// // id, err := uuid.Parse(idParam)
	// if err != nil {
	// 	return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid intent ID"})
	// }

	var request UpdateIntentRequest
	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body!!"})
	}

	if err := h.validator.Struct(request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// since := time.Time(request.Since)
	// intentUpdate := models.IntentUpdate{
	// 	ID:       id,
	// 	IsActive: request.IsActive,
	// 	Since:    &since,
	// }

	// intent, err := h.intentService.UpdateIntent(c.Request().Context(), intentUpdate)
	// if err != nil {
	// 	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update intent"})
	// }

	return c.JSON(http.StatusOK, "intent")
}

func (h *IntentHandler) FetchIntent(c echo.Context) error {
	// idParam := c.Param("id")
	// id, err := uuid.Parse(idParam)
	// if err != nil {
	// 	return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid intent ID"})
	// }

	// intent, err := h.intentService.GetIntentById(c.Request().Context(), id)
	// if err != nil {
	// 	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update intent"})
	// }

	return c.JSON(http.StatusOK, "intent")
}

func (h *IntentHandler) FetchIntents(c echo.Context) error {

	flag := c.QueryParam("is_active")
	status := c.QueryParam("status")
	repoName := c.QueryParam("repository_name")

	filter := models.IntentFilter{}

	if flag != "" {
		boolValue, err := strconv.ParseBool(flag)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid is_active parameter"})
		}
		filter.IsActive = &boolValue
	}

	if strings.TrimSpace(status) != "" {
		intentStatus := models.IntentStatus(status)
		filter.Status = &intentStatus
	}

	if strings.TrimSpace(repoName) != "" {
		filter.RepositoryName = &repoName
	}

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil || page < 1 {
		page = 1
	}

	perPage, err := strconv.Atoi(c.QueryParam("per_page"))
	if err != nil || perPage < 1 {
		perPage = 10
	}

	paginatedIntents, err := h.service.GetIntents(c.Request().Context(), filter, perPage, page)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch intents"})
	}

	response := map[string]interface{}{
		"data":        paginatedIntents.Data,
		"total_count": paginatedIntents.TotalCount,
		"page":        paginatedIntents.Page,
		"per_page":    paginatedIntents.PerPage,
	}

	return c.JSON(http.StatusOK, response)
}
