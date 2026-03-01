# OW-CustomMtch-BOT

Overwatch のカスタムマッチ向けに、プレイヤー募集とチーム分けを行う Discord Bot です。
<img src="assets/icon.jpeg" alt="bot-icon" width="120" />

## 前提

このプロジェクトは **Windows 向け exe 単体** で配布・運用する想定です。

- **利用者（運用者）**: Go 環境は不要です。GitHub Release から `ow-custommatch-bot.exe` を取得して起動するだけで使えます。
- **開発者（ソースを変更して再ビルドする人）**: 開発者向け手順を参照（[docs/DEVELOPER.md](docs/DEVELOPER.md)）

`BOT_TOKEN` は初回起動時に入力すると自動で保存されます。
Windows の Credential Manager に保存され、SQLite データベースとログは `%LOCALAPPDATA%\ow-custommatch-bot\` 配下に保存されます。
保存済みトークンを別の値に変えたい場合は、起動メニューの「トークンを上書きする」を使ってください。

## セットアップ・使用方法

**使い方ページ**を見てください。
https://ratetedev.github.io/ow-custommatch-bot/

## ライセンス通知

サードパーティー依存関係の通知一覧は [THIRD-PARTY-NOTICES.txt](THIRD-PARTY-NOTICES.txt) をご確認ください。
通知ファイルはリポジトリルートで管理しており、GitHub Release asset には同梱していません。

