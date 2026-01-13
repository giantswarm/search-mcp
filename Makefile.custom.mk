##@ Custom Targets

# Binary name
BINARY_NAME := mcp-server
BIN_DIR := bin

# Docker image settings
DOCKER_IMAGE := gsoci.azurecr.io/giantswarm/search-mcp
DOCKER_TAG := $(shell git describe --tags --always --dirty)

.PHONY: build
build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/server
	@echo "Binary built: $(BIN_DIR)/$(BINARY_NAME)"

.PHONY: build-linux
build-linux: ## Build the binary for Linux
	@echo "Building $(BINARY_NAME) for Linux..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/server
	@echo "Binary built: $(BIN_DIR)/$(BINARY_NAME)-linux-amd64"

.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

.PHONY: test-coverage
test-coverage: test ## Run tests with coverage report
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: lint
lint: ## Run linters
	@echo "Running linters..."
	@which golangci-lint > /dev/null 2>&1 || (echo "golangci-lint not found. Install from https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Format code
	@echo "Formatting code..."
	gofmt -s -w .
	goimports -local github.com/giantswarm/search-mcp -w .

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest
	@echo "Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

.PHONY: docker-push
docker-push: ## Push Docker image
	@echo "Pushing Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

.PHONY: run
run: ## Run the server locally (stdio transport)
	@echo "Running $(BINARY_NAME) with stdio transport..."
	go run ./cmd/server

.PHONY: run-http
run-http: ## Run the server locally (HTTP transport)
	@echo "Running $(BINARY_NAME) with HTTP transport..."
	go run ./cmd/server --transport=streamable-http --http-addr=:8080

.PHONY: run-debug
run-debug: ## Run the server with debug logging
	@echo "Running $(BINARY_NAME) with debug logging..."
	go run ./cmd/server --debug

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html

.PHONY: deps
deps: ## Download and tidy dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

.PHONY: verify
verify: fmt lint test ## Run all verification steps (format, lint, test)
	@echo "All verification steps passed!"
