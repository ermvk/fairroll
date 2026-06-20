package main

import (
	"context"
	"fairroll/pkg/config"
	"fairroll/pkg/logger"
	"log"
	"time"
)

func main() {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

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
}
