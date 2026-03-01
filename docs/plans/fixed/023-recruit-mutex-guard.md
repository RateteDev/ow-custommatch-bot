# 023-recruit-mutex-guard

## Context

同一 guild 内の別 channel で募集を開始した際に干渉する可能性と、
並行実行時に募集状態 map が安全に扱われていない懸念がありました。

募集状態の参照と更新を排他制御し、同一 channel の二重開始を原子的に防止します。

## 変更ファイル

| ファイル | 対応内容 |
|---------|---------|
| `internal/bot/bot.go` | `recruitments` 用の mutex とアクセサ関数を追加 |
| `internal/bot/bot_test.go` | 同一 guild 別 channel、同一 channel、並行開始の回帰テストを追加 |

## 実施順序

1. `Bot` に `sync.RWMutex` を追加
2. 募集状態アクセスを helper に集約
3. 開始処理を原子的にする
4. race を含む回帰テストを追加

## 要件

### 要件1: 募集状態の排他制御

- `recruitments` 参照を mutex で保護する
- `getRecruitment` / `setRecruitment` / `deleteRecruitment` / `startRecruitment` を追加する

### 要件2: 同一 channel の二重開始防止

- `/match` 開始時に既存の open な募集があれば、新規開始しない
- この判定と登録を原子的に扱う

### 要件3: 回帰テスト

- 同一 guild の別 channel で募集状態が独立すること
- 同一 channel の二重開始を防ぐこと
- 並行開始時に 1 件だけ開始されること

## 検証方法

- `go test ./internal/bot/...`
- `go test -race ./internal/bot/...`

## 実装結果

- `internal/bot/bot.go`
  - `recruitments` 参照に `sync.RWMutex` を追加
  - `getRecruitment` / `setRecruitment` / `deleteRecruitment` / `startRecruitment` を追加
  - `/match` 開始時の二重開始防止を原子的に処理するよう変更
- `internal/bot/bot_test.go`
  - 同一 guild 内の別 channel で募集状態が独立することを追加検証
  - 同一 channel の二重開始を防ぐことを追加検証
  - 並行開始時に 1 件だけ開始されることを追加検証

## 動作確認結果

- `go test ./...` 全件パス
- `go build ./...` 成功
- `go test -race ./internal/bot/...` 成功
- `/match` を同一 guild の別 channel で同時に開始しても互いに干渉しないことを確認済み

## ユーザー確認コメント

- 同じ channel で `/match` を連続実行したとき、単に防止するだけでなく、
  既存募集を閉じるかキャンセルするかのエフェメラルメッセージを出す想定だったのではないか

## 次期改善事項

- 同一 channel 再実行時の UX を確認ダイアログ方式へ改善するか検討する
- `pendingRegistrations` や `testDummies` の排他方針も必要に応じて再評価する
