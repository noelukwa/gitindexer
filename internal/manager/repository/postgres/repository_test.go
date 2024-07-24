package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/joho/godotenv/autoload"
	"github.com/noelukwa/indexer/internal/manager/models"
	"github.com/noelukwa/indexer/internal/manager/repository"
	"github.com/noelukwa/indexer/internal/manager/repository/postgres"
	"github.com/test-go/testify/require"
)

var (
	connStr = "postgres://indexer:explorer2025@localhost/manager-tests?sslmode=disable"
)

func setupDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	conn, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	return conn
}

func teardownDB(t *testing.T, conn *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := conn.Exec(ctx, "TRUNCATE TABLE intents, commits, authors, repositories RESTART IDENTITY CASCADE")
	require.NoError(t, err)
	conn.Close()
}

func TestMain(m *testing.M) {

	code := m.Run()

	os.Exit(code)
}

func TestSaveIntent(t *testing.T) {
	ctx := context.Background()
	conn := setupDB(t)
	defer teardownDB(t, conn)

	store, err := postgres.NewManagerStore(ctx, connStr)
	require.NoError(t, err)

	intent := models.Intent{
		ID:             uuid.New(),
		RepositoryName: "repo1",
		StartDate:      time.Now(),
		Status:         models.SuccessBroadCast,
		IsActive:       true,
	}

	savedIntent, err := store.SaveIntent(ctx, intent)
	require.NoError(t, err)
	require.NotNil(t, savedIntent)
	require.Equal(t, intent.ID, savedIntent.ID)
}

func TestUpdateIntent(t *testing.T) {
	ctx := context.Background()
	conn := setupDB(t)
	defer teardownDB(t, conn)

	store, err := postgres.NewManagerStore(ctx, connStr)
	require.NoError(t, err)

	intent := models.Intent{
		ID:             uuid.New(),
		RepositoryName: "repo1",
		StartDate:      time.Now(),
		Status:         models.SuccessBroadCast,
		IsActive:       true,
	}

	savedIntent, err := store.SaveIntent(ctx, intent)
	require.NoError(t, err)

	tomorrow := time.Now().Add(24 * time.Hour)
	update := models.IntentUpdate{
		ID:        savedIntent.ID,
		IsActive:  new(bool),
		Status:    &savedIntent.Status,
		StartDate: &tomorrow,
	}
	*update.IsActive = false

	updatedIntent, err := store.UpdateIntent(ctx, update)
	require.NoError(t, err)
	require.NotNil(t, updatedIntent)
	require.Equal(t, savedIntent.ID, updatedIntent.ID)
	require.Equal(t, *update.Status, updatedIntent.Status)
	require.Equal(t, *update.IsActive, updatedIntent.IsActive)
	require.Equal(t, update.StartDate.Unix(), updatedIntent.StartDate.Unix())
}

func TestSaveIntentError(t *testing.T) {
	ctx := context.Background()
	conn := setupDB(t)
	defer teardownDB(t, conn)

	store, err := postgres.NewManagerStore(ctx, connStr)
	require.NoError(t, err)

	intent := models.Intent{
		ID:             uuid.New(),
		RepositoryName: "repo1",
		StartDate:      time.Now(),
		Status:         models.PendingBroadCast,
		IsActive:       true,
	}

	savedIntent, err := store.SaveIntent(ctx, intent)
	require.NoError(t, err)

	intentError := models.IntentError{
		IntentID:  savedIntent.ID,
		CreatedAt: time.Now(),
		Message:   "error message",
	}

	err = store.SaveIntentError(ctx, intentError)
	require.NoError(t, err)
}

func TestFindIntents(t *testing.T) {
	ctx := context.Background()
	conn := setupDB(t)
	defer teardownDB(t, conn)

	store, err := postgres.NewManagerStore(ctx, connStr)
	require.NoError(t, err)

	intent1 := models.Intent{
		ID:             uuid.New(),
		RepositoryName: "repo1",
		StartDate:      time.Now(),
		Status:         models.PendingBroadCast,
		IsActive:       true,
	}

	intent2 := models.Intent{
		ID:             uuid.New(),
		RepositoryName: "repo2",
		StartDate:      time.Now(),
		Status:         models.PendingBroadCast,
		IsActive:       false,
	}

	_, err = store.SaveIntent(ctx, intent1)
	require.NoError(t, err)
	_, err = store.SaveIntent(ctx, intent2)
	require.NoError(t, err)

	filter := models.IntentFilter{}
	pagination := repository.Pagination{
		Page:    1,
		PerPage: 10,
	}

	result, err := store.FindIntents(ctx, filter, pagination)
	require.NoError(t, err)
	require.Len(t, result.Data, 2)
}

func TestFindIntent(t *testing.T) {
	ctx := context.Background()
	conn := setupDB(t)
	defer teardownDB(t, conn)

	store, err := postgres.NewManagerStore(ctx, connStr)
	require.NoError(t, err)

	intent := models.Intent{
		ID:             uuid.New(),
		RepositoryName: "repo1",
		StartDate:      time.Now(),
		Status:         models.PendingBroadCast,
		IsActive:       true,
	}

	savedIntent, err := store.SaveIntent(ctx, intent)
	require.NoError(t, err)

	foundIntent, err := store.FindIntent(ctx, savedIntent.ID)
	require.NoError(t, err)
	require.NotNil(t, foundIntent)
	require.Equal(t, savedIntent.ID, foundIntent.ID)
}

func TestSaveManyCommit(t *testing.T) {
	ctx := context.Background()
	conn := setupDB(t)
	defer teardownDB(t, conn)

	store, err := postgres.NewManagerStore(ctx, connStr)
	require.NoError(t, err)

	repo := &models.Repository{
		ID:        1,
		FullName:  "repo1",
		Watchers:  10,
		Stars:     5,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Language:  "Go",
		Forks:     2,
	}

	err = store.SaveRepo(ctx, repo)
	require.NoError(t, err)

	commits := []*models.Commit{
		{
			Hash:      "hash1",
			Author:    models.Author{ID: 200, Name: "Author1", Email: "author1@example.com", Username: "author1"},
			CreatedAt: time.Now(),
			Message:   "commit message 1",
		},
		{
			Hash:      "hash2",
			Author:    models.Author{ID: 800, Name: "Author2", Email: "author2@example.com", Username: "author2"},
			CreatedAt: time.Now(),
			Message:   "commit message 2",
		},
	}

	err = store.SaveManyCommit(ctx, repo.ID, commits)
	require.NoError(t, err)
}

func TestSaveRepo(t *testing.T) {
	ctx := context.Background()
	conn := setupDB(t)
	defer teardownDB(t, conn)

	store, err := postgres.NewManagerStore(ctx, connStr)
	require.NoError(t, err)

	repo := &models.Repository{
		ID:        345667,
		Watchers:  100,
		Stars:     200,
		FullName:  "repo1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Language:  "Go",
		Forks:     50,
	}

	err = store.SaveRepo(ctx, repo)
	require.NoError(t, err)
}

func TestGetRepo(t *testing.T) {
	ctx := context.Background()
	conn := setupDB(t)
	defer teardownDB(t, conn)

	store, err := postgres.NewManagerStore(ctx, connStr)
	require.NoError(t, err)

	repo := &models.Repository{
		ID:        87654,
		Watchers:  100,
		Stars:     200,
		FullName:  "repo1",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Language:  "Go",
		Forks:     50,
	}

	err = store.SaveRepo(ctx, repo)
	require.NoError(t, err)

	foundRepo, err := store.GetRepo(ctx, "repo1")
	require.NoError(t, err)
	require.NotNil(t, foundRepo)
	require.Equal(t, repo.ID, foundRepo.ID)
}

func TestFindCommits(t *testing.T) {
	ctx := context.Background()
	conn := setupDB(t)
	defer teardownDB(t, conn)

	store, err := postgres.NewManagerStore(ctx, connStr)
	require.NoError(t, err)

	repo := &models.Repository{
		ID:        1,
		FullName:  "repo1",
		Watchers:  10,
		Stars:     5,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Language:  "Go",
		Forks:     2,
	}

	err = store.SaveRepo(ctx, repo)
	require.NoError(t, err)

	commits := []*models.Commit{
		{
			Hash:      "hash1",
			Author:    models.Author{ID: 200, Name: "Author1", Email: "author1@example.com", Username: "author1"},
			CreatedAt: time.Now(),
			Message:   "commit message 1",
			Repository: models.Repository{
				ID: repo.ID,
			},
		},
		{
			Hash:      "hash2",
			Author:    models.Author{ID: 800, Name: "Author2", Email: "author2@example.com", Username: "author2"},
			CreatedAt: time.Now(),
			Message:   "commit message 2",
			Repository: models.Repository{
				ID: repo.ID,
			},
		},
	}

	err = store.SaveManyCommit(ctx, repo.ID, commits)
	require.NoError(t, err)

	filter := models.CommitsFilter{
		RepositoryName: repo.FullName,
	}

	pagination := repository.Pagination{
		Page:    1,
		PerPage: 10,
	}

	foundCommits, err := store.FindCommits(ctx, filter, pagination)
	require.NoError(t, err)
	require.Len(t, foundCommits.Data, 2)
}

func TestGetTopCommitters(t *testing.T) {
	ctx := context.Background()
	conn := setupDB(t)
	defer teardownDB(t, conn)

	store, err := postgres.NewManagerStore(ctx, connStr)
	require.NoError(t, err)

	// Seed the database with data
	repo := &models.Repository{
		ID:        1,
		FullName:  "test-repo",
		Watchers:  10,
		Stars:     5,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Language:  "Go",
		Forks:     2,
	}

	err = store.SaveRepo(ctx, repo)
	require.NoError(t, err)

	authors := []*models.Author{
		{
			ID:       5588,
			Name:     "Author1",
			Email:    "author1@example.com",
			Username: "author1",
		},
		{
			ID:       8877,
			Name:     "Author2",
			Email:    "author2@example.com",
			Username: "author2",
		},
	}

	for _, author := range authors {
		err = store.SaveAuthor(ctx, author)
		require.NoError(t, err)
	}

	commits := []*models.Commit{
		{
			Hash: "hash1",
			Author: models.Author{
				ID: authors[0].ID,
			},
			CreatedAt: time.Now().AddDate(0, 0, -10),
			Message:   "commit message 1",
			Repository: models.Repository{
				ID: repo.ID,
			},
		},
		{
			Hash: "hash2",
			Author: models.Author{
				ID: authors[1].ID,
			},
			CreatedAt: time.Now().AddDate(0, 0, -5),
			Message:   "commit message 2",
			Repository: models.Repository{
				ID: repo.ID,
			},
		},
		{
			Hash: "hash3",
			Author: models.Author{
				ID: authors[0].ID,
			},
			CreatedAt: time.Now().AddDate(0, 0, -1),
			Message:   "commit message 3",
			Repository: models.Repository{
				ID: repo.ID,
			},
		},
	}

	err = store.SaveManyCommit(ctx, repo.ID, commits)
	require.NoError(t, err)
	startDate := time.Now().AddDate(0, -1, 0) // 1 month ago
	endDate := time.Now()
	pagination := repository.Pagination{
		Page:    1,
		PerPage: 10,
	}

	result, err := store.GetTopCommitters(ctx, repo.FullName, &startDate, &endDate, pagination)
	require.NoError(t, err)

	require.NotNil(t, result)
	require.NotEmpty(t, result.Data)
	require.Equal(t, pagination.Page, result.Page)
	require.Equal(t, pagination.PerPage, result.PerPage)
	require.True(t, result.TotalCount > 0)

	for _, stat := range result.Data {
		if stat.Author.ID == authors[0].ID {
			require.True(t, stat.Commits > 1)
		}
		require.NotEmpty(t, stat.Author.ID)
		require.NotEmpty(t, stat.Author.Name)
		require.NotEmpty(t, stat.Author.Email)
		require.NotEmpty(t, stat.Author.Username)
		require.True(t, stat.Commits > 0)
	}
}
