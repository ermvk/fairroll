package config

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Service     ServiceConfig
	Logger      LoggerConfig
	Environment string
	Database    DatabaseConfig
	JWT         JWTConfig
	Kafka       KafkaConfig
	Redis       RedisConfig
	Services    ServicesConfig
}

type ServicesConfig struct {
	Currency CurrencyConfig
}

type CurrencyConfig struct {
	Addr string
}

type ServiceConfig struct {
	Name    string
	Version string
	Port    int
}

type LoggerConfig struct {
	Level  string
	Format string
}

type DatabaseConfig struct {
	DSN string
}

type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type KafkaConfig struct {
	Brokers []string
	Topic   string
}

type RedisConfig struct {
	Host string
	Port int
	DB   int
}

func LoadConfig(ctx context.Context) (*Config, error) {
	cfg := &Config{
		Service: ServiceConfig{
			Name:    getEnv("SERVICE_NAME", "fairrol-service"),
			Version: getEnv("SERVICE_VERSION", "1.0.0"),
			Port:    getEnvInt("SERVICE_PORT", 8080),
		},
		Logger: LoggerConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Environment: getEnv("ENVIRONMENT", "development"),
		Database: DatabaseConfig{
			DSN: getEnv("DATABASE_DSN", "postgres://localhost:5432/fairroll"),
		},
		JWT: JWTConfig{
			SecretKey:       getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			AccessTokenTTL:  time.Duration(getEnvInt64("JWT_ACCESS_TTL", 900)) * time.Second,
			RefreshTokenTTL: time.Duration(getEnvInt64("JWT_REFRESH_TTL", 604800)) * time.Second,
		},
		Kafka: KafkaConfig{
			Brokers: []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
			Topic:   getEnv("KAFKA_TOPIC", "fairrol-events"),
		},
		Redis: RedisConfig{
			Host: getEnv("REDIS_HOST", "localhost"),
			Port: getEnvInt("REDIS_PORT", 6379),
			DB:   getEnvInt("REDIS_DB", 0),
		},
		Services: ServicesConfig{
			Currency: CurrencyConfig{
				Addr: getEnv("CURRENCY_SERVICE_ADDR", "localhost:8085"),
			},
		},
	}

	if cfg.Environment == "production" && cfg.JWT.SecretKey == "your-secret-key-change-in-production" {
		return nil, errors.New("JWT_SECRET must be set in production")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvInt64(key string, defaultValue int64) int64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return defaultValue
	}
	return value
}
