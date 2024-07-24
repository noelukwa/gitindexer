package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/noelukwa/indexer/internal/events"
	"github.com/noelukwa/indexer/internal/manager/models"
	"github.com/noelukwa/indexer/internal/manager/repository"
	"github.com/noelukwa/indexer/internal/pkg/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ErrInvalidRepository  error = fmt.Errorf("invalid repository name: must be in <owner>/<repo> format")
	ErrInvalidStartDate   error = fmt.Errorf("start date cannot be in the future")
	ErrExistingIntent     error = fmt.Errorf("repository intent already exists")
	ErrIntentNotFound     error = fmt.Errorf("repository intent not found")
	ErrRepositoryNotFound error = fmt.Errorf("repository intent not found")
)

type Service struct {
	store       repository.ManagerStore
	intentsChan chan *events.IntentCommand
	cfg         *config.ManagerConfig
}

func NewService(store repository.ManagerStore, cfg *config.ManagerConfig) *Service {
	return &Service{
		store:       store,
		intentsChan: make(chan *events.IntentCommand, 1),
		cfg:         cfg,
	}
}

func (svc *Service) CreateIntent(ctx context.Context, repoName string, startDate time.Time) (*models.Intent, error) {
	if err := validateRepositoryName(repoName); err != nil {
		return nil, err
	}

	if err := validateStartDate(startDate); err != nil {
		return nil, err
	}

	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	intent := &models.Intent{
		Status:         models.PendingBroadCast,
		IsActive:       true,
		ID:             id,
		RepositoryName: repoName,
		StartDate:      startDate,
		Until:          time.Now(),
	}
	intent, err = svc.store.SaveIntent(ctx, *intent)
	if err != nil {
		return nil, err
	}

	svc.intentsChan <- events.NewIntentCommand(events.NewIntentKind, &events.IntentPayload{
		ID:        intent.ID,
		RepoOwner: strings.Split(repoName, "/")[0],
		RepoName:  strings.Split(repoName, "/")[1],
		From:      intent.StartDate,
	})
	return intent, nil
}

func (svc *Service) UpdateIntentStatus(ctx context.Context, id uuid.UUID) (*models.Intent, error) {
	intent, err := svc.store.FindIntent(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find intent: %w", err)
	}

	log.Printf("found intent: %v", intent)
	if intent == nil {
		return nil, ErrIntentNotFound
	}

	newStatus := !intent.IsActive

	update, err := svc.store.UpdateIntent(ctx, models.IntentUpdate{
		ID:       id,
		IsActive: &newStatus,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update intent: %w", err)
	}

	var eventKind events.IntentKind
	if intent.IsActive && !newStatus {
		eventKind = events.CancelIntentKind
	} else if !intent.IsActive && newStatus {
		eventKind = events.NewIntentKind
	} else {
		eventKind = events.UpdateIntentKind
	}

	svc.intentsChan <- events.NewIntentCommand(eventKind, &events.IntentPayload{
		ID:        update.ID,
		RepoOwner: strings.Split(update.RepositoryName, "/")[0],
		RepoName:  strings.Split(update.RepositoryName, "/")[1],
		From:      update.StartDate,
	})

	return update, nil
}

func (svc *Service) ResetIntentStartDate(ctx context.Context, id uuid.UUID, newDate time.Time) error {
	if err := validateStartDate(newDate); err != nil {
		return err
	}

	intent, err := svc.store.UpdateIntent(ctx, models.IntentUpdate{
		ID:        id,
		StartDate: &newDate,
	})
	if err != nil {
		return err
	}

	svc.intentsChan <- events.NewIntentCommand(events.UpdateIntentKind, &events.IntentPayload{
		ID:        intent.ID,
		RepoOwner: strings.Split(intent.RepositoryName, "/")[0],
		RepoName:  strings.Split(intent.RepositoryName, "/")[1],
		From:      intent.StartDate,
	})

	return nil
}

func (svc *Service) GetIntent(ctx context.Context, id uuid.UUID) (*models.Intent, error) {
	return svc.store.FindIntent(ctx, id)
}

func (svc *Service) GetIntents(ctx context.Context, filter models.IntentFilter, limit, offset int) (repository.Paginated[models.Intent], error) {

	pagination := repository.Pagination{
		Page:    offset,
		PerPage: limit,
	}

	return svc.store.FindIntents(ctx, filter, pagination)
}

func (svc *Service) GetTopCommitters(ctx context.Context, repoName string, page, perPage int) (repository.Paginated[models.AuthorStats], error) {
	pagination := repository.Pagination{
		Page:    page,
		PerPage: perPage,
	}

	topCommitters, err := svc.store.GetTopCommitters(ctx, repoName, nil, nil, pagination)
	if err != nil {
		return repository.Paginated[models.AuthorStats]{}, fmt.Errorf("failed to get top committers: %w", err)
	}

	if topCommitters.Data == nil {
		topCommitters.Data = []models.AuthorStats{}
	}

	return topCommitters, nil
}
func (svc *Service) BatchSaveCommits(ctx context.Context, commits []*models.Commit) error {
	if len(commits) == 0 {
		return nil
	}

	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Repository.FullName < commits[j].Repository.FullName
	})

	currentRepoName := commits[0].Repository.FullName

	var currentRepoCommits []*models.Commit

	for i, commit := range commits {
		if commit.Repository.FullName != currentRepoName || i == len(commits)-1 {
			if i == len(commits)-1 {
				currentRepoCommits = append(currentRepoCommits, commit)
			}
			repo, err := svc.store.GetRepo(ctx, commit.Repository.FullName)
			if err != nil || repo == nil {
				return fmt.Errorf("failed to find repository %s: %w", currentRepoName, err)
			}

			err = svc.store.SaveManyCommit(ctx, repo.ID, currentRepoCommits)
			if err != nil {
				return fmt.Errorf("failed to save commits for repository %s: %w", currentRepoName, err)
			}
			currentRepoName = commit.Repository.FullName
			currentRepoCommits = []*models.Commit{commit}
		} else {
			currentRepoCommits = append(currentRepoCommits, commit)
		}
	}

	return nil
}

func (svc *Service) FindRepository(ctx context.Context, repoName string) (*models.Repository, error) {
	return svc.store.GetRepo(ctx, repoName)
}

func (svc *Service) GetCommits(ctx context.Context, repo string, startDate, endDate time.Time, page, perPage int) (models.CommitPage, error) {

	_, err := svc.store.GetRepo(ctx, repo)
	if err != nil {
		return models.CommitPage{}, err
	}

	filter := models.CommitsFilter{
		RepositoryName: repo,
		StartDate:      &startDate,
		EndDate:        &endDate,
	}
	pagination := repository.Pagination{
		Page:    page,
		PerPage: perPage,
	}

	repoResp, err := svc.store.FindCommits(ctx, filter, pagination)
	if err != nil {
		return models.CommitPage{}, err
	}

	return models.CommitPage{
		Commits:    repoResp.Data,
		TotalCount: repoResp.TotalCount,
		Page:       int32(repoResp.Page),
		PerPage:    int32(repoResp.PerPage),
	}, nil
}

func (svc *Service) ProcessCommitCommands(ctx context.Context, body []byte) error {
	var command events.CommitsCommand
	err := json.Unmarshal(body, &command)
	if err != nil {
		return fmt.Errorf("failed to unmarshal commit command: %w", err)
	}

	switch command.Kind {
	case events.NewRepoInfoKind:
		if command.Payload.Repo == nil {
			return fmt.Errorf("repo info is missing in the payload")
		}
		err = svc.store.SaveRepo(ctx, command.Payload.Repo)
		if err != nil {
			return fmt.Errorf("failed to save repo: %w", err)
		}

	case events.NewCommitsKind:
		if len(command.Payload.Commits) == 0 {
			return fmt.Errorf("commits are missing in the payload")
		}
		log.Printf("new commits payload: %+v\n", command.Payload.Commits)
		err = svc.BatchSaveCommits(ctx, command.Payload.Commits)
		if err != nil {
			return fmt.Errorf("failed to save commits: %w", err)
		}

	default:
		return fmt.Errorf("unknown commit command kind: %s", command.Kind)
	}

	return nil
}

func (svc *Service) StartBroadCast(ctx context.Context, ch *amqp.Channel) error {
	for {
		select {
		case v, ok := <-svc.intentsChan:
			if !ok {
				return nil
			}

			body, err := json.Marshal(v)
			if err != nil {
				log.Printf("failed to marshal intent: %v", err)
				continue
			}

			err = ch.PublishWithContext(ctx,
				"",
				svc.cfg.IntentsQueueName,
				false,
				false,
				amqp.Publishing{
					ContentType: "application/json",
					Body:        body,
				})
			if err != nil {
				log.Printf("failed to publish message: %v", err)
				continue
			}

			newStatus := models.SuccessBroadCast
			_, err = svc.store.UpdateIntent(ctx, models.IntentUpdate{
				ID:     v.Intent.ID,
				Status: &newStatus,
			})
			if err != nil {
				return err
			}
		case <-ctx.Done():
			log.Println("context cancelled, stopping broadcast")
			return ctx.Err()
		}
	}
}

func validateRepositoryName(name string) error {
	parts := strings.Split(name, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ErrInvalidRepository
	}
	return nil
}

func validateStartDate(date time.Time) error {
	if date.After(time.Now()) {
		return ErrInvalidStartDate
	}
	return nil
}
