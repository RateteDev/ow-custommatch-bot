# ow-custommatch-bot

Overwatch のカスタムマッチ向けに、プレイヤー募集とチーム分けを行う Discord Bot（Go 実装）です。

## 前提（重要）

このプロジェクトは **Windows 向け exe 単体** で配布・運用する想定です。

- **利用者（運用者）**: Go 環境は不要です。GitHub Release から `ow-custommatch-bot.exe` を取得して起動するだけで使えます。
- **開発者（ソースを変更して再ビルドする人）**: 開発者向け手順を参照（[docs/DEVELOPER.md](docs/DEVELOPER.md)）

`BOT_TOKEN` は初回起動時に入力すると自動で保存されます。Windows では Credential Manager に保存され、SQLite データベースとログは `%LOCALAPPDATA%\ow-custommatch-bot\` 配下に保存されます。保存済みトークンを別の値に変えたい場合は、起動メニューの「トークンを上書きする」を使ってください。

## セットアップ（利用者向け）

### 1. Discord Bot の作成

[Discord Developer Portal](https://discord.com/developers/applications) で以下を実施してください。

1. アプリケーション作成
2. Bot 作成
3. Bot Token の発行
4. OAuth2 URL Generator で `bot` と `applications.commands` を有効化してサーバーに招待

> スラッシュコマンド（`/match` など）を使うため、`applications.commands` スコープが必要です。

### 2. exe の取得

GitHub Release から `ow-custommatch-bot.exe` をダウンロードしてください。配布物は exe 1 ファイルのみです。
リリースの起点となるバージョンは Git Tag です。正式版は付与された Tag 名がそのまま埋め込まれ、Tag がない開発ビルドは `dev-<shortsha>` 形式になります。

### 3. 初回起動

1. `ow-custommatch-bot.exe` を起動します。
2. 初回起動時に表示される案内に従って `BOT_TOKEN` を入力します。
3. 入力したトークンは Windows Credential Manager に保存され、次回以降は再入力不要です。
4. プレイヤー情報・VC 設定・ログは `%LOCALAPPDATA%\ow-custommatch-bot\` 配下に保存されます。
5. ランク定義は exe に埋め込まれています。

### 4. トークンを上書きしたいとき

1. `ow-custommatch-bot.exe` を起動します。
2. 起動メニューで `[3] トークンを上書きする` を選びます。
3. 新しい `BOT_TOKEN` を入力します。
4. 「保存済みトークンを更新しました」と表示されたら、続けて起動方法（通常運用 / 動作確認用）を選んで起動します。

保存先は Windows Credential Manager の `ow-custommatch-bot/BOT_TOKEN` です。トークン値そのものは画面に表示されません。

### 5. 動作確認

1. Bot がオンラインになっていることを確認
2. 対象サーバーの任意チャンネルで `/match` を実行
3. 募集パネルが表示されることを確認

## ライセンス通知

サードパーティー依存関係の通知一覧は [THIRD-PARTY-NOTICES.txt](THIRD-PARTY-NOTICES.txt) をご確認ください。通知ファイルはリポジトリルートで管理しており、GitHub Release asset には同梱していません。

## トラブルシュート

- **「BOT_TOKEN が保存されていません」と表示される**
  - `ow-custommatch-bot.exe` を起動し、画面の案内に従って初回のトークン入力を完了してください
- **トークンを別の値に変えたい**
  - 起動メニューで `[3] トークンを上書きする` を選び、新しい `BOT_TOKEN` を入力してください
- **「BOT_TOKEN の設定に失敗しました」と表示される**
  - Windows Credential Manager に保存できなかった可能性があります。PC を再起動してから再度入力してください
- **「failed to initialize bot」と表示される**
  - `%LOCALAPPDATA%\ow-custommatch-bot\` フォルダへの書き込み権限があるか確認してください
- **スラッシュコマンドが表示されない**
  - Bot 招待時に `applications.commands` スコープを付けたか確認してください
  - コマンドの反映には数分かかる場合があります。少し待ってから再確認してください

## 現在の実装範囲

- Discord セッション起動
- `/match` / `/register_rank` / `/my_rank` コマンド登録と応答
- プレイヤーデータ / VC 設定の SQLite 永続化
- ランクデータ読み込み（`go:embed`）
- スコアベースのチーム分けロジック
