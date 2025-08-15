.PHONY: build run test clean install help

# Build variables
BINARY_NAME=k8s-diagnostic
BUILD_DIR=build
MAIN_PACKAGE=.

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PACKAGE)

run: ## Run the application
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) -v $(MAIN_PACKAGE)
	./$(BUILD_DIR)/$(BINARY_NAME)

test: ## Run tests
	$(GOTEST) -v ./...

clean: ## Clean build files
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

install: ## Install the binary
	$(GOCMD) install

# Development commands
dev-setup: ## Set up development environment
	$(GOMOD) download
	mkdir -p $(BUILD_DIR)

lint: ## Run linter
	golangci-lint run

# Build for different platforms
build-linux: ## Build for Linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)

build-windows: ## Build for Windows  
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)

build-darwin: ## Build for macOS
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)

build-all: build-linux build-windows build-darwin ## Build for all platforms

# ==========================================
# DOCKER BUILD TARGETS
# ==========================================
docker-build: docker-build-ui docker-build-cli ## Build all Docker containers

docker-build-cli: build-linux ## Build CLI Docker container
	@echo "🐳 Building CLI Docker container..."
	docker build -f Dockerfile.cli-simple -t k8s_diagnostic-k8s-diagnostic-cli .
	@echo "✅ CLI container built successfully"

docker-build-ui: ## Build UI Docker container
	@echo "🐳 Building UI Docker container..."
	docker build -f Dockerfile.ui -t k8s_diagnostic-k8s-diagnostic-ui ./web
	@echo "✅ UI container built successfully"

docker-compose-build: build-linux ## Build with docker-compose
	@echo "🐳 Building containers with docker-compose..."
	docker compose build
	@echo "✅ All containers built successfully"

docker-up: docker-compose-build ## Build and start containers
	@echo "🚀 Starting k8s-diagnostic services..."
	docker compose up -d k8s-diagnostic-ui
	@echo "✅ Services started! UI available at http://localhost:3000"

docker-down: ## Stop containers
	@echo "🛑 Stopping k8s-diagnostic services..."
	docker compose down
	@echo "✅ Services stopped"

docker-clean: ## Clean Docker images and containers
	@echo "🧹 Cleaning Docker artifacts..."
	docker compose down --rmi all --volumes --remove-orphans
	@echo "✅ Docker cleanup complete"
