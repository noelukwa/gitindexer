package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
	"github.com/noelukwa/indexer/internal/pkg/config"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type Event struct {
	Repository string    `json:"repository"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
}

func parseEvent(data []byte) (*Event, error) {
	var event Event
	err := json.Unmarshal(data, &event)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func storeEvent(ctx context.Context, redisClient *redis.Client, event *Event) {
	key := "event:" + event.Repository
	existingEvent, err := redisClient.Get(ctx, key).Result()
	if err == redis.Nil || isNewEventValid(existingEvent, event) {
		eventData, _ := json.Marshal(event)
		redisClient.Set(ctx, key, eventData, 0)
	}
}

func isNewEventValid(existingEventData string, newEvent *Event) bool {
	if existingEventData == "" {
		return true
	}

	var existingEvent Event
	err := json.Unmarshal([]byte(existingEventData), &existingEvent)
	if err != nil {
		return false
	}

	return !isOverlap(existingEvent.StartDate, existingEvent.EndDate, newEvent.StartDate, newEvent.EndDate)
}

func isOverlap(start1, end1, start2, end2 time.Time) bool {
	return start1.Before(end2) && start2.Before(end1)
}

func getAllEvents(ctx context.Context, redisClient *redis.Client) ([]*Event, error) {
	var events []*Event

	keys, err := redisClient.Keys(ctx, "event:*").Result()
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		eventData, err := redisClient.Get(ctx, key).Result()
		if err != nil {
			return nil, err
		}

		var event Event
		err = json.Unmarshal([]byte(eventData), &event)
		if err != nil {
			return nil, err
		}

		events = append(events, &event)
	}

	return events, nil
}

func clearEvents(ctx context.Context, redisClient *redis.Client) {
	keys, _ := redisClient.Keys(ctx, "event:*").Result()
	for _, key := range keys {
		redisClient.Del(ctx, key)
	}
}

func publishEvent(ctx context.Context, ch *amqp.Channel, queueName string, event *Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(ctx,
		"",
		queueName,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	return err
}

func main() {
	var config config.DiscoveryConfig
	err := envconfig.Process("discovery_service", &config)
	if err != nil {
		log.Fatal(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: config.RedisURL,
	})
	defer redisClient.Close()

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
	go func() {
		for d := range msgs {
			processMessage(ctx, redisClient, d.Body)
		}
	}()

	ticker := time.NewTicker(config.BroadcastInterval)
	go func() {
		for range ticker.C {
			broadcastEvents(ctx, ch, redisClient, config.RabbitMQPublishQueue)
		}
	}()

	fmt.Println("Service is running...")

	// graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	fmt.Println("shutting down service...")
}

func processMessage(ctx context.Context, redisClient *redis.Client, body []byte) {
	event, err := parseEvent(body)
	if err != nil {
		log.Printf("Failed to parse event: %v", err)
		return
	}

	storeEvent(ctx, redisClient, event)
}

func broadcastEvents(ctx context.Context, ch *amqp.Channel, redisClient *redis.Client, publishQueue string) {
	events, err := getAllEvents(ctx, redisClient)
	if err != nil {
		log.Printf("Failed to get all events: %v", err)
		return
	}

	for _, event := range events {
		err := publishEvent(ctx, ch, publishQueue, event)
		if err != nil {
			log.Printf("Failed to publish event: %v", err)
		}
	}

	clearEvents(ctx, redisClient)
}
