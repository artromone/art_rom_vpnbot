.PHONY: build run clean test fmt vet

BINARY_NAME=xray-telegram-bot
BUILD_DIR=./build

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/main.go

run: build
	@echo "Running $(BINARY_NAME)..."
	$(BUILD_DIR)/$(BINARY_NAME)

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	go clean

test:
	@echo "Running tests..."
	go test -v ./...

fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Vetting code..."
	go vet ./...

install-deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

docker-build:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME) .

.DEFAULT_GOAL := build

