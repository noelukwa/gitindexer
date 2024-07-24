package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/go-github/v63/github"
	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
	"github.com/noelukwa/indexer/internal/events"
	"github.com/noelukwa/indexer/internal/manager/models"
	"github.com/noelukwa/indexer/internal/pkg/config"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
)

const (
	batchSize        = 100
	maxRetries       = 3
	retryDelay       = 5 * time.Second
	lockTTL          = 10 * time.Minute
	publishTimeout   = 5 * time.Second
	githubAPITimeout = 30 * time.Second
)

type CommitResult struct {
	Repository string `json:"repo"`
	commit     *github.RepositoryCommit
}

func main() {
	var config config.MonitorConfig
	err := envconfig.Process("monitor_service", &config)
	if err != nil {
		log.Fatalf("Failed to process config: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: config.RedisAddr,
	})

	conn, err := amqp.Dial(config.RabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	cq, err := ch.QueueDeclare(
		config.RabbitMQConsumeQueue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to declare consumer queue: %v", err)
	}

	_, err = ch.QueueDeclare(
		config.RabbitMQPublishQueue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to declare publish queue: %v", err)
	}

	msgs, err := ch.Consume(
		cq.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.GitHubToken},
	)

	tc := oauth2.NewClient(ctx, ts)

	ghClient := github.NewClient(tc)

	commitsChan := make(chan *CommitResult, batchSize)
	repoChan := make(chan *github.Repository, 1)

	go repoResolver(ctx, ch, config.RabbitMQPublishQueue, repoChan)
	go commitsResolver(ctx, ch, config.RabbitMQPublishQueue, commitsChan)

	var wg sync.WaitGroup

	go func() {
		for d := range msgs {
			wg.Add(1)
			go func(d amqp.Delivery) {
				defer wg.Done()
				handleMessage(ctx, ghClient, redisClient, commitsChan, repoChan, d.Body)
			}(d)
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	// Trigger shutdown
	cancel()

	// Close channels and wait for all goroutines to complete
	close(repoChan)
	close(commitsChan)
	wg.Wait()

	log.Println("Shutting down service...")
}

func handleMessage(ctx context.Context, client *github.Client, redisClient *redis.Client, commitsChan chan<- *CommitResult, repoChan chan<- *github.Repository, body []byte) error {
	event, err := parseEvent(body)
	if err != nil {
		return fmt.Errorf("failed to parse event: %w", err)
	}

	lockKey := fmt.Sprintf("lock:%s.%s", event.Intent.RepoOwner, event.Intent.RepoName)
	ok, err := acquireLock(redisClient, lockKey, lockTTL)
	if err != nil || !ok {
		return fmt.Errorf("failed to acquire lock for %s: %w", lockKey, err)
	}
	defer releaseLock(redisClient, lockKey)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := fetchGithubInfo(ctx, client, repoChan, event.Intent); err != nil {
			log.Printf("Error fetching GitHub info: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := fetchCommits(ctx, client, commitsChan, event.Intent); err != nil {
			log.Printf("Error fetching commits: %v", err)
		}
	}()

	wg.Wait()
	return nil
}

func commitsResolver(ctx context.Context, ch *amqp.Channel, publishQueue string, commitsChan <-chan *CommitResult) {
	batch := make([]*CommitResult, 0, batchSize)

	for {
		select {
		case commit, ok := <-commitsChan:
			if !ok {
				if len(batch) > 0 {
					publishCommitsBatch(ctx, ch, publishQueue, batch)
				}
				return
			}
			batch = append(batch, commit)
			if len(batch) == batchSize {
				publishCommitsBatch(ctx, ch, publishQueue, batch)
				batch = batch[:0]
			}
		case <-time.After(5 * time.Second):
			if len(batch) > 0 {
				publishCommitsBatch(ctx, ch, publishQueue, batch)
				batch = batch[:0]
			}
		case <-ctx.Done():
			if len(batch) > 0 {
				publishCommitsBatch(ctx, ch, publishQueue, batch)
			}
			return
		}
	}
}

func publishCommitsBatch(ctx context.Context, ch *amqp.Channel, publishQueue string, results []*CommitResult) {
	payload := &events.CommitsCommand{
		Kind: events.NewCommitsKind,
		Payload: &events.CommitPayload{
			Commits: make([]*models.Commit, 0, len(results)),
		},
	}

	for _, result := range results {
		commit := result.commit
		payload.Payload.Commits = append(payload.Payload.Commits, &models.Commit{
			Hash:    *commit.SHA,
			Message: *commit.Commit.Message,
			Author: models.Author{
				Name:  *commit.Commit.Author.Name,
				Email: *commit.Commit.Author.Email,
			},
			CreatedAt: commit.Commit.Author.Date.Time,
			Repository: models.Repository{
				FullName: result.Repository,
			},
		})
	}

	err := publishWithRetry(ctx, ch, publishQueue, payload)
	if err != nil {
		log.Printf("Failed to publish commits batch after retries: %v", err)
	}
}

func repoResolver(ctx context.Context, ch *amqp.Channel, publishQueue string, repoChan <-chan *github.Repository) {
	for {
		select {
		case repo, ok := <-repoChan:
			if !ok {
				return
			}
			payload := &events.CommitsCommand{
				Kind: events.NewRepoInfoKind,
				Payload: &events.CommitPayload{
					Repo: &models.Repository{
						ID:        *repo.ID,
						FullName:  *repo.FullName,
						CreatedAt: repo.CreatedAt.Time,
						UpdatedAt: repo.UpdatedAt.Time,
						Stars:     int32(*repo.StargazersCount),
						Watchers:  int32(*repo.WatchersCount),
						Forks:     int32(*repo.ForksCount),
						Language:  *repo.Language,
					},
				},
			}

			err := publishWithRetry(ctx, ch, publishQueue, payload)
			if err != nil {
				log.Printf("Failed to publish repo info after retries: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func parseEvent(data []byte) (*events.IntentCommand, error) {
	var event events.IntentCommand
	err := json.Unmarshal(data, &event)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}
	return &event, nil
}

func fetchGithubInfo(ctx context.Context, client *github.Client, repoChan chan<- *github.Repository, ev *events.IntentPayload) error {
	repo, _, err := client.Repositories.Get(ctx, ev.RepoOwner, ev.RepoName)
	if err != nil {
		return fmt.Errorf("failed to fetch repo info: %w", err)
	}
	select {
	case repoChan <- repo:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func fetchCommits(ctx context.Context, client *github.Client, commitsChan chan<- *CommitResult, ev *events.IntentPayload) error {
	opts := &github.CommitsListOptions{
		Since: ev.From,
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		commits, resp, err := client.Repositories.ListCommits(ctx, ev.RepoOwner, ev.RepoName, opts)
		if err != nil {
			return fmt.Errorf("error fetching commits: %w", err)
		}

		for _, commit := range commits {
			select {
			case commitsChan <- &CommitResult{
				Repository: fmt.Sprintf("%s/%s", ev.RepoOwner, ev.RepoName),
				commit:     commit,
			}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}
	return nil
}

func acquireLock(client *redis.Client, key string, ttl time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ok, err := client.SetNX(ctx, key, "locked", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}
	return ok, nil
}

func releaseLock(client *redis.Client, key string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := client.Del(ctx, key).Result()
	if err != nil {
		log.Printf("Failed to release lock for %s: %v", key, err)
	}
}

func publishWithRetry(ctx context.Context, ch *amqp.Channel, queueName string, ev *events.CommitsCommand) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			err = ch.PublishWithContext(ctx,
				"",
				queueName,
				false,
				false,
				amqp.Publishing{
					ContentType: "application/json",
					Body:        body,
				})
			if err == nil {
				return nil
			}
			log.Printf("Failed to publish (attempt %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(retryDelay)
		}
	}
	return fmt.Errorf("failed to publish after %d attempts", maxRetries)
}
