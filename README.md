# MatchyBot

OW のカスタムマッチ向けに、プレイヤー募集とチーム分けを行う Discord Bot（Go 実装）です。

## 事前準備

### Discord BOTを作成

[Discord Developer Portal](https://discord.com/developers/applications) で、Bot の作成・トークン発行・サーバー招待を済ませてください。

## 配布/実行方針

Docker は使用せず、**ビルド済み実行ファイル（exe）+ 設定ファイル**で運用します。

## 設定ファイル

`config.example.json` をコピーして `config.json` を作成し、値を設定してください。

```bash
cp config.example.json config.json
```

- `bot_token`: Discord Bot トークン
- `player_data_path`: プレイヤーデータ JSON の保存先（存在しない場合は自動作成）
- `rank_data_path`: ランクデータ JSON の読み込み元

## 実行方法（Go）

### 1) ビルド

```bash
go build -o matchybot ./cmd/matchybot
```

Windows 向け exe を作る場合:

```bash
GOOS=windows GOARCH=amd64 go build -o matchybot.exe ./cmd/matchybot
```

### 2) 起動

```bash
./matchybot
```

別パスの設定ファイルを使う場合:

```bash
./matchybot ./path/to/config.json
```

## 現在の実装範囲

- Discord セッション起動
- `/menu` コマンド登録と応答
- プレイヤーデータ JSON の読み書き
- ランクデータ読み込み
- スコアベースのチーム分けロジック
