# 開発者向けガイド

このドキュメントは、MatchyBot をソースコードから変更・ビルドする開発者向けです。

## 必要なもの

- Go 1.22 以上

```bash
go version
```

## セットアップ

依存関係を取得:

```bash
go mod tidy
```

## ビルド出力先

当面、ビルド済みファイルはリポジトリ内の `bin/` に配置します（最終的には Release 配布を想定）。

## ビルド

```bash
go build -o bin/matchybot ./cmd/matchybot
```

Windows 向け exe:

```bash
GOOS=windows GOARCH=amd64 go build -o bin/matchybot.exe ./cmd/matchybot
```

## 実行時に必要なファイル（bin 配下）

- `.env`（`BOT_TOKEN` を設定）
- `matchybot.db`（初回起動時に自動生成されるため事前作成不要）

```bash
cp .env.example bin/.env
```

補足:

- ランクマスタは `go:embed` でバイナリに埋め込まれています。
- `matchybot.db` は実行ファイルと同じディレクトリに作成されます。

## テスト

```bash
go test ./...
```
