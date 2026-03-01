# Context

現在の配布形式は Windows 実行ファイル（exe + .env）のみ。
Linux サーバーで常時稼働させたいユーザーは Go をインストールしてビルドするか、
バイナリをダウンロードして systemd 等を自力で設定する必要がある。

`Dockerfile` と `docker-compose.yml` を提供することで、
Docker が使える環境であれば Go 環境なしで 1 コマンド起動できるようにする。

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `Dockerfile`（新規） | マルチステージビルドで軽量イメージを作成 |
| `docker-compose.yml`（新規） | .env / db のマウント設定 |
| `README.md` | Docker による起動手順を追記 |

# 実施順序

- 要件1（Dockerfile）→ 要件2（compose）→ 要件3（README）の順で実施

# 要件1: Dockerfile

マルチステージビルドで distroless / alpine ベースの軽量イメージを作成する。

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /ow-custommatch-bot ./cmd/ow-custommatch-bot/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /data
COPY --from=builder /ow-custommatch-bot /usr/local/bin/ow-custommatch-bot
ENTRYPOINT ["ow-custommatch-bot"]
```

# 要件2: docker-compose.yml

```yaml
services:
  bot:
    build: .
    restart: unless-stopped
    volumes:
      - ./.env:/data/.env:ro
      - ./data:/data
    environment:
      - NO_COLOR=1
```

- `.env` は読み取り専用マウント
- DB ファイルは `./data/` に永続化

# 要件3: README への Docker 手順追加

```markdown
## Docker で起動する場合

1. `.env` を作成して `BOT_TOKEN` を設定
2. `docker compose up -d`
3. `docker compose logs -f` でログを確認
```

# 検証方法

## 手動確認

- `docker build .` がエラーなく完了すること
- `docker compose up` で Bot が起動し Discord 接続ログが出ること
- `data/` ディレクトリに `ow-custommatch-bot.db` が生成されること
- コンテナ再起動後も DB が保持されること

---

## 不採用理由

このボットの対象ユーザーは Overwatch のカスタムマッチ主催者であり、
ほぼ全員が Windows 環境でゲームをプレイしている。

Docker を使えるユーザーは少数派であり、対応コストに対してメリットが薄い。
現在の配布形式（Windows exe + .env の zip）で十分にカバーできていると判断した。
