.PHONY: all build run test clean docker-build help dev

APP_NAME := graft
GO_CMD := go
CMD_PATH := cmd/graft/main.go
BUILD_DIR := bin

all: build

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build         Build the binary"
	@echo "  run           Run the application"
	@echo "  test          Run tests"
	@echo "  test-v        Run tests with verbose output"
	@echo "  clean         Clean build artifacts"
	@echo "  docker-build  Build Docker image"
	@echo "  deps          Tidy go modules"
	@echo "  vet           Run go vet"

## build: Build the binary
build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	$(GO_CMD) build -o $(BUILD_DIR)/$(APP_NAME) $(CMD_PATH)

## run: Run the application
run:
	$(GO_CMD) run $(CMD_PATH)

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
