APP_NAME := ow-custommatch-bot
CMD_PATH := ./cmd/ow-custommatch-bot/
BIN_DIR := bin
BIN_PATH := $(BIN_DIR)/$(APP_NAME)
WIN_BIN_PATH := $(BIN_DIR)/$(APP_NAME).exe
DIST_DIR := dist
WIN_RELEASE_EXE := $(DIST_DIR)/$(APP_NAME).exe
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.version=$(VERSION)

.PHONY: test build run build-win release-win-exe

test:
	go test ./...

build:
	mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_PATH) $(CMD_PATH)

run: build
	./$(BIN_PATH)

build-win:
	mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(WIN_BIN_PATH) $(CMD_PATH)

release-win-exe: build-win
	mkdir -p $(DIST_DIR)
	cp "$(WIN_BIN_PATH)" "$(WIN_RELEASE_EXE)"
