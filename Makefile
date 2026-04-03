APP_NAME := termflix
BIN_DIR := bin

.PHONY: all build run test fmt lint clean

all: build

build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/termflix

run: build
	@$(BIN_DIR)/$(APP_NAME) $(ARGS)

test:
	go test ./...

fmt:
	go fmt ./...

lint:
	@echo "Running go vet..."
	go vet ./...

clean:
	rm -rf $(BIN_DIR)

