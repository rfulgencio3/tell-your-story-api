.PHONY: help run build test test-coverage clean lint fmt docker-up docker-down docker-build deps dev-setup dev-watch

BINARY_NAME=tell-your-story
MAIN_PATH=./cmd/server
BUILD_DIR=./bin

help: ## Show all commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

run: ## Run the server
	go run $(MAIN_PATH)/main.go

build: ## Build binary
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)/main.go

test: ## Run tests
	go test ./... -v

test-coverage: ## Run tests with coverage
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR) coverage.out coverage.html tmp/

lint: ## Run linter
	golangci-lint run ./...

fmt: ## Format code
	go fmt ./...
	goimports -w .

docker-up: ## Start Docker containers
	docker-compose up -d

docker-down: ## Stop Docker containers
	docker-compose down

docker-build: ## Build Docker image
	docker-compose build

deps: ## Download dependencies
	go mod download
	go mod tidy

dev-setup: ## Setup development tools
	go install github.com/air-verse/air@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

dev-watch: ## Watch for changes (air)
	air
