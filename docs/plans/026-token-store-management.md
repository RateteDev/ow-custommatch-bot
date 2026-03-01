# 026-token-store-management

## Context

`BOT_TOKEN` は Windows Credential Manager に保存されるようになりましたが、
利用者向けに「確認」「上書き」「削除」をどう行うかの導線がありません。

初回設定だけでなく、トークン更新やトラブル時の再設定まで自己解決できるよう、
アプリ内導線と案内文書を整備します。

## 変更ファイル

| ファイル | 対応内容 |
|---------|---------|
| `cmd/ow-custommatch-bot/main.go` | トークン再設定や削除に関する CLI 導線を追加 |
| `cmd/ow-custommatch-bot/token_windows.go` | 必要であれば削除用の資格情報ストア操作を追加 |
| `cmd/ow-custommatch-bot/main_test.go` | 追加導線のテストを追加 |
| `README.md` | TOKEN の確認、上書き、削除方法を追記 |
| `assets/windows/使い方.html` | 利用者向け手順を追記 |

## 実施順序

1. どの操作をアプリ内で提供するか決める
2. 資格情報ストア操作を実装する
3. CLI とエラーメッセージの導線を整える
4. README と使い方ガイドを更新する

## 要件

### 要件1: 上書き導線

- 既存のトークンを再入力して更新できる手段を用意する

### 要件2: 削除導線

- 資格情報ストア上の `BOT_TOKEN` を削除できる手段を用意する

### 要件3: 利用者向け案内

- README とガイドに確認、上書き、削除の手順を明記する

## 検証方法

- `go test ./cmd/ow-custommatch-bot/...`
- Windows 実機でトークン再設定と削除を手動確認
