.PHONY: all build build-all run test test-v clean docker-build docker-up docker-down docker-logs help deps vet fmt lint setup docs

APP_NAME := graft
GO_CMD := go
CMD_PATH := cmd/graft/main.go
BUILD_DIR := bin

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")

LDFLAGS := -ldflags="-X 'github.com/DongDuong2001/graft/internal/version.Version=$(VERSION)' \
                     -X 'github.com/DongDuong2001/graft/internal/version.Commit=$(COMMIT)' \
                     -X 'github.com/DongDuong2001/graft/internal/version.BuildDate=$(BUILD_DATE)'"

all: build

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build         Build the binary"
	@echo "  build-all     Build binaries for multiple platforms"
	@echo "  run           Run the application"
	@echo "  docs          Generate CLI documentation"
	@echo "  test          Run tests"
	@echo "  test-v        Run tests with verbose output"
	@echo "  clean         Clean build artifacts"
	@echo "  docker-build  Build Docker image"
	@echo "  docker-up     Start Docker containers in background"
	@echo "  docker-down   Stop Docker containers"
	@echo "  docker-logs   Follow Docker logs"
	@echo "  deps          Tidy go modules"
	@echo "  vet           Run go vet"
	@echo "  fmt           Format all code"
	@echo "  lint          Run golangci-lint"
	@echo "  setup         Create .env and generate DEV keys automatically"

## build: Build the binary
build:
	@echo "Building..."
	-@mkdir $(BUILD_DIR) 2>/dev/null || true
	$(GO_CMD) build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(CMD_PATH)

## build-all: Build for multiple operating systems
build-all:
	@echo "Building all platforms via Go script..."
	$(GO_CMD) run scripts/build.go

## docs: Generate CLI markdown documentation
docs:
	@echo "Generating CLI documentation..."
	$(GO_CMD) run $(LDFLAGS) $(CMD_PATH) docs ./docs/cli

## run: Run the application
run:
	$(GO_CMD) run $(LDFLAGS) $(CMD_PATH)

## test: Run tests
test:
	$(GO_CMD) test ./...

## test-v: Run tests with verbose output
test-v:
	$(GO_CMD) test -v ./...

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)

## docker-build: Build Docker image
docker-build:
	docker build -t $(APP_NAME):latest -f deployments/Dockerfile .

## deps: Tidy go modules
deps:
	$(GO_CMD) mod tidy

## vet: Run go vet
vet:
	$(GO_CMD) vet ./...

## fmt: Format all code
fmt:
	$(GO_CMD) fmt ./...

## lint: Run golangci-lint
lint:
	golangci-lint run

## setup: Automatically copy example.env to .env and generate random keys
setup:
	@if [ ! -f .env ]; then \
		echo "Creating .env from configs/example.env..."; \
		cp configs/example.env .env; \
		if command -v openssl >/dev/null 2>&1; then \
			MASTER=$$(openssl rand -hex 32); \
			ADMIN=ak_$$(openssl rand -hex 16); \
			sed -i.bak -e "s/MASTER_KEY=0000000000000000000000000000000000000000000000000000000000000000/MASTER_KEY=$$MASTER/" .env; \
			sed -i.bak -e "s/ADMIN_API_KEY=change-me-to-a-long-random-string/ADMIN_API_KEY=$$ADMIN/" .env; \
			rm -f .env.bak; \
			echo ".env created with secure DEV keys!"; \
		else \
			echo "openssl not found. Please manually update .env with random keys."; \
		fi \
	else \
		echo ".env already exists. Skipping setup."; \
	fi

## docker-up: Start Docker containers
docker-up:
	docker compose -f deployments/docker-compose.yml up -d --build

## docker-down: Stop Docker containers
docker-down:
	docker compose -f deployments/docker-compose.yml down

## docker-logs: Follow Docker logs
docker-logs:
	docker compose -f deployments/docker-compose.yml logs -f
