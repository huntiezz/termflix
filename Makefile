APP_NAME := termflix
BIN_DIR := bin

.PHONY: build build-mac build-linux build-win build-all install clean

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/termflix

build-mac:
	mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=arm64 go build -o $(BIN_DIR)/$(APP_NAME)-mac-arm64 ./cmd/termflix
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/$(APP_NAME)-mac-amd64 ./cmd/termflix

build-linux:
	mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(APP_NAME)-linux-amd64 ./cmd/termflix

build-win:
	mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(BIN_DIR)/$(APP_NAME)-windows-amd64.exe ./cmd/termflix

build-all: build-mac build-linux build-win

install:
	go install ./cmd/termflix

clean:
	rm -rf $(BIN_DIR)