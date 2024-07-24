package handlers

import (
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/noelukwa/indexer/internal/manager"
	"github.com/noelukwa/indexer/internal/manager/models"
)

// RemoteHandler handles HTTP requests related to remote repositories
type RemoteHandler struct {
	service   *manager.Service
	validator *validator.Validate
}

// NewRemoteRepositoryHandler creates a new RemoteHandler instance
func NewRemoteRepositoryHandler(service *manager.Service) *RemoteHandler {
	return &RemoteHandler{
		service:   service,
		validator: validator.New(),
	}
}

// TopCommittersRequest represents the request parameters for fetching top committers
type TopCommittersRequest struct {
	Repo    string `query:"repo" validate:"required"`
	Page    int    `query:"page" validate:"required,min=1"`
	PerPage int    `query:"per_page" validate:"required,min=1,max=100"`
}

// TopCommittersResponse represents the response for top committers
type TopCommittersResponse struct {
	Data       []models.AuthorStats `json:"data"`
	TotalCount int64                `json:"total_count"`
	Page       int                  `json:"page"`
	PerPage    int                  `json:"per_page"`
}

// FetchTopCommitters godoc
// @Summary Fetch the top committers in a repository
// @Description Get a paginated list of top committers for a specified repository
// @Tags repos
// @Accept json
// @Produce json
// @Param repo query string true "Repository name in the format 'owner/repo'"
// @Param page query int true "Page number for pagination" minimum(1)
// @Param per_page query int true "Number of items per page" minimum(1) maximum(100)
// @Success 200 {object} TopCommittersResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /repos/top-committers [get]
func (h *RemoteHandler) FetchTopCommitters(c echo.Context) error {
	var req TopCommittersRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request parameters"})
	}

	if err := h.validator.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}

	paginatedResult, err := h.service.GetTopCommitters(c.Request().Context(), req.Repo, req.Page, req.PerPage)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: fmt.Sprintf("Failed to get top committers: %v", err)})
	}

	response := TopCommittersResponse{
		Data:       paginatedResult.Data,
		TotalCount: paginatedResult.TotalCount,
		Page:       paginatedResult.Page,
		PerPage:    paginatedResult.PerPage,
	}

	return c.JSON(http.StatusOK, response)
}

// FetchRepoInfo godoc
// @Summary Fetch repository information
// @Description Get detailed information about a specific repository
// @Tags repos
// @Accept json
// @Produce json
// @Param owner path string true "Repository owner"
// @Param name path string true "Repository name"
// @Success 200 {object} models.Repository
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /repos/{owner}/{name} [get]
func (h *RemoteHandler) FetchRepoInfo(c echo.Context) error {
	owner := c.Param("owner")
	name := c.Param("name")
	repoInfo, err := h.service.FindRepository(c.Request().Context(), fmt.Sprintf("%s/%s", owner, name))
	if err != nil {
		if err == manager.ErrRepositoryNotFound {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: "Repository not found"})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch repository information"})
	}

	return c.JSON(http.StatusOK, repoInfo)
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}
