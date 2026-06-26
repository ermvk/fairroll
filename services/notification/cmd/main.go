// NOTIFICATION SERVICE
// Listens to events and sends notifications (emails, pushes, etc) like mock for now brudda

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/twmb/franz-go/pkg/kgo"

	"fairroll/pkg/config"
	"fairroll/pkg/logger"
	"fairroll/services/notification/internal/consumer"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env.example found")
	}

	cfg, err := config.LoadConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	loggerCfg := logger.Config{
		Loglevel: cfg.Logger.Level,
		Format:   cfg.Logger.Format,
		OutPut:   "stdout",
	}

	if err := logger.Init(loggerCfg); err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}

	defer logger.Sync()

	logger.Info(ctx, "Starting Notification Service",
		"name", cfg.Service.Name,
		"version", cfg.Service.Version,
		"port", cfg.Service.Port,
		"environment", cfg.Environment,
	)

	connStr := cfg.Database.DSN
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		log.Printf("Note: Could not connect to database, notifications will be logged only: %v", err)
	} else {
		defer conn.Close(context.Background())
		if err = conn.Ping(context.Background()); err != nil {
			log.Printf("Note: Database ping failed, notifications will be logged only: %v", err)
		}
	}

	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Kafka.Brokers...),
		kgo.ConsumeTopics(
			"user.events",
			"wallet.events",
			"transfer.events",
			"deposit.completed",
			"withdrawal.completed",
		),
		kgo.ConsumerGroup("notification-service"),
	)
	if err != nil {
		log.Fatalf("Failed to create kafka client: %v", err)
	}
	defer kafkaClient.Close()

	eventConsumer := consumer.NewEventConsumer(kafkaClient)

	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()

	go eventConsumer.Run(consumerCtx)
	logger.Info(ctx, "Event consumer started", "topics", "user.events, transfer.events, etc")

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"status": "ok", "service": "notification", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	})

	addr := fmt.Sprintf(":%d", cfg.Service.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info(ctx, "HTTP server started", "address", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(ctx, "HTTP Server error", err, "address", addr)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info(ctx, "Shutdown signal received", "signal", sig.String())

	consumerCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error(shutdownCtx, "Failed to shutdown server gracefully", err)
	}

	logger.Info(shutdownCtx, "Notification Service shutdown complete")
}
