package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
	"github.com/labstack/echo/v4"
	"github.com/noelukwa/indexer/internal/manager"
	"github.com/noelukwa/indexer/internal/manager/api"
	"github.com/noelukwa/indexer/internal/manager/repository/postgres"
	"github.com/noelukwa/indexer/internal/pkg/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	var cfg config.ManagerConfig
	err := envconfig.Process("manager_service", &cfg)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
		cfg.IntentsQueueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to declare consumer queue: %v", err)
	}

	cq, err := ch.QueueDeclare(
		cfg.CommitsQueueName,
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

	dataStore, err := postgres.NewManagerStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to establish DB connection: %v", err)
	}

	service := manager.NewService(dataStore)

	e := echo.New()
	handler := api.SetupRoutes(service, e)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.ServerPort),
		Handler: handler,
	}

	go func() {
		log.Printf("server listening on %d\n", cfg.ServerPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
	}()

	go func() {
		for d := range msgs {
			if err := service.ProcessCommits(d.Body); err != nil {
				log.Printf("Error processing commit: %v", err)
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	if err := ch.Close(); err != nil {
		log.Printf("Error closing RabbitMQ channel: %v", err)
	}
	if err := conn.Close(); err != nil {
		log.Printf("Error closing RabbitMQ connection: %v", err)
	}

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := httpServer.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("HTTP server Shutdown: %v", err)
	}

	log.Println("Server exiting")
}
