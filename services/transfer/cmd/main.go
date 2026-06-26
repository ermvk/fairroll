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
	"fairroll/pkg/outbox"
	"fairroll/services/transfer/internal/repository"
	"fairroll/services/transfer/internal/service"
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

	logger.Info(ctx,
		"Starting Transfer Service",
		"name", cfg.Service.Name,
		"version", cfg.Service.Version,
		"port", cfg.Service.Port,
		"environment", cfg.Environment)

	connStr := cfg.Database.DSN
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	if err = conn.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	kafkaClient, err := kgo.NewClient(kgo.SeedBrokers(cfg.Kafka.Brokers...))
	if err != nil {
		log.Fatalf("Failed to create kafka client: %v", err)
	}
	defer kafkaClient.Close()

	relay := outbox.NewRelay(conn, kafkaClient, "transfer.events")
	relayCtx, relayCancel := context.WithCancel(context.Background())
	defer relayCancel()
	go relay.Run(relayCtx)
	logger.Info(ctx, "Transfer outbox relay started")

	transferRepo := repository.NewTransferRepository(conn)
	eventPublisher := service.NewEventPublisher(conn)
	currencyServiceAddr := cfg.Services.Currency.Addr

	transferService, err := service.NewTransferService(transferRepo, eventPublisher, currencyServiceAddr)
	if err != nil {
		log.Fatalf("Failed to create transfer service: %v", err)
	}
	defer transferService.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","service":"transfer","timestamp":"%s"}`,
			time.Now().Format(time.RFC3339))
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
			logger.Error(ctx, "HTTP server error", err, "address", addr)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info(ctx, "Shutdown signal received", "signal", sig.String())

	relayCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error(shutdownCtx, "Failed to shutdown server gracefully", err)
	}
}
