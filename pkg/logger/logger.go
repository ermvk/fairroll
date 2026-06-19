package logger

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *zap.Logger

type Logger struct {
}

type Config struct {
	Loglevel string
	Format   string
	OutPut   string
}

func Init(cfg Config) error {
	var zapConfig zap.Config

	level := zapcore.InfoLevel
	switch cfg.Loglevel {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	if cfg.Format == "json" {
		zapConfig = zap.NewProductionConfig()

	} else {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}

	zapConfig.Level = zap.NewAtomicLevelAt(level)

	var err error
	globalLogger, err = zapConfig.Build()

	if err != nil {
		return fmt.Errorf("failed to build logger: %w", err)
	}

	return nil
}

func Sync() error {
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}
func GetZap() *zap.Logger {
	if globalLogger == nil {
		globalLogger = zap.NewNop()
	}
	return globalLogger
}

func fieldsFromKVPairs(kvPairs ...interface{}) []zap.Field {
	fields := make([]zap.Field, 0)
	for i := 0; i < len(kvPairs); i += 2 {
		if i+1 >= len(kvPairs) {
			break
		}
		key, ok := kvPairs[i].(string)
		if !ok {
			continue
		}
		value := kvPairs[i+1]
		fields = append(fields, zap.Any(key, value))
	}
	return fields
}

func Info(ctx context.Context, msg string, kvPairs ...interface{}) {
	GetZap().Info(msg, fieldsFromKVPairs(kvPairs...)...)
}

func Error(ctx context.Context, msg string, err error, kvPairs ...interface{}) {
	fields := fieldsFromKVPairs(kvPairs...)
	fields = append(fields, zap.Error(err))
	GetZap().Error(msg, fields...)
}

func Warn(ctx context.Context, msg string, err error, kvPairs ...interface{}) {
	fields := fieldsFromKVPairs(kvPairs...)
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	GetZap().Warn(msg, fields...)
}

func Debug(ctx context.Context, msg string, kvPairs ...interface{}) {
	GetZap().Debug(msg, fieldsFromKVPairs(kvPairs...)...)
}
