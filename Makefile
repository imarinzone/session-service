.PHONY: help build run dev install-air test clean docker-build docker-up docker-down migrate generate-keys lint fmt vet deps

# Variables
BINARY_NAME=server
MAIN_PATH=./cmd/server
DOCKER_IMAGE=session-service
DOCKER_TAG=latest

# Colors for output
GREEN=\033[0;32m
YELLOW=\033[0;33m
NC=\033[0m # No Color

help: ## Display this help message
	@echo "$(GREEN)Available targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'

deps: ## Download and tidy dependencies
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	go mod download
	go mod tidy

build: ## Build the application
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	go build -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "$(GREEN)Build complete!$(NC)"

run: ## Run the application locally
	@echo "$(GREEN)Running $(BINARY_NAME)...$(NC)"
	go run $(MAIN_PATH)

dev: ## Run with live reloading (requires air)
	@echo "$(GREEN)Starting development server with live reloading...$(NC)"
	@if ! command -v air > /dev/null; then \
		echo "$(YELLOW)Air not found. Installing...$(NC)"; \
		go install github.com/air-verse/air@latest; \
	fi
	@if command -v air > /dev/null; then \
		air; \
	else \
		$$(go env GOPATH)/bin/air || $(HOME)/go/bin/air; \
	fi

install-air: ## Install air for live reloading
	@echo "$(GREEN)Installing air...$(NC)"
	go install github.com/air-verse/air@latest
	@echo "$(GREEN)Air installed!$(NC)"


test: ## Run tests
	@echo "$(GREEN)Running tests...$(NC)"
	go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning...$(NC)"
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	@echo "$(GREEN)Clean complete!$(NC)"

fmt: ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	go fmt ./...

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	go vet ./...

lint: fmt vet ## Run linters (fmt + vet)

docker-build: ## Build Docker image
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "$(GREEN)Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"

docker-up: ## Start services with Docker Compose
	@echo "$(GREEN)Starting services with Docker Compose...$(NC)"
	docker-compose up -d
	@echo "$(GREEN)Services started!$(NC)"

docker-down: ## Stop services with Docker Compose
	@echo "$(GREEN)Stopping services...$(NC)"
	docker-compose down
	@echo "$(GREEN)Services stopped!$(NC)"

docker-logs: ## View Docker Compose logs
	docker-compose logs -f

docker-clean: docker-down ## Stop services and remove volumes
	@echo "$(GREEN)Cleaning Docker resources...$(NC)"
	docker-compose down -v
	@echo "$(GREEN)Docker resources cleaned!$(NC)"

migrate: ## Run database migrations
	@echo "$(GREEN)Running database migrations...$(NC)"
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "$(YELLOW)Warning: DATABASE_URL not set. Using default from docker-compose...$(NC)"; \
		psql postgres://user:password@localhost:5435/sessiondb -f migrations/001_init.sql; \
	else \
		psql $(DATABASE_URL) -f migrations/001_init.sql; \
	fi
	@echo "$(GREEN)Migrations complete!$(NC)"

generate-keys: ## Generate RSA key pair for JWT and update .env
	@echo "$(GREEN)Generating RSA key pair...$(NC)"
	@if [ -f scripts/setup-keys.sh ]; then \
		./scripts/setup-keys.sh; \
	else \
		openssl genrsa -out private.pem 2048; \
		openssl rsa -in private.pem -pubout -out public.pem; \
		echo "$(GREEN)Keys generated: private.pem, public.pem$(NC)"; \
		echo "$(YELLOW)Please manually update .env file with the key contents$(NC)"; \
		echo "$(YELLOW)Copy the contents of private.pem to JWT_PRIVATE_KEY$(NC)"; \
		echo "$(YELLOW)Copy the contents of public.pem to JWT_PUBLIC_KEY$(NC)"; \
	fi

dev-setup: generate-keys docker-up migrate ## Complete development setup
	@echo "$(GREEN)Development environment ready!$(NC)"
	@echo "$(YELLOW)Don't forget to:$(NC)"
	@echo "  1. Verify .env contains JWT keys and key rotation settings (KEY_ROTATION_DAYS, KEY_GRACE_DAYS)"
	@echo "  2. Create a client (and any tenants/users) in the database using 'make create-client', 'make create-tenant', 'make create-user' or SQL"
	@echo "  3. Run 'make run' to start the service locally"

create-client: ## Create a test client (requires DATABASE_URL or docker-compose)
	@echo "$(GREEN)Creating test client...$(NC)"
	@read -p "Client ID: " CLIENT_ID; \
	read -sp "Client Secret: " CLIENT_SECRET; \
	echo ""; \
	HASH=$$(go run -c 'golang.org/x/crypto/bcrypt' <<< "$$CLIENT_SECRET" 2>/dev/null || echo "$$(htpasswd -nbBC 10 "" $$CLIENT_SECRET | cut -d: -f2)"); \
	if [ -z "$(DATABASE_URL)" ]; then \
		DB_URL="postgres://user:password@localhost:5435/sessiondb?sslmode=disable"; \
	else \
		DB_URL="$(DATABASE_URL)"; \
	fi; \
	psql $$DB_URL -c "INSERT INTO clients (client_id, client_secret_hash, rate_limit) VALUES ('$$CLIENT_ID', '$$HASH', 100) ON CONFLICT (client_id) DO UPDATE SET client_secret_hash = EXCLUDED.client_secret_hash, rate_limit = EXCLUDED.rate_limit;"; \
	echo "$(GREEN)Client created/updated: $$CLIENT_ID$(NC)"

create-tenant: ## Create or update a tenant record
	@echo "$(GREEN)Creating/updating tenant...$(NC)"
	@read -p "Tenant ID (internal, required): " TENANT_ID; \
	read -p "External Tenant ID (optional): " EXTERNAL_TID; \
	read -p "Tenant Name: " TENANT_NAME; \
	if [ -z "$$TENANT_ID" ] || [ -z "$$TENANT_NAME" ]; then \
		echo "$(YELLOW)Tenant ID and Tenant Name are required.$(NC)"; \
		exit 1; \
	fi; \
	if [ -z "$(DATABASE_URL)" ]; then \
		DB_URL="postgres://user:password@localhost:5435/sessiondb?sslmode=disable"; \
	else \
		DB_URL="$(DATABASE_URL)"; \
	fi; \
	psql $$DB_URL -c "INSERT INTO tenants (id, external_tid, name) VALUES ('$$TENANT_ID', NULLIF('$$EXTERNAL_TID',''), '$$TENANT_NAME') ON CONFLICT (id) DO UPDATE SET external_tid = EXCLUDED.external_tid, name = EXCLUDED.name;"; \
	echo "$(GREEN)Tenant created/updated: $$TENANT_ID$(NC)"

create-user: ## Create or update a user record
	@echo "$(GREEN)Creating/updating user...$(NC)"
	@read -p "User ID (internal, required): " USER_ID; \
	read -p "Tenant ID (must already exist): " TENANT_ID; \
	read -p "Email (optional): " EMAIL; \
	read -p "Full Name (optional): " FULL_NAME; \
	read -p "Phone Number (optional): " PHONE; \
	if [ -z "$$USER_ID" ] || [ -z "$$TENANT_ID" ]; then \
		echo "$(YELLOW)User ID and Tenant ID are required.$(NC)"; \
		exit 1; \
	fi; \
	if [ -z "$(DATABASE_URL)" ]; then \
		DB_URL="postgres://user:password@localhost:5435/sessiondb?sslmode=disable"; \
	else \
		DB_URL="$(DATABASE_URL)"; \
	fi; \
	psql $$DB_URL -c "INSERT INTO users (id, tenant_id, email, full_name, phone_number) VALUES ('$$USER_ID', '$$TENANT_ID', NULLIF('$$EMAIL',''), NULLIF('$$FULL_NAME',''), NULLIF('$$PHONE','')) ON CONFLICT (id) DO UPDATE SET tenant_id = EXCLUDED.tenant_id, email = EXCLUDED.email, full_name = EXCLUDED.full_name, phone_number = EXCLUDED.phone_number;"; \
	echo "$(GREEN)User created/updated: $$USER_ID (tenant $$TENANT_ID)$(NC)"

install-tools: ## Install development tools
	@echo "$(GREEN)Installing development tools...$(NC)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(GREEN)Tools installed!$(NC)"

swagger: ## Generate Swagger documentation
	@echo "$(GREEN)Generating Swagger documentation...$(NC)"
	@if ! command -v swag > /dev/null && [ ! -f $$(go env GOPATH)/bin/swag ] && [ ! -f $(HOME)/go/bin/swag ]; then \
		echo "$(YELLOW)swag not found. Installing...$(NC)"; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	fi
	@if command -v swag > /dev/null; then \
		swag init -g cmd/server/main.go -o docs; \
	elif [ -f $$(go env GOPATH)/bin/swag ]; then \
		$$(go env GOPATH)/bin/swag init -g cmd/server/main.go -o docs; \
	elif [ -f $(HOME)/go/bin/swag ]; then \
		$(HOME)/go/bin/swag init -g cmd/server/main.go -o docs; \
	else \
		echo "$(YELLOW)swag not found. Please install it manually: go install github.com/swaggo/swag/cmd/swag@latest$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)Swagger documentation generated in docs/$(NC)"

ci: deps lint test ## Run CI checks (deps, lint, test)

all: clean deps build test ## Clean, download deps, build, and test

