// WALLET SERVICE

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

	"fairroll/pkg/config"
	"fairroll/pkg/logger"
	"fairroll/services/wallet/internal/handler"
	"fairroll/services/wallet/internal/model"
	"fairroll/services/wallet/internal/repository"
	"fairroll/services/wallet/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env found")
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

	logger.Info(ctx, "Starting Wallet Service",
		"name", cfg.Service.Name,
		"version", cfg.Service.Name,
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
		log.Fatalf("Failed to ping wallet database")
	}

	walletRepo := repository.NewWalletRepository(conn)
	walletService := service.NewWalletService(walletRepo)
	walletHandler := handler.NewWalletHandler(walletService)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"status": "ok", "service": "wallet", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	})

	model.HandlerFromMux(walletHandler, mux)
	addr := fmt.Sprintf(":%d", cfg.Service.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info(ctx, "HTTP server started", "adress", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(ctx, "HTTP Server error", err, "address", addr)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	logger.Info(ctx, "Shutdown signal recieved", "signal", sig.String())

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error(shutdownCtx, "Failed to shutdown server gracefully", err)
	}

	logger.Info(ctx, "Wallet Service shutdown complete")
}
