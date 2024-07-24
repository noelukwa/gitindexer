package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/noelukwa/indexer/internal/manager/models"
	"github.com/noelukwa/indexer/internal/manager/repository"
	"github.com/noelukwa/indexer/internal/manager/repository/postgres/sqlc"
	"github.com/pressly/goose/v3"
)

type pgStore struct {
	conn *pgxpool.Pool
	q    *sqlc.Queries
}

//go:embed migrations/*.sql
var migrations embed.FS

func NewManagerStore(ctx context.Context, connStr string) (repository.ManagerStore, error) {
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string: %w", err)
	}

	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	conn, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	store := &pgStore{
		conn: conn,
		q:    sqlc.New(conn),
	}

	log.Println("running database migrations...")
	if err := store.runMigrate(conn); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return store, nil
}

func (p *pgStore) runMigrate(conn *pgxpool.Pool) error {
	goose.SetBaseFS(migrations)

	if err := goose.SetDialect("postgres"); err != nil {
		log.Printf("failed to set goose dialect: %v", err)
		return err
	}

	db := conn.Config().ConnConfig.ConnString()

	dbConn, err := goose.OpenDBWithDriver("pgx", db)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	defer dbConn.Close()

	if err := goose.Up(dbConn, "migrations"); err != nil {
		log.Printf("failed to run goose migrations: %v", err)
		return err
	}

	return nil
}

func (p *pgStore) SaveIntent(ctx context.Context, freshIntent models.Intent) (*models.Intent, error) {
	intent, err := p.q.SaveIntent(ctx, sqlc.SaveIntentParams{
		ID:             freshIntent.ID,
		RepositoryName: freshIntent.RepositoryName,
		StartDate: pgtype.Timestamptz{
			Time:  freshIntent.StartDate,
			Valid: true,
		},
		Status:   sqlc.IntentStatus(freshIntent.Status),
		IsActive: freshIntent.IsActive,
	})
	if err != nil {
		return nil, err
	}

	return &models.Intent{
		ID:             intent.ID,
		RepositoryName: intent.RepositoryName,
		StartDate:      intent.StartDate.Time,
		Status:         models.IntentStatus(intent.Status),
		IsActive:       intent.IsActive,
	}, nil
}

func (p *pgStore) UpdateIntent(ctx context.Context, update models.IntentUpdate) (*models.Intent, error) {
	params := sqlc.UpdateIntentParams{}
	if update.Status != nil {
		params.Status = sqlc.IntentStatus(*update.Status)
	}

	if update.IsActive != nil {
		params.IsActive = *update.IsActive
	}
	if update.StartDate != nil {
		params.StartDate = pgtype.Timestamptz{
			Time:  *update.StartDate,
			Valid: true,
		}
	}

	intent, err := p.q.UpdateIntent(ctx, sqlc.UpdateIntentParams{
		ID:        update.ID,
		Status:    params.Status,
		IsActive:  params.IsActive,
		StartDate: params.StartDate,
	})
	if err != nil {
		return nil, err
	}

	return &models.Intent{
		ID:             intent.ID,
		RepositoryName: intent.RepositoryName,
		StartDate:      intent.StartDate.Time,
		Status:         models.IntentStatus(intent.Status),
		IsActive:       intent.IsActive,
	}, nil
}

func (p *pgStore) SaveIntentError(ctx context.Context, err models.IntentError) error {

	return p.q.SaveIntentError(ctx, sqlc.SaveIntentErrorParams{
		ID:       uuid.New(),
		IntentID: err.IntentID,
		CreatedAt: pgtype.Timestamptz{
			Time:  err.CreatedAt,
			Valid: true,
		},
		Message: err.Message,
	})
}

func (p *pgStore) FindIntents(ctx context.Context, filter models.IntentFilter, pag repository.Pagination) (repository.Paginated[models.Intent], error) {

	sb := squirrel.Select(
		"i.id",
		"i.repository_name",
		"i.start_date",
		"i.status",
		"i.is_active",
	).From("intents i")

	if filter.Status != nil {
		sb = sb.Where(squirrel.Eq{"i.status": *filter.Status})
	}
	if filter.IsActive != nil {
		sb = sb.Where(squirrel.Eq{"i.is_active": *filter.IsActive})
	}
	if filter.RepositoryName != nil {
		sb = sb.Where(squirrel.Eq{"i.repository_name": *filter.RepositoryName})
	}

	countBuilder := sb.PlaceholderFormat(squirrel.Dollar).Prefix("SELECT COUNT(*) FROM (").Suffix(") AS subquery")
	totalCountSQL, args, err := countBuilder.ToSql()
	if err != nil {
		return repository.Paginated[models.Intent]{}, fmt.Errorf("failed to build count SQL: %w", err)
	}

	var totalCount int64
	err = p.conn.QueryRow(ctx, totalCountSQL, args...).Scan(&totalCount)
	if err != nil {
		return repository.Paginated[models.Intent]{}, fmt.Errorf("failed to get total count: %w", err)
	}

	sb = sb.Offset(uint64((pag.Page - 1) * pag.PerPage)).Limit(uint64(pag.PerPage)).PlaceholderFormat(squirrel.Dollar)
	sql, args, err := sb.ToSql()
	if err != nil {
		return repository.Paginated[models.Intent]{}, fmt.Errorf("failed to build SQL: %w", err)
	}

	rows, err := p.conn.Query(ctx, sql, args...)
	if err != nil {
		return repository.Paginated[models.Intent]{}, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	intents := []models.Intent{}
	for rows.Next() {
		var intent models.Intent

		err = rows.Scan(
			&intent.ID,
			&intent.RepositoryName,
			&intent.StartDate,
			&intent.Status,
			&intent.IsActive,
		)
		if err != nil {
			return repository.Paginated[models.Intent]{}, fmt.Errorf("failed to scan row: %w", err)
		}

		intents = append(intents, intent)
	}

	return repository.Paginated[models.Intent]{
		Data:       intents,
		TotalCount: totalCount,
		Page:       pag.Page,
		PerPage:    pag.PerPage,
	}, nil
}

func (p *pgStore) FindIntent(ctx context.Context, id uuid.UUID) (*models.Intent, error) {
	intent, err := p.q.FindIntent(ctx, id)
	if err != nil {
		return nil, err
	}

	return &models.Intent{
		ID:             intent.ID,
		RepositoryName: intent.RepositoryName,
		StartDate:      intent.StartDate.Time,
		Status:         models.IntentStatus(intent.Status),
		IsActive:       intent.IsActive,
	}, nil
}

func (p *pgStore) SaveManyCommit(ctx context.Context, repoID int64, commits []*models.Commit) error {
	tx, err := p.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := p.q.WithTx(tx)

	for _, commit := range commits {
		author, err := qtx.GetAuthor(ctx, commit.Author.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			continue
		}

		if errors.Is(err, pgx.ErrNoRows) {
			author, err = qtx.SaveAuthor(ctx, sqlc.SaveAuthorParams{
				ID:       commit.Author.ID,
				Name:     commit.Author.Name,
				Email:    commit.Author.Email,
				Username: commit.Author.Username,
			})
			if err != nil {
				return fmt.Errorf("failed to save author %s: %w", commit.Author.Username, err)
			}
		}

		err = qtx.SaveCommit(ctx, sqlc.SaveCommitParams{
			Hash:         commit.Hash,
			AuthorID:     author.ID,
			CreatedAt:    pgtype.Timestamptz{Time: commit.CreatedAt, Valid: true},
			Message:      commit.Message,
			RepositoryID: repoID,
		})
		if err != nil {
			return fmt.Errorf("failed to save commit %s: %w", commit.Hash, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (p *pgStore) SaveRepo(ctx context.Context, repo *models.Repository) error {
	var createdAt, updatedAt pgtype.Timestamptz
	createdAt.Time = repo.CreatedAt
	createdAt.Valid = true
	updatedAt.Time = repo.UpdatedAt
	updatedAt.Valid = true

	return p.q.SaveRepo(ctx, sqlc.SaveRepoParams{
		ID:         repo.ID,
		Watchers:   int32(repo.Watchers),
		Stargazers: int32(repo.Stars),
		FullName:   repo.FullName,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
		Language:   pgtype.Text{String: repo.Language, Valid: true},
		Forks:      int32(repo.Forks),
	})
}

func (p *pgStore) GetRepo(ctx context.Context, name string) (*models.Repository, error) {
	repo, err := p.q.GetRepo(ctx, name)
	if err != nil {
		return nil, err
	}

	return &models.Repository{
		ID:        repo.ID,
		Watchers:  repo.Watchers,
		Stars:     repo.Stargazers,
		FullName:  repo.FullName,
		CreatedAt: repo.CreatedAt.Time,
		UpdatedAt: repo.UpdatedAt.Time,
		Language:  repo.Language.String,
		Forks:     repo.Forks,
	}, nil
}

func (p *pgStore) FindCommits(ctx context.Context, filter models.CommitsFilter, pagination repository.Pagination) (repository.Paginated[models.Commit], error) {
	var startDate, endDate pgtype.Timestamptz

	// Set startDate if filter.StartDate is provided and not zero
	if filter.StartDate != nil && !filter.StartDate.IsZero() {
		startDate.Time = *filter.StartDate
		startDate.Valid = true
	}

	// Set endDate if filter.EndDate is provided and not zero
	if filter.EndDate != nil && !filter.EndDate.IsZero() {
		endDate.Time = *filter.EndDate
		endDate.Valid = true
	}

	// Execute the FindCommits query
	rows, err := p.q.FindCommits(ctx, sqlc.FindCommitsParams{
		FullName: filter.RepositoryName,
		Column2:  startDate,
		Column3:  endDate,
		Limit:    int32(pagination.PerPage),
		Offset:   int32((pagination.Page - 1) * pagination.PerPage),
	})
	if err != nil {
		return repository.Paginated[models.Commit]{}, err
	}

	var commits []models.Commit
	for _, row := range rows {
		commits = append(commits, models.Commit{
			Hash:      row.Hash,
			Message:   row.Message,
			Url:       parseURL(row.Url),
			CreatedAt: row.CreatedAt.Time,
			Repository: models.Repository{
				ID:        row.RepoID,
				Watchers:  row.Watchers,
				Stars:     row.Stargazers,
				FullName:  row.Repository,
				CreatedAt: row.RepoCreatedAt.Time,
				UpdatedAt: row.RepoUpdatedAt.Time,
				Language:  row.Language.String,
				Forks:     row.Forks,
			},
			Author: models.Author{
				ID:       row.AuthorID,
				Name:     row.AuthorName,
				Email:    row.AuthorEmail,
				Username: row.AuthorUsername,
			},
		})
	}

	// Get the total count of commits matching the filter
	totalCount, err := p.q.CountCommits(ctx, sqlc.CountCommitsParams{
		FullName: filter.RepositoryName,
		Column2:  startDate,
		Column3:  endDate,
	})
	if err != nil {
		return repository.Paginated[models.Commit]{}, err
	}

	return repository.Paginated[models.Commit]{
		Data:       commits,
		TotalCount: totalCount,
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
	}, nil
}

func (p *pgStore) GetTopCommitters(ctx context.Context, repo string, startDate, endDate *time.Time, pagination repository.Pagination) (repository.Paginated[models.AuthorStats], error) {
	var start, end pgtype.Timestamptz
	if startDate != nil {
		start.Time = *startDate
		start.Valid = true
	}
	if endDate != nil {
		end.Time = *endDate
		end.Valid = true
	}

	rows, err := p.q.GetTopCommitters(ctx, sqlc.GetTopCommittersParams{
		FullName: repo,
		Column2:  start,
		Column3:  end,
		Limit:    int32(pagination.PerPage),
		Offset:   int32((pagination.Page - 1) * pagination.PerPage),
	})
	if err != nil {
		return repository.Paginated[models.AuthorStats]{}, err
	}

	var stats []models.AuthorStats
	for _, row := range rows {
		stats = append(stats, models.AuthorStats{
			Author: models.Author{
				ID:       row.ID,
				Name:     row.Name,
				Email:    row.Email,
				Username: row.Username,
			},
			Commits: row.CommitCount,
		})
	}

	return repository.Paginated[models.AuthorStats]{
		Data:       stats,
		TotalCount: int64(len(stats)),
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
	}, nil
}
func (p *pgStore) SaveAuthor(ctx context.Context, author models.Author) error {
	_, err := p.q.SaveAuthor(ctx, sqlc.SaveAuthorParams{
		ID:       author.ID,
		Name:     author.Name,
		Email:    author.Email,
		Username: author.Username,
	})
	return err
}

func stringOrNull(str *string) string {
	if str == nil {
		return ""
	}
	return *str
}

func parseURL(rawURL pgtype.Text) *url.URL {

	if rawURL.Valid {
		u, _ := url.Parse(rawURL.String)
		return u
	}
	return nil
}
