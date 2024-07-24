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
	"github.com/noelukwa/indexer/internal/events"
	"github.com/noelukwa/indexer/internal/pkg/config"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

func parseEvent(data []byte) (*events.IntentCommand, error) {
	var event events.IntentCommand
	err := json.Unmarshal(data, &event)
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func processIntent(ctx context.Context, redisClient *redis.Client, event *events.IntentCommand) error {
	key := fmt.Sprintf("intent:%s:%s", event.Intent.RepoOwner, event.Intent.RepoName)

	switch event.Kind {
	case events.NewIntentKind:
		return storeNewIntent(ctx, redisClient, key, event.Intent)
	case events.UpdateIntentKind:
		return updateIntent(ctx, redisClient, key, event.Intent)
	case events.CancelIntentKind:
		return cancelIntent(ctx, redisClient, key)
	default:
		return fmt.Errorf("unknown intent kind: %s", event.Kind)
	}
}

func storeNewIntent(ctx context.Context, redisClient *redis.Client, key string, intent *events.IntentPayload) error {
	intentData, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	return redisClient.Set(ctx, key, intentData, 0).Err()
}

func updateIntent(ctx context.Context, redisClient *redis.Client, key string, updatedIntent *events.IntentPayload) error {
	existingIntentData, err := redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return storeNewIntent(ctx, redisClient, key, updatedIntent)
	} else if err != nil {
		return err
	}

	var existingIntent events.IntentPayload
	if err := json.Unmarshal([]byte(existingIntentData), &existingIntent); err != nil {
		return err
	}

	existingIntent.From = updatedIntent.From
	existingIntent.Until = updatedIntent.Until

	return storeNewIntent(ctx, redisClient, key, &existingIntent)
}

func cancelIntent(ctx context.Context, redisClient *redis.Client, key string) error {
	return redisClient.Del(ctx, key).Err()
}

func getAllIntents(ctx context.Context, redisClient *redis.Client) ([]*events.IntentPayload, error) {
	var intents []*events.IntentPayload

	keys, err := redisClient.Keys(ctx, "intent:*").Result()
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		intentData, err := redisClient.Get(ctx, key).Result()
		if err != nil {
			return nil, err
		}

		intent := &events.IntentPayload{}
		if err := json.Unmarshal([]byte(intentData), intent); err != nil {
			return nil, err
		}

		intents = append(intents, intent)
	}

	return intents, nil
}

func publishEvent(ctx context.Context, ch *amqp.Channel, queueName string, event *events.IntentCommand) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return ch.PublishWithContext(ctx,
		"",
		queueName,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
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
			broadcastIntents(ctx, ch, redisClient, config.RabbitMQPublishQueue)
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
	log.Println("received message.")
	event, err := parseEvent(body)
	if err != nil {
		log.Printf("Failed to parse event: %v", err)
		return
	}

	log.Printf("received event: %v", event)

	if err := processIntent(ctx, redisClient, event); err != nil {
		log.Printf("Failed to process intent: %v", err)
	}
}

func broadcastIntents(ctx context.Context, ch *amqp.Channel, redisClient *redis.Client, publishQueue string) {
	intents, err := getAllIntents(ctx, redisClient)
	if err != nil {
		log.Printf("Failed to get all intents: %v", err)
		return
	}

	for _, intent := range intents {
		event := &events.IntentCommand{
			Kind:   events.NewIntentKind,
			Intent: intent,
		}

		if err := publishEvent(ctx, ch, publishQueue, event); err != nil {
			log.Printf("Failed to publish intent: %v", err)
		}
	}
}
