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

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/twmb/franz-go/pkg/kgo"

	"fairroll/pkg/config"
	"fairroll/pkg/logger"
	"fairroll/pkg/outbox"
	"fairroll/services/auth/internal/handler"
	"fairroll/services/auth/internal/repository"
	"fairroll/services/auth/internal/service"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env found")
	}

	cfg, err := config.LoadConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	LoggerCfg := logger.Config{
		Loglevel: cfg.Logger.Level,
		Format:   cfg.Logger.Format,
		OutPut:   "stdout",
	}
	err = logger.Init(LoggerCfg)
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info(ctx, "Starting Auth Service",
		"name", cfg.Service.Name,
		"version", cfg.Service.Version,
		"port", cfg.Service.Port,
		"environment", cfg.Environment,
	)

	connStr := cfg.Database.DSN

	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	defer conn.Close(context.Background())

	if err = conn.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping database %v", err)
	}

	logger.Info(ctx, "Successfully connected to database")

	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Kafka.Brokers...))
	if err != nil {
		log.Fatalf("Failed to create kafka client: %v", err)
	}

	defer kafkaClient.Close()

	relay := outbox.NewRelay(conn, kafkaClient, "user.events")

	relayCtx, relayCancel := context.WithCancel(context.Background())
	defer relayCancel()

	go relay.Run(relayCtx)
	logger.Info(ctx, "Outbox relay started", "topic", "user.events")

	userRepo := repository.NewUserRepository(conn)
	sessionRepo := repository.NewSessionRepository(conn)

	authService := service.NewAuthService(userRepo, sessionRepo, cfg)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, `{ "status": "ok", "service": "auth", "timestamp":"%s"`, time.Now().Format(time.RFC3339))
	})

	authHandler := handler.NewAuthHandler(authService)
	authHandler.RegisterRouters(mux)
	// TODO: Connection to DB

	addr := fmt.Sprintf(":%d", cfg.Service.Port)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info(ctx, "HTTP server started",
			"address", addr)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "HTTP Server error", err, "addr", addr)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info(ctx, "Shutdown signal recieved",
		"signal", sig.String(),
	)

	relayCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error(shutdownCtx, "Failed to shoutdown server gracefully", err)
	}

	logger.Info(shutdownCtx, "Auth Service shutdown complete")
}
