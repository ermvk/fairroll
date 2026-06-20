package main

import (
	"context"
	"fairroll/pkg/logger"
	"time"
)

func main() {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)

	defer cancel()

	LoggerCfg := logger.Config{
		Loglevel: cfg.L
	}
}
