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
.PHONY: all build run test clean deps lint fmt sim prod check-env

# Default target
all: build

# Build the application
build: deps
	@echo "Building $(PROJECT_NAME)..."
	@mkdir -p $(BIN_DIR)
	@$(GO) build -o $(BIN_DIR)/$(PROJECT_NAME) $(CMD_DIR)/main.go
	@echo "Build completed: $(BIN_DIR)/$(PROJECT_NAME)"

# Run the application (default config)
run: deps
	@echo "Running $(PROJECT_NAME)..."
	@$(GO) run $(CMD_DIR)/main.go

# Run in simulation mode (模拟盘)
sim: deps
	@echo "============================================"
	@echo "  启动模式: 模拟盘 (SIMULATION)"
	@echo "============================================"
	@$(GO) run $(CMD_DIR)/main.go -env simulation

# Run in production mode (实盘) - requires confirmation
prod: deps
	@echo "============================================"
	@echo "  警告: 实盘模式 (PRODUCTION)"
	@echo "  将使用真实资金!"
	@echo "============================================"
	@read -p "确认启动实盘模式? (输入 yes 继续): " confirm; \
	if [ "$$confirm" != "yes" ]; then \
		echo "已取消"; \
		exit 1; \
	fi
	@if [ -z "$$OKX_API_KEY" ]; then \
		echo "错误: 请先设置 OKX_API_KEY 环境变量"; \
		exit 1; \
	fi
	@$(GO) run $(CMD_DIR)/main.go -env production

# Check environment configuration (环境检查)
check-env:
	@echo "检查环境配置..."
	@echo ""
	@echo "=== 环境变量 ==="
	@if [ -n "$$OKX_API_KEY" ]; then \
		echo "OKX_API_KEY: 已设置"; \
	else \
		echo "OKX_API_KEY: 未设置"; \
	fi
	@if [ -n "$$OKX_SECRET_KEY" ]; then \
		echo "OKX_SECRET_KEY: 已设置"; \
	else \
		echo "OKX_SECRET_KEY: 未设置"; \
	fi
	@if [ -n "$$OKX_PASSPHRASE" ]; then \
		echo "OKX_PASSPHRASE: 已设置"; \
	else \
		echo "OKX_PASSPHRASE: 未设置"; \
	fi
	@if [ -n "$$CRYPTOQUANT_API_KEY" ]; then \
		echo "CRYPTOQUANT_API_KEY: 已设置"; \
	else \
		echo "CRYPTOQUANT_API_KEY: 未设置"; \
	fi
	@echo ""
	@echo "=== QUANT_ENV ==="
	@if [ -n "$$QUANT_ENV" ]; then \
		echo "QUANT_ENV: $$QUANT_ENV"; \
	else \
		echo "QUANT_ENV: 未设置 (将使用默认配置)"; \
	fi
	@echo ""
	@echo "=== 配置文件 ==="
	@if [ -f "configs/config.sim.yaml" ]; then \
		echo "configs/config.sim.yaml: 存在"; \
	else \
		echo "configs/config.sim.yaml: 不存在"; \
	fi
	@if [ -f "configs/config.prod.yaml" ]; then \
		echo "configs/config.prod.yaml: 存在"; \
	else \
		echo "configs/config.prod.yaml: 不存在"; \
	fi

# Run tests
test: deps
	@echo "Running tests..."
	@$(GO) test ./... -v

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	@$(GO) test -cover ./internal/risk ./internal/execution ./internal/strategy

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	@$(GO) test -race ./...

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
	@echo "============================================"
	@echo "  OKX量化交易系统 - 可用命令"
	@echo "============================================"
	@echo ""
	@echo "构建命令:"
	@echo "  make build     - 编译程序"
	@echo "  make build-all - 编译所有平台版本"
	@echo "  make clean     - 清理编译产物"
	@echo ""
	@echo "运行命令:"
	@echo "  make run       - 运行程序 (默认配置)"
	@echo "  make sim       - 模拟盘模式运行"
	@echo "  make prod      - 实盘模式运行 (需确认)"
	@echo ""
	@echo "测试命令:"
	@echo "  make test       - 运行测试"
	@echo "  make test-cover - 运行覆盖率测试"
	@echo "  make test-race  - 运行竞态检测"
	@echo ""
	@echo "环境命令:"
	@echo "  make check-env  - 检查环境配置"
	@echo ""
	@echo "其他命令:"
	@echo "  make deps      - 安装依赖"
	@echo "  make lint      - 代码检查"
	@echo "  make fmt       - 格式化代码"
	@echo "  make help      - 显示帮助"
	@echo ""
	@echo "============================================"
	@echo "快速切换环境:"
	@echo "  make sim       -> 模拟盘"
	@echo "  make prod      -> 实盘"
	@echo "============================================"
