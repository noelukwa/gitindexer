package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/noelukwa/indexer/internal/manager/models"
)

type Paginated[T any] struct {
	Data       []T
	TotalCount int64
	Page       int
	PerPage    int
}

type Pagination struct {
	Page    int
	PerPage int
}

type ManagerStore interface {
	SaveIntent(ctx context.Context, freshIntent models.Intent) (intent *models.Intent, err error)
	UpdateIntent(ctx context.Context, update models.IntentUpdate) (intent *models.Intent, err error)
	SaveIntentError(ctx context.Context, err models.IntentError) error
	FindIntents(ctx context.Context, filter models.IntentFilter, pag Pagination) (Paginated[models.Intent], error)
	FindIntent(ctx context.Context, id uuid.UUID) (*models.Intent, error)
	SaveRepo(ctx context.Context, repo *models.Repository) error
	GetRepo(ctx context.Context, name string) (*models.Repository, error)
	FindCommits(ctx context.Context, filter models.CommitsFilter, pag Pagination) (Paginated[models.Commit], error)
	GetTopCommitters(ctx context.Context, repository string, startDate, endDate *time.Time, pagination Pagination) (Paginated[models.AuthorStats], error)
	SaveManyCommit(ctx context.Context, repoID int64, commit []*models.Commit) error
	SaveAuthor(ctx context.Context, author *models.Author) error
}
