.PHONY: help build run clean test lint install-deps

help: ## Display this help message
	@echo "EBS SSO Gateway - Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

install-deps: ## Install Go dependencies
	go mod download
	go mod tidy

build: ## Build the application
	go build -o ebssso

build-linux: ## Build for Linux (amd64)
	GOOS=linux GOARCH=amd64 go build -o ebssso

build-all: ## Build for multiple platforms
	GOOS=linux GOARCH=amd64 go build -o ebssso-linux-amd64
	GOOS=darwin GOARCH=amd64 go build -o ebssso-darwin-amd64
	GOOS=windows GOARCH=amd64 go build -o ebssso-windows-amd64.exe

run: ## Run the application
	go run main.go

clean: ## Clean build artifacts
	rm -f ebssso ebssso-*
	go clean

test: ## Run tests
	go test -v ./...

test-coverage: ## Run tests with coverage
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint: ## Run linter (requires golangci-lint)
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run

fmt: ## Format code
	go fmt ./...
	gofmt -s -w .

vet: ## Run go vet
	go vet ./...

check: fmt vet lint test ## Run all checks (format, vet, lint, test)

docker-build: ## Build Docker image
	docker build -t ebssso:latest .

docker-run: ## Run Docker container
	docker run -p 8080:8080 -v $(PWD)/config.yaml:/config.yaml ebssso:latest

.DEFAULT_GOAL := help
