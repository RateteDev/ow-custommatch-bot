# 実行ファイル名です。
APP_NAME := ow-custommatch-bot
# メインパッケージのパスです。
CMD_PATH := ./cmd/ow-custommatch-bot/
# 開発用ビルドを置くディレクトリです。
BIN_DIR := bin
# ローカル実行用バイナリの出力先です。
BIN_PATH := $(BIN_DIR)/$(APP_NAME)
# Windows 向けローカルビルドの出力先です。
WIN_BIN_PATH := $(BIN_DIR)/$(APP_NAME).exe
# 配布用ファイルを置くディレクトリです。
DIST_DIR := dist
# Release 用 exe のコピー先です。
WIN_RELEASE_EXE := $(DIST_DIR)/$(APP_NAME).exe
# バージョン文字列です。未指定時は exact tag -> dev-<shortsha> -> dev の順で解決します。
VERSION_FROM_GIT = $(shell \
	if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then \
		tag=$$(git describe --tags --exact-match 2>/dev/null); \
		if [ -n "$$tag" ]; then \
			printf '%s' "$$tag"; \
		else \
			sha=$$(git rev-parse --short HEAD 2>/dev/null); \
			if [ -n "$$sha" ]; then \
				printf 'dev-%s' "$$sha"; \
			else \
				printf 'dev'; \
			fi; \
		fi; \
	else \
		printf 'dev'; \
	fi)
VERSION ?= $(VERSION_FROM_GIT)
# バージョン埋め込み用の ldflags です。
LDFLAGS := -X main.version=$(VERSION)

.PHONY: test build run build-win release-win-exe

# 全テストを実行します。
test:
	go test ./...

# 開発用のローカルビルドを行います。
build:
	mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_PATH) $(CMD_PATH)

# ローカルビルド後にそのまま起動します。
run: build
	./$(BIN_PATH)

# Windows 向け exe をビルドします。
build-win:
	mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(WIN_BIN_PATH) $(CMD_PATH)

# Release 配布用の Windows exe を dist に配置します。
release-win-exe: build-win
	mkdir -p $(DIST_DIR)
	cp "$(WIN_BIN_PATH)" "$(WIN_RELEASE_EXE)"
