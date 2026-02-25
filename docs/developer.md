# 開発者向けガイド

このドキュメントは、ow-custommatch-bot をソースコードから変更・ビルドする開発者向けです。

以降の開発・ビルド・実行・テスト手順は、原則として `Makefile` 経由で実行します。
（個別の `go build` / `go test` コマンドは日常運用では使用しない前提）

## 必要なもの

- Go 1.22 以上

## セットアップ

初回のみ、Go Modules の依存関係を取得・整備してください。

## 開発用コマンド（Makefile）

`Makefile` を用意しているため、開発時は以下を利用できます。

```bash
make test       # ユニットテスト実行
make build      # bin/ow-custommatch-bot をビルド
make run        # bin/ow-custommatch-bot を実行（内部で make build 実行）
make build-win  # Windows 向け bin/ow-custommatch-bot.exe をビルド
make package-win # Windows 配布用 zip を生成
```

## ビルド出力先

当面、ビルド済みファイルはリポジトリ内の `bin/` に配置します（最終的には Release 配布を想定）。

## ビルド

```bash
make build
make build-win
```

## Windows 配布パッケージ（zip）

Windows 向け配布用 zip は以下で生成します。

```bash
make package-win
```

生成物:

- `dist/ow-custommatch-bot-win64.zip`

zip の同梱内容:

- `ow-custommatch-bot.exe`
- `.env`（`.env.example` をリネームして同梱）
- `使い方.html`
- `LICENSE`

補足:

- `zip` コマンドが必要です。
- `assets/windows/使い方.html` が存在しない場合、`make package-win` はエラー終了します。

## 実行時に必要なファイル（bin 配下）

- `.env`（`BOT_TOKEN` を設定）
- `ow-custommatch-bot.db`（初回起動時に自動生成されるため事前作成不要）

`.env.example` を `bin/.env` としてコピーしてください。
（配布用 zip では `make package-win` が `.env.example` を `.env` にリネームして同梱します）

補足:

- ランクマスタは `go:embed` でバイナリに埋め込まれています。
- `ow-custommatch-bot.db` は実行ファイルと同じディレクトリに作成されます。

## 環境変数（.env）

最低限必要:

- `BOT_TOKEN`: Discord Bot トークン

テストモード（任意）:

- `OW_CUSTOMMATCH_BOT_TEST_MODE=true`

テストモードを有効化すると、`/match` コマンドに `fill` オプション（boolean）が追加されます。
`fill=true` で募集開始した場合、ダミープレイヤーを20〜60人ランダム追加してテスト用の振り分けを行えます。

例（`bin/.env`）:

```dotenv
BOT_TOKEN=your_bot_token
OW_CUSTOMMATCH_BOT_TEST_MODE=true
```

補足:
- 判定は文字列一致のため、`true`（小文字）を設定してください。
- テストモードを無効化したい場合は、未設定にするか `true` 以外の値を設定してください。

## テスト

```bash
make test
```

## 配布素材（Windows）

Windows 配布用の説明ファイルは以下に配置します。

- `assets/windows/使い方.html`
