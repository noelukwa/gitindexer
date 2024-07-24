package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/noelukwa/indexer/internal/manager"
)

type RemoteHandler struct {
	service   *manager.Service
	validator *validator.Validate
}

func NewRemoteRepositoryHandler(service *manager.Service) *RemoteHandler {
	return &RemoteHandler{
		service:   service,
		validator: validator.New(),
	}
}

func (h *RemoteHandler) FetchTopCommitters(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	repo := c.QueryParam("repo")

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid limit parameter"})
	}

	committers, err := h.service.GetTopCommitters(c.Request().Context(), repo, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get top committers"})
	}
	return c.JSON(http.StatusOK, committers)
}

func (h *RemoteHandler) FetchRepoInfo(c echo.Context) error {
	repo := c.Param("name")
	intent, err := h.service.FindRepository(c.Request().Context(), repo)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update intent"})
	}

	return c.JSON(http.StatusOK, intent)
}
