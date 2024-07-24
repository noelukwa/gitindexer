package manager_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/noelukwa/indexer/internal/manager"
	"github.com/noelukwa/indexer/internal/manager/models"
	"github.com/noelukwa/indexer/internal/manager/repository"
	"github.com/noelukwa/indexer/internal/pkg/config"
	"github.com/stretchr/testify/mock"
	"github.com/test-go/testify/assert"
)

type MockStore struct {
	mock.Mock
}

func (m *MockStore) SaveIntent(ctx context.Context, freshIntent models.Intent) (*models.Intent, error) {
	args := m.Called(ctx, freshIntent)
	return args.Get(0).(*models.Intent), args.Error(1)
}

func (m *MockStore) UpdateIntent(ctx context.Context, update models.IntentUpdate) (*models.Intent, error) {
	args := m.Called(ctx, update)
	return args.Get(0).(*models.Intent), args.Error(1)
}

func (m *MockStore) SaveIntentError(ctx context.Context, err models.IntentError) error {
	args := m.Called(ctx, err)
	return args.Error(0)
}

func (m *MockStore) FindIntents(ctx context.Context, filter models.IntentFilter, pag repository.Pagination) (repository.Paginated[models.Intent], error) {
	args := m.Called(ctx, filter, pag)
	return args.Get(0).(repository.Paginated[models.Intent]), args.Error(1)
}

func (m *MockStore) FindIntent(ctx context.Context, id uuid.UUID) (*models.Intent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Intent), args.Error(1)
}

func (m *MockStore) SaveRepo(ctx context.Context, repo *models.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockStore) GetRepo(ctx context.Context, name string) (*models.Repository, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Repository), args.Error(1)
}

func (m *MockStore) FindCommits(ctx context.Context, filter models.CommitsFilter, pag repository.Pagination) (repository.Paginated[models.Commit], error) {
	args := m.Called(ctx, filter, pag)
	return args.Get(0).(repository.Paginated[models.Commit]), args.Error(1)
}

func (m *MockStore) GetTopCommitters(ctx context.Context, repo string, startDate, endDate *time.Time, pagination repository.Pagination) (repository.Paginated[models.AuthorStats], error) {
	args := m.Called(ctx, repo, startDate, endDate, pagination)
	return args.Get(0).(repository.Paginated[models.AuthorStats]), args.Error(1)
}

func (m *MockStore) SaveManyCommit(ctx context.Context, repoID int64, commits []*models.Commit) error {
	args := m.Called(ctx, repoID, commits)
	return args.Error(0)
}

func (m *MockStore) SaveAuthor(ctx context.Context, author *models.Author) error {
	args := m.Called(ctx, author)
	return args.Error(0)
}

// Helper function to create a new service instance
func newTestService(store repository.ManagerStore) *manager.Service {
	cfg := &config.ManagerConfig{
		IntentsQueueName: "test-queue",
	}
	return manager.NewService(store, cfg)
}

func TestCreateIntent(t *testing.T) {

	store := new(MockStore)
	service := newTestService(store)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	repoName := "owner/repo"
	startDate := time.Now().Add(-time.Hour)
	intent := &models.Intent{
		Status:         models.PendingBroadCast,
		IsActive:       true,
		ID:             uuid.New(),
		RepositoryName: repoName,
		StartDate:      startDate,
		Until:          time.Now(),
	}

	store.On("SaveIntent", ctx, mock.AnythingOfType("models.Intent")).Return(intent, nil).Once()

	result, err := service.CreateIntent(ctx, repoName, startDate)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, repoName, result.RepositoryName)
}

func TestCreateIntent_InvalidRepoName(t *testing.T) {
	ctx := context.Background()
	store := new(MockStore)
	service := newTestService(store)

	repoName := "invalid-repo"
	startDate := time.Now().Add(-time.Hour)

	result, err := service.CreateIntent(ctx, repoName, startDate)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, manager.ErrInvalidRepository, err)
}

func TestCreateIntent_InvalidStartDate(t *testing.T) {
	ctx := context.Background()
	store := new(MockStore)
	service := newTestService(store)

	repoName := "owner/repo"
	startDate := time.Now().Add(time.Hour)

	result, err := service.CreateIntent(ctx, repoName, startDate)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, manager.ErrInvalidStartDate, err)
}

func TestUpdateIntentStatus(t *testing.T) {
	ctx := context.Background()
	store := new(MockStore)
	service := newTestService(store)

	intentID := uuid.New()
	intent := &models.Intent{
		ID:             intentID,
		RepositoryName: "owner/repo",
		IsActive:       true,
	}
	updatedIntent := &models.Intent{
		ID:             intentID,
		RepositoryName: "owner/repo",
		IsActive:       false,
	}

	store.On("FindIntent", ctx, intentID).Return(intent, nil).Once()
	store.On("UpdateIntent", ctx, mock.AnythingOfType("models.IntentUpdate")).Return(updatedIntent, nil).Once()

	result, err := service.UpdateIntentStatus(ctx, intentID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsActive)
}

func TestUpdateIntentStatus_NotFound(t *testing.T) {
	ctx := context.Background()
	store := new(MockStore)
	service := newTestService(store)

	intentID := uuid.New()

	store.On("FindIntent", ctx, intentID).Return(nil, nil).Once()

	result, err := service.UpdateIntentStatus(ctx, intentID)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, manager.ErrIntentNotFound, err)
}

func TestResetIntentStartDate(t *testing.T) {
	ctx := context.Background()
	store := new(MockStore)
	service := newTestService(store)

	intentID := uuid.New()
	newDate := time.Now().Add(-time.Hour)
	intent := &models.Intent{
		ID:             intentID,
		RepositoryName: "owner/repo",
		StartDate:      newDate,
	}

	store.On("UpdateIntent", ctx, mock.AnythingOfType("models.IntentUpdate")).Return(intent, nil).Once()

	err := service.ResetIntentStartDate(ctx, intentID, newDate)
	assert.NoError(t, err)
}

func TestResetIntentStartDate_InvalidDate(t *testing.T) {
	ctx := context.Background()
	store := new(MockStore)
	service := newTestService(store)

	intentID := uuid.New()
	newDate := time.Now().Add(time.Hour)

	err := service.ResetIntentStartDate(ctx, intentID, newDate)
	assert.Error(t, err)
	assert.Equal(t, manager.ErrInvalidStartDate, err)
}

func TestGetIntent(t *testing.T) {
	ctx := context.Background()
	store := new(MockStore)
	service := newTestService(store)

	intentID := uuid.New()
	intent := &models.Intent{
		ID:             intentID,
		RepositoryName: "owner/repo",
	}

	store.On("FindIntent", ctx, intentID).Return(intent, nil).Once()

	result, err := service.GetIntent(ctx, intentID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, intentID, result.ID)
}

func TestGetIntents(t *testing.T) {
	ctx := context.Background()
	store := new(MockStore)
	service := newTestService(store)

	filter := models.IntentFilter{}
	limit := 10
	offset := 0
	intents := []models.Intent{
		{
			ID:             uuid.New(),
			RepositoryName: "owner/repo",
		},
	}

	paginatedIntents := repository.Paginated[models.Intent]{
		Data:       intents,
		TotalCount: 1,
		Page:       offset,
		PerPage:    limit,
	}

	store.On("FindIntents", ctx, filter, repository.Pagination{Page: offset, PerPage: limit}).Return(paginatedIntents, nil).Once()

	result, err := service.GetIntents(ctx, filter, limit, offset)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result.Data))
	assert.Equal(t, intents[0].ID, result.Data[0].ID)
}

func TestGetTopCommitters(t *testing.T) {
	ctx := context.Background()
	store := new(MockStore)
	service := newTestService(store)

	repoName := "owner/repo"
	page := 1
	perPage := 10
	committers := []models.AuthorStats{
		{
			Author: models.Author{
				Name:     "Test Author",
				Email:    "test@example.com",
				Username: "testuser",
			},
			Commits: 5,
		},
	}

	paginatedCommitters := repository.Paginated[models.AuthorStats]{
		Data:       committers,
		TotalCount: 1,
		Page:       page,
		PerPage:    perPage,
	}

	// Mock the GetRepo call
	store.On("GetRepo", ctx, repoName).Return(&models.Repository{}, nil).Once()

	store.On("GetTopCommitters", ctx, repoName, (*time.Time)(nil), (*time.Time)(nil), repository.Pagination{Page: page, PerPage: perPage}).Return(paginatedCommitters, nil).Once()

	result, err := service.GetTopCommitters(ctx, repoName, page, perPage)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result.Data))
	assert.Equal(t, committers[0].Author.Name, result.Data[0].Author.Name)
}
