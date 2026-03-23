# Makefile for OKX Quant Trading System

# Go settings
GO=go
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

# Project settings
PROJECT_NAME=okx-trader
VERSION=1.0.0

# Directories
CMD_DIR=cmd/trader
BUILD_DIR=build
BIN_DIR=bin

# Targets
.PHONY: all build run test clean deps lint fmt

# Default target
all: build

# Build the application
build: deps
	@echo "Building $(PROJECT_NAME)..."
	@mkdir -p $(BIN_DIR)
	@$(GO) build -o $(BIN_DIR)/$(PROJECT_NAME) $(CMD_DIR)/main.go
	@echo "Build completed: $(BIN_DIR)/$(PROJECT_NAME)"

# Run the application
run: deps
	@echo "Running $(PROJECT_NAME)..."
	@$(GO) run $(CMD_DIR)/main.go

# Run tests
test: deps
	@echo "Running tests..."
	@$(GO) test ./... -v

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR) $(BIN_DIR)
	@rm -f *.log

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@$(GO) mod tidy

# Lint the code
lint:
	@echo "Linting code..."
	@$(GO) fmt ./...
	@if command -v golint > /dev/null; then golint ./...; else echo "golint not found, skipping linting"; fi

# Format code
fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...

# Build for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	
	# Linux
	@GOOS=linux GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(PROJECT_NAME)-linux-amd64 $(CMD_DIR)/main.go
	
	# macOS
	@GOOS=darwin GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(PROJECT_NAME)-darwin-amd64 $(CMD_DIR)/main.go
	@GOOS=darwin GOARCH=arm64 $(GO) build -o $(BUILD_DIR)/$(PROJECT_NAME)-darwin-arm64 $(CMD_DIR)/main.go
	
	# Windows
	@GOOS=windows GOARCH=amd64 $(GO) build -o $(BUILD_DIR)/$(PROJECT_NAME)-windows-amd64.exe $(CMD_DIR)/main.go
	
	@echo "Builds completed in $(BUILD_DIR)/"

# Show help
help:
	@echo "Available commands:"
	@echo "  make all       - Build the application"
	@echo "  make build     - Build the application"
	@echo "  make run       - Run the application"
	@echo "  make test      - Run tests"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make deps      - Install dependencies"
	@echo "  make lint      - Lint the code"
	@echo "  make fmt       - Format code"
	@echo "  make build-all - Build for all platforms"
	@echo "  make help      - Show this help"
