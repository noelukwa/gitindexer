package postgres

import (
	"context"
	"embed"
	"fmt"
	"log"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
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

// FindCommits implements repository.ManagerStore.
func (p *pgStore) FindCommits(ctx context.Context, filter models.CommitsFilter, pag repository.Pagination) (repository.Paginated[models.Commit], error) {
	panic("unimplemented")
}

// GetRepo implements repository.ManagerStore.
func (p *pgStore) GetRepo(ctx context.Context, name string) (*models.Repository, error) {
	panic("unimplemented")
}

// GetTopCommitters implements repository.ManagerStore.
func (p *pgStore) GetTopCommitters(ctx context.Context, repository string, startDate *time.Time, endDate *time.Time, pagination repository.Pagination) (repository.Paginated[models.AuthorStats], error) {
	panic("unimplemented")
}

// SaveAuthor implements repository.ManagerStore.
func (p *pgStore) SaveAuthor(ctx context.Context, author models.Author) error {
	panic("unimplemented")
}

// SaveManyCommit implements repository.ManagerStore.
func (p *pgStore) SaveManyCommit(ctx context.Context, repoID int64, commit []models.Commit) error {
	panic("unimplemented")
}

// SaveRepo implements repository.ManagerStore.
func (p *pgStore) SaveRepo(ctx context.Context, repo *models.Repository) error {
	panic("unimplemented")
}

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
