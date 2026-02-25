APP_NAME := ow-custommatch-bot
CMD_PATH := ./cmd/ow-custommatch-bot/
BIN_DIR := bin
BIN_PATH := $(BIN_DIR)/$(APP_NAME)
WIN_BIN_PATH := $(BIN_DIR)/$(APP_NAME).exe

.PHONY: test build run build-win

test:
	go test ./...

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_PATH) $(CMD_PATH)

run: build
	./$(BIN_PATH)

build-win:
	mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(WIN_BIN_PATH) $(CMD_PATH)
