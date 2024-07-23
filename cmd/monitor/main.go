package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/go-github/v63/github"
	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
	"github.com/noelukwa/indexer/internal/pkg/config"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

func main() {
	var config config.MonitorConfig
	err := envconfig.Process("monitor_service", &config)
	if err != nil {
		log.Fatal(err)
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
		log.Fatalf("Failed to declare producer queue: %v", err)
	}

	msgs, err := ch.Consume(
		cq.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	// ts := oauth2.StaticTokenSource(
	// 	&oauth2.Token{AccessToken: config.GitHubToken},
	// )
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// tc := oauth2.NewClient(ctx, ts)
	hc := &http.Client{
		Timeout: 10 * time.Second,
	}
	ghClient := github.NewClient(hc)

	resolverChannel := make(chan *github.RepositoryCommit)
	go resolver(ch, config.RabbitMQPublishQueue, resolverChannel)

	var wg sync.WaitGroup

	// Start a goroutine to listen for messages continuously
	go func() {
		for d := range msgs {
			log.Println("messaggggee!")
			wg.Add(1)
			go func(d amqp.Delivery) {
				defer wg.Done()
				handleMessage(ctx, ghClient, redisClient, resolverChannel, d.Body)
			}(d)
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	// Trigger shutdown
	cancel()

	// Close resolver channel and wait for all goroutines to complete
	close(resolverChannel)
	wg.Wait()

	fmt.Println("Shutting down service...")
}

func handleMessage(ctx context.Context, client *github.Client, redisClient *redis.Client, resolverChannel chan<- *github.RepositoryCommit, body []byte) {
	event, err := parseEvent(body)
	if err != nil {
		log.Printf("Failed to parse event: %v", err)
		return
	}

	log.Printf("event: %v", event)

	lockKey := fmt.Sprintf("lock:%s.%s", event.RepoOwner, event.RepoName)
	ok, err := acquireLock(redisClient, lockKey, 10*time.Minute)
	if err != nil || !ok {
		log.Printf("Failed to acquire lock for %s: %v", lockKey, err)
		return
	}
	defer releaseLock(redisClient, lockKey)

	var wg sync.WaitGroup
	wg.Add(1)
	go fetchCommits(ctx, client, resolverChannel, event, &wg)
	wg.Wait()
}

func resolver(ch *amqp.Channel, publishQueue string, resolverChannel <-chan *github.RepositoryCommit) {
	for commit := range resolverChannel {
		log.Println(commit.SHA)
		err := publishCommit(ch, publishQueue, commit)
		if err != nil {
			log.Printf("Error publishing commit: %v", err)
		}
	}
}
