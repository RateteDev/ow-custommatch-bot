# Context

`cmd/ow-custommatch-bot/main.go` の `var version = "dev"` がハードコードされており、
ビルド済み実行ファイルでも `--version` が常に `dev` を表示する。

Go の `-ldflags "-X main.version=<TAG>"` を使い、ビルド時にバージョン文字列を埋め込む。
Makefile と GitHub Actions（plan 009）の両方から利用できるようにする。

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `Makefile` | `VERSION` 変数と `-ldflags` を build / build-win / package-win に追加 |

# 実施順序

依存関係なし。plan 009 の前に対応しておくと望ましい。

# 要件1: Makefile への ldflags 追加

```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.version=$(VERSION)

build:
	mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_PATH) $(CMD_PATH)

build-win:
	mkdir -p $(BIN_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(WIN_BIN_PATH) $(CMD_PATH)
```

- `VERSION` は環境変数で上書き可能（CI から `make build VERSION=v1.0.0` で呼べる）
- タグなし環境では `git describe` のフォールバックで `dev` を使う

# 検証方法

## 手動確認

- `make build` 後、`./bin/ow-custommatch-bot --version` が `ow-custommatch-bot dev` 以外（git describe の結果）を表示することを確認
- `make build VERSION=v1.2.3` で `v1.2.3` が表示されることを確認
- `go test ./...` が引き続きパスすることを確認

## 実装結果

- `Makefile`: VERSION / LDFLAGS 変数を追加、build / build-win に `-ldflags` を反映
- `make build && ./bin/ow-custommatch-bot --version` → `v0.1.0-3-g4b387a8` ✅
- `go test ./...` 全件パス ✅
- `go build ./...` 成功 ✅

## 次期改善事項

- `make build VERSION=v1.2.3` の CI 連携は plan 009 で対応済み
