.PHONY: help build up down logs clean test lint fmt health status

# Variables
DOCKER_COMPOSE := docker-compose
GO := go
GOFLAGS := -v
SERVICES := auth wallet transfer notification currency

help:
	@echo "Fairroll Payment Platform - Available Commands"
	@echo ""
	@echo "Service Management:"
	@echo "  make up              Start all services (docker-compose up)"
	@echo "  make down            Stop all services (docker-compose down)"
	@echo "  make ps              Show running containers"
	@echo "  make logs            Tail logs from all services"
	@echo "  make logs-SERVICE    Tail logs from specific service (e.g., make logs-auth)"
	@echo ""
	@echo "Development:"
	@echo "  make build           Build all services locally"
	@echo "  make build-SERVICE   Build specific service (e.g., make build-auth)"
	@echo "  make fmt             Format all Go code"
	@echo "  make lint            Run linter on all services"
	@echo "  make test            Run tests (when implemented)"
	@echo ""
	@echo "Database:"
	@echo "  make db-clean        Remove all database volumes"
	@echo "  make db-reset        Tear down and rebuild databases"
	@echo ""
	@echo "Monitoring:"
	@echo "  make health          Check health of all services"
	@echo "  make status          Show service status"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean           Remove build artifacts and volumes"
	@echo ""

# Service Management
up:
	@echo "Starting Fairroll platform..."
	$(DOCKER_COMPOSE) up -d
	@echo "Waiting for services to be ready..."
	@sleep 5
	@make health

down:
	@echo "Stopping Fairroll platform..."
	$(DOCKER_COMPOSE) down

ps:
	$(DOCKER_COMPOSE) ps

logs:
	$(DOCKER_COMPOSE) logs -f

logs-auth:
	$(DOCKER_COMPOSE) logs -f auth

logs-wallet:
	$(DOCKER_COMPOSE) logs -f wallet

logs-transfer:
	$(DOCKER_COMPOSE) logs -f transfer

logs-notification:
	$(DOCKER_COMPOSE) logs -f notification

logs-currency:
	$(DOCKER_COMPOSE) logs -f currency

logs-kafka:
	$(DOCKER_COMPOSE) logs -f kafka

logs-db:
	$(DOCKER_COMPOSE) logs -f postgres-wallet postgres-auth

# Build targets
build: build-auth build-wallet build-transfer build-notification build-currency
	@echo "All services built successfully"

build-auth:
	@echo "Building auth service..."
	cd services/auth && $(GO) build $(GOFLAGS) -o ../../bin/auth ./cmd/main.go

build-wallet:
	@echo "Building wallet service..."
	cd services/wallet && $(GO) build $(GOFLAGS) -o ../../bin/wallet ./cmd/main.go

build-transfer:
	@echo "Building transfer service..."
	cd services/transfer && $(GO) build $(GOFLAGS) -o ../../bin/transfer ./cmd/main.go

build-notification:
	@echo "Building notification service..."
	cd services/notification && $(GO) build $(GOFLAGS) -o ../../bin/notification ./cmd/main.go

build-currency:
	@echo "Building currency service..."
	cd services/currency && $(GO) build $(GOFLAGS) -o ../../bin/currency ./cmd/main.go

# Code quality
fmt:
	@echo "Formatting Go code..."
	$(GO) fmt ./...

lint:
	@echo "Running linter..."
	golangci-lint run ./...

test:
	@echo "Running tests..."
	$(GO) test -v ./...

# Database management
db-clean:
	@echo "Removing database volumes..."
	$(DOCKER_COMPOSE) down -v

db-reset: down
	@echo "Resetting databases..."
	$(DOCKER_COMPOSE) down -v
	$(DOCKER_COMPOSE) up -d postgres-auth postgres-wallet kafka
	@echo "Waiting for databases to be ready..."
	@sleep 10

# Monitoring & Health Checks
health:
	@echo "Checking service health..."
	@echo ""
	@echo "Auth Service (8080):"
	@curl -s http://localhost:8080/health | jq '.' || echo "  ❌ Not responding"
	@echo ""
	@echo "Wallet Service (8081):"
	@curl -s http://localhost:8081/health | jq '.' || echo "  ❌ Not responding"
	@echo ""
	@echo "Transfer Service (8082):"
	@curl -s http://localhost:8082/health | jq '.' || echo "  ❌ Not responding"
	@echo ""
	@echo "Notification Service (8084):"
	@curl -s http://localhost:8084/health | jq '.' || echo "  ❌ Not responding"
	@echo ""
	@echo "Currency Service (8085):"
	@curl -s http://localhost:8085/health | jq '.' || echo "  ❌ Not responding"

status:
	@echo "Fairroll Platform Status"
	@echo "========================"
	@$(DOCKER_COMPOSE) ps --format "table {{.Service}}\t{{.Status}}\t{{.Ports}}"

# Cleanup
clean: down
	@echo "Cleaning up build artifacts..."
	@rm -rf bin/
	@$(GO) clean ./...
	@echo "Cleanup complete"

# Useful development commands
generate:
	@echo "Generating code from OpenAPI specs (not yet implemented)..."

migrate-up:
	@echo "Running database migrations..."
	@echo "Note: Manual migration required. See services/*/migrations/"

# Quick integration test
test-auth:
	@echo "Testing Auth Service..."
	@curl -X POST http://localhost:8080/auth/register \
		-H "Content-Type: application/json" \
		-d '{"email":"test@example.com","username":"testuser","password":"password123"}' | jq '.'

test-transfer:
	@echo "Testing Transfer Service..."
	@curl -X GET http://localhost:8082/health | jq '.'

test-notification:
	@echo "Testing Notification Service..."
	@curl -X GET http://localhost:8084/health | jq '.'

test-currency:
	@echo "Testing Currency Service..."
	@curl -X GET http://localhost:8085/health | jq '.'

# Docker image management
docker-build:
	@echo "Building Docker images..."
	$(DOCKER_COMPOSE) build

docker-push:
	@echo "Pushing Docker images to registry..."
	@echo "Note: Configure registry in docker-compose.yml first"

# Development environment
dev-setup:
	@echo "Setting up development environment..."
	@$(GO) mod download
	@$(GO) mod tidy
	@echo "Development environment ready"

version:
	@echo "Fairroll Version Info:"
	@echo "Go Version: $$($(GO) version)"
	@echo "Docker Compose Version: $$($(DOCKER_COMPOSE) version --short)"
	@echo "Platform: $$(uname -s)"

# Database inspection (when containers are running)
db-inspect-auth:
	@echo "Connecting to Auth DB (fairroll_auth)..."
	@$(DOCKER_COMPOSE) exec postgres-auth psql -U fairroll -d fairroll_auth

db-inspect-wallet:
	@echo "Connecting to Wallet DB (fairroll_wallet)..."
	@$(DOCKER_COMPOSE) exec postgres-wallet psql -U fairroll -d fairroll_wallet

kafka-topics:
	@echo "Listing Kafka topics..."
	@$(DOCKER_COMPOSE) exec kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server kafka:9092 --list

kafka-consume-user-events:
	@echo "Consuming from user.events topic..."
	@$(DOCKER_COMPOSE) exec kafka /opt/kafka/bin/kafka-console-consumer.sh \
		--bootstrap-server kafka:9092 \
		--topic user.events \
		--from-beginning

.DEFAULT_GOAL := help
