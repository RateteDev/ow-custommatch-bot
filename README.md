# MatchyBot

OW のカスタムマッチ向けに、プレイヤー募集とチーム分けを行う Discord Bot（Go 実装）です。

## 前提（重要）

このプロジェクトは **ビルド済み実行ファイル（exe）+ `.env` で配布/運用** する想定です。

- **利用者（運用者）**: Go 環境は不要（配布された実行ファイルを起動）
- **開発者（ソースを変更して再ビルドする人）**: 開発者向け手順を参照（[docs/developer.md](docs/developer.md)）

当面、リポジトリ内でビルドファイルを置く場合は `bin/` 配下を使用します（最終的には Release 配布を想定）。

また、`.env`・`player_data.json`・`rank.json` は **実行ファイルと同じディレクトリ** に配置する前提です。

## セットアップ（利用者向け）

### 1. Discord Bot の作成

[Discord Developer Portal](https://discord.com/developers/applications) で以下を実施してください。

1. アプリケーション作成
2. Bot 作成
3. Bot Token の発行
4. OAuth2 URL Generator で `bot` と `applications.commands` を有効化してサーバーに招待

> `/menu` はスラッシュコマンドなので、`applications.commands` スコープが必要です。

### 2. 設定ファイル作成（`.env`）

`.env.example` をコピーして、実行ファイルと同じディレクトリ（例: `bin/`）に `.env` を作成します。

```bash
cp .env.example bin/.env
```

`.env` の項目:

- `BOT_TOKEN`: Discord Bot トークン

例:

```dotenv
BOT_TOKEN=YOUR_DISCORD_BOT_TOKEN
```

### 3. データファイル配置

実行ファイルと同じディレクトリに以下のファイルを置きます。

- `rank.json`（初期ランク定義）
- `player_data.json`（起動時に存在しなければ自動生成）

`rank.json` の配置例:

```bash
cp data/rank.json bin/rank.json
```

### 4. 起動（配布された実行ファイルを使用）

```bash
./bin/matchybot
```

> 起動時に、`.env` / `player_data.json` / `rank.json` は実行ファイルと同じディレクトリから読み込みます。

### 5. 動作確認

1. Bot がオンラインになっていることを確認
2. 対象サーバーの任意チャンネルで `/menu` を実行
3. 「コマンドメニュー」Embed が返ってくることを確認

## トラブルシュート

- `failed to load env` が出る
  - 実行ファイルと同じディレクトリに `.env` があるか確認
- `BOT_TOKEN is required` が出る
  - `.env` の `BOT_TOKEN` が空でないか確認
- `failed to initialize bot` が出る
  - 実行ファイルと同じディレクトリに `rank.json` があるか確認
- スラッシュコマンドが表示されない
  - Bot 招待時に `applications.commands` を付けたか確認
  - コマンド反映まで少し待って再確認

## 現在の実装範囲

- Discord セッション起動
- `/menu` コマンド登録と応答
- プレイヤーデータ JSON の読み書き
- ランクデータ読み込み
- スコアベースのチーム分けロジック
