APP_NAME := ow-custommatch-bot
CMD_PATH := ./cmd/ow-custommatch-bot/
BIN_DIR := bin
BIN_PATH := $(BIN_DIR)/$(APP_NAME)
WIN_BIN_PATH := $(BIN_DIR)/$(APP_NAME).exe
DIST_DIR := dist
WIN_PACKAGE_NAME := $(APP_NAME)-win64
WIN_PACKAGE_DIR := $(DIST_DIR)/$(WIN_PACKAGE_NAME)
WIN_PACKAGE_ZIP := $(DIST_DIR)/$(WIN_PACKAGE_NAME).zip
WIN_GUIDE_PATH := assets/windows/使い方.html
ENV_TEMPLATE_PATH := .env.example

.PHONY: test build run build-win package-win

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

package-win: build-win
	test -f "$(WIN_GUIDE_PATH)" || (echo "missing file: $(WIN_GUIDE_PATH)" && exit 1)
	test -f "$(ENV_TEMPLATE_PATH)" || (echo "missing file: $(ENV_TEMPLATE_PATH)" && exit 1)
	test -f "LICENSE" || (echo "missing file: LICENSE" && exit 1)
	mkdir -p $(DIST_DIR)
	rm -rf "$(WIN_PACKAGE_DIR)" "$(WIN_PACKAGE_ZIP)"
	mkdir -p "$(WIN_PACKAGE_DIR)"
	cp "$(WIN_BIN_PATH)" "$(WIN_PACKAGE_DIR)/"
	cp "$(ENV_TEMPLATE_PATH)" "$(WIN_PACKAGE_DIR)/.env"
	cp "$(WIN_GUIDE_PATH)" "$(WIN_PACKAGE_DIR)/使い方.html"
	cp "LICENSE" "$(WIN_PACKAGE_DIR)/LICENSE"
	cd "$(DIST_DIR)" && zip -r "$(WIN_PACKAGE_NAME).zip" "$(WIN_PACKAGE_NAME)"
