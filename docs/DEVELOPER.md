# 開発者向けガイド

このドキュメントは、ow-custommatch-bot をソースコードから変更・ビルドする開発者向けです。

以降の開発・ビルド・実行・テスト手順は、原則として `Makefile` 経由で実行します。
（個別の `go build` / `go test` コマンドは日常運用では使用しない前提）

## 必要なもの

- Go 1.26 以上

## セットアップ

初回のみ、依存パッケージを取得してください。

```bash
go mod download
```

## 開発用コマンド（Makefile）

| コマンド | 内容 |
|---|---|
| `make test` | ユニットテスト実行 |
| `make build` | `bin/ow-custommatch-bot` をビルド |
| `make run` | ビルド後に実行（内部で `make build` も実行） |
| `make release-win-exe` | 配布用 exe を `dist/` に生成 |

## ビルド

```bash
make build
```

出力先: `bin/ow-custommatch-bot`

## Windows 配布用 exe

```bash
make release-win-exe
```

出力先: `dist/ow-custommatch-bot.exe`

## 実行時に必要なファイル

- `ow-custommatch-bot.db` — 初回起動時に自動生成されます。

**補足:**

- ランクマスタは `go:embed` でバイナリに埋め込まれています。
- SQLite DB・VC 設定・ログは `%LOCALAPPDATA%\ow-custommatch-bot\` 配下に作成されます。
- `BOT_TOKEN` は `.env` ではなく Windows Credential Manager の `ow-custommatch-bot/BOT_TOKEN` に保存されます。

## 環境変数

テストモード（任意）:

- `OW_CUSTOMMATCH_BOT_TEST_MODE=true`

テストモードを有効化すると、`/match` コマンドに `fill` オプション（boolean）が追加されます。
`fill=true` で募集開始した場合、ダミープレイヤーを 20〜60 人ランダム追加してテスト用の振り分けを行えます。

**補足:**

- 判定は文字列一致のため、`true`（小文字）を設定してください。
- テストモードを無効化したい場合は、未設定にするか `true` 以外の値を設定してください。

## テスト

```bash
make test
```

## 保存済みトークンの削除（開発者向け）

利用者向け画面には削除導線を出していません。開発や検証で保存済みトークンを消したい場合は、Windows のコマンドプロンプトまたは PowerShell で以下を実行してください。

```powershell
cmdkey /delete:ow-custommatch-bot/BOT_TOKEN
```

削除後に通常起動すると、初回起動と同様に `BOT_TOKEN` の入力を求められます。

## 配布素材（Windows）

Windows 配布用の説明ファイルは以下に配置します。

- `assets/windows/使い方.html`
