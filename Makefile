# Parental Control Application Makefile

# Version information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S_UTC')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go configuration
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

# Binary names
BINARY_NAME = parental-control
BINARY_UNIX = $(BINARY_NAME)_unix
BINARY_WINDOWS = $(BINARY_NAME).exe

# Build flags
LDFLAGS = -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"
BUILD_FLAGS = $(LDFLAGS)

# Production build flags (optimized)
PROD_LDFLAGS = -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"
PROD_BUILD_FLAGS = $(PROD_LDFLAGS)

# Build directories
BUILD_DIR = build
CMD_DIR = cmd/parental-control

# Find all Go source files
GO_FILES = $(shell find . -name '*.go')

.PHONY: all build build-prod clean test deps tidy lint fmt help
.PHONY: build-linux build-windows build-cross
.PHONY: run install uninstall version

# Default target
all: clean deps test build

# Help target
help: ## Show this help message
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Create build directory
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Build for current platform
build: $(GO_FILES) $(BUILD_DIR) ## Build binary for current platform
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

# Production build (optimized)
build-prod: $(GO_FILES) $(BUILD_DIR) ## Build optimized binary for current platform
	$(GOBUILD) $(PROD_BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

# Cross-platform builds
build-linux: $(BUILD_DIR) ## Build for Linux
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(PROD_BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_UNIX) ./$(CMD_DIR)

build-windows: $(BUILD_DIR) ## Build for Windows
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(PROD_BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_WINDOWS) ./$(CMD_DIR)

build-cross: build-linux build-windows ## Build for all target platforms

# Development tasks
run: build ## Build and run the application
	./$(BUILD_DIR)/$(BINARY_NAME)

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

test: ## Run tests
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	$(GOTEST) -v -coverprofile=$(BUILD_DIR)/coverage.out ./...
	$(GOCMD) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html

# Dependency management
deps: ## Download dependencies
	$(GOGET) -d ./...

tidy: ## Clean up dependencies
	$(GOMOD) tidy

# Code quality
fmt: ## Format code
	$(GOCMD) fmt ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

# Version information
version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"

# Installation targets (for development)
install: build-prod ## Install to system (requires sudo on Linux)
	@echo "Installing parental-control service..."
ifeq ($(shell uname), Linux)
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installed to /usr/local/bin/$(BINARY_NAME)"
else ifeq ($(shell uname), Darwin)
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Installed to /usr/local/bin/$(BINARY_NAME)"
else
	@echo "Manual installation required on this platform"
	@echo "Copy $(BUILD_DIR)/$(BINARY_NAME) to your desired location"
endif

uninstall: ## Uninstall from system
	@echo "Uninstalling parental-control service..."
ifeq ($(shell uname), Linux)
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Removed from /usr/local/bin/"
else ifeq ($(shell uname), Darwin)
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Removed from /usr/local/bin/"
else
	@echo "Manual uninstallation required on this platform"
endif 