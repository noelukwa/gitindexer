package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	ErrInvalidRepository error = fmt.Errorf("invalid repository name: must be in <owner>/<repo> format")
	ErrInvalidStartDate  error = fmt.Errorf("start date cannot be in the future")
	ErrExistingIntent    error = fmt.Errorf("repository intent already exists")
)

type Service struct {
	intentStore repository.ManagerStore
	intentsChan chan *models.Intent
	cfg         *config.ManagerConfig
}

func NewService(intentStore repository.ManagerStore, cfg *config.ManagerConfig) *Service {
	return &Service{
		intentStore: intentStore,
		intentsChan: make(chan *models.Intent),
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
	intent, err = svc.intentStore.SaveIntent(ctx, *intent)
	if err != nil {
		return nil, err
	}

	svc.intentsChan <- intent
	return intent, nil
}

func (svc *Service) UpdateIntentStatus(ctx context.Context, id uuid.UUID, status bool) error {
	// intent, err := svc.intentStore.FindIntent(ctx, id)
	// if err != nil {
	// 	return err
	// }

	// if update.StartDate != nil {
	// 	if err := validateStartDate(*update.StartDate); err != nil {
	// 		return err
	// 	}
	// 	intent.StartDate = *update.StartDate
	// }

	// if update.IsActive != nil {
	// 	intent.IsActive = *update.IsActive
	// }

	// if update.Status != nil {
	// 	intent.Status = *update.Status
	// }

	// return svc.intentStore.UpdateIntent(ctx, models.IntentUpdate{
	// 	Status:    &intent.Status,
	// 	IsActive:  &intent.IsActive,
	// 	StartDate: &intent.StartDate,
	// })

	return nil
}

func (svc *Service) ResetIntentStartDate(ctx context.Context, id uuid.UUID, newDate time.Time) error {
	// intent, err := svc.intentStore.FindIntent(ctx, id)
	// if err != nil {
	// 	return err
	// }
	// if err := validateStartDate(newDate); err != nil {
	// 	return err
	// }

	return nil
}

func (svc *Service) GetIntent(ctx context.Context, id uuid.UUID) (*models.Intent, error) {
	return svc.intentStore.FindIntent(ctx, id)
}

func (svc *Service) GetIntents(ctx context.Context, filter models.IntentFilter, limit, offset int) (repository.Paginated[models.Intent], error) {

	pagination := repository.Pagination{
		Page:    offset,
		PerPage: limit,
	}

	return svc.intentStore.FindIntents(ctx, filter, pagination)
}

func (svc *Service) BatchSaveCommits(ctx context.Context, commits []models.Commit) error {

	return nil
}

func (svc *Service) FindRepository(ctx context.Context, repoName string) (*models.Repository, error) {
	return nil, nil
}

func (svc *Service) GetTopCommitters(ctx context.Context, repoName string, limit int) ([]models.AuthorStats, error) {

	return nil, nil
}

func (svc *Service) ProcessCommits(ctx context.Context, body []byte) error {
	return nil
}

func (svc *Service) StartBroadCast(ctx context.Context, ch *amqp.Channel) error {
	for {
		select {
		case v, ok := <-svc.intentsChan:
			if !ok {
				return nil
			}

			id, err := uuid.NewRandom()
			if err != nil {
				return err
			}
			parts := strings.Split(v.RepositoryName, "/")

			body, err := json.Marshal(events.NewIntent{
				RepoOwner: parts[0],
				RepoName:  parts[1],
				Until:     v.Until,
				From:      v.StartDate,
				ID:        id,
			})
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
			_, err = svc.intentStore.UpdateIntent(ctx, models.IntentUpdate{
				ID:     v.ID,
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

func (svc *Service) GetCommits(ctx context.Context, repo string, startDate, endDate time.Time, page, perPage int) (models.CommitPage, error) {

	return models.CommitPage{}, nil
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
