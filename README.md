# MatchyBot

OW のカスタムマッチ向けに、プレイヤー募集とチーム分けを行う Discord Bot（Go 実装）です。

## 前提（重要）

このプロジェクトは **ビルド済み実行ファイル（exe）+ 設定ファイルで配布/運用** する想定です。

- **利用者（運用者）**: Go 環境は不要（配布された実行ファイルを起動）
- **開発者（ソースを変更して再ビルドする人）**: 開発者向け手順を参照（[docs/developer.md](docs/developer.md)）

また、`config.json`・`player_data.json`・`rank.json` は **実行ファイルと同じディレクトリ** に配置する前提です。

## セットアップ（利用者向け）

### 1. Discord Bot の作成

[Discord Developer Portal](https://discord.com/developers/applications) で以下を実施してください。

1. アプリケーション作成
2. Bot 作成
3. Bot Token の発行
4. OAuth2 URL Generator で `bot` と `applications.commands` を有効化してサーバーに招待

> `/menu` はスラッシュコマンドなので、`applications.commands` スコープが必要です。

### 2. 設定ファイル作成

`config.example.json` をコピーして `config.json` を作成します。

```bash
cp config.example.json config.json
```

`config.json` の各項目:

- `bot_token`: Discord Bot トークン
- `player_data_path`: プレイヤーデータ JSON の保存先（相対パスは実行ファイル基準）
- `rank_data_path`: ランクデータ JSON の読み込み元（相対パスは実行ファイル基準）

例（実行ファイルと同じディレクトリのファイルを使用）:

```json
{
  "bot_token": "YOUR_DISCORD_BOT_TOKEN",
  "player_data_path": "player_data.json",
  "rank_data_path": "rank.json"
}
```

### 3. 起動（配布された実行ファイルを使用）

デフォルトでは、実行ファイルと同じディレクトリにある `config.json` を読み込みます。

```bash
./matchybot
```

別パスの設定ファイルを使う場合:

```bash
./matchybot ./path/to/config.json
```

> ただし `player_data_path` / `rank_data_path` の相対パスは、設定ファイルの場所ではなく実行ファイルの場所を基準に解決されます。

### 4. 動作確認

1. Bot がオンラインになっていることを確認
2. 対象サーバーの任意チャンネルで `/menu` を実行
3. 「コマンドメニュー」Embed が返ってくることを確認

## トラブルシュート

- `failed to load config` が出る
  - `config.json` のパスが正しいか確認
  - JSON の文法エラーがないか確認
- `bot_token is required` が出る
  - `config.json` の `bot_token` が空になっていないか確認
- スラッシュコマンドが表示されない
  - Bot 招待時に `applications.commands` を付けたか確認
  - コマンド反映まで少し待って再確認

## 現在の実装範囲

- Discord セッション起動
- `/menu` コマンド登録と応答
- プレイヤーデータ JSON の読み書き
- ランクデータ読み込み
- スコアベースのチーム分けロジック
