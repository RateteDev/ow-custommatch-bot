# 開発者向けガイド

このドキュメントは、MatchyBot をソースコードから変更・ビルドする開発者向けです。

## 必要なもの

- Go 1.23 以上

```bash
go version
```

## セットアップ

依存関係を取得:

```bash
go mod tidy
```

## ビルド

```bash
go build -o matchybot ./cmd/matchybot
```

Windows 向け exe:

```bash
GOOS=windows GOARCH=amd64 go build -o matchybot.exe ./cmd/matchybot
```

## テスト

```bash
go test ./...
```
