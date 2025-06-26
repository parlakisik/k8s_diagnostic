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