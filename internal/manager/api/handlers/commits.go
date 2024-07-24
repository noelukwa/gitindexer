package handlers

import (
	"fmt"
	"net/http"

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

type TopCommittersRequest struct {
	Repo    string `query:"repo" validate:"required"`
	Page    int    `query:"page" validate:"required,min=1"`
	PerPage int    `query:"per_page" validate:"required,min=1,max=100"`
}

func (h *RemoteHandler) FetchTopCommitters(c echo.Context) error {
	var req TopCommittersRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request parameters"})
	}

	if err := h.validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	committers, err := h.service.GetTopCommitters(c.Request().Context(), req.Repo, req.Page, req.PerPage)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to get top committers: %v", err)})
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
