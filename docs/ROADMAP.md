# ROADMAP

ow-custommatch-bot の現行ロードマップです。
詳細設計は `docs/plans/` 配下の各 plan を参照してください。

## 現在の前提

- `docs/plans/fixed/001`〜`024` は完了済みまたはクローズ済みです
- 今後の実装対象は `025`〜`028` です
- 実装は **1プランずつ** 進め、各 plan の「実施順序」「検証方法」に従って完了判定します

## 実行順

| 順番 | プラン | 概要 | 依存・理由 |
|---|---|---|---|
| 1 | [025](plans/025-match-restart-confirm.md) | `/match` 再実行時の確認 UI 追加 | 023 で残った UX 課題の解消を優先 |
| 2 | [026](plans/026-token-store-management.md) | TOKEN の確認・上書き・削除導線整備 | 024 完了後の運用上の不足を補う |
| 3 | [027](plans/027-launch-ui-polish.md) | 起動 UI の細部調整 | 022 実装後のユーザーコメント反映 |
| 4 | [028](plans/028-makefile-help-comments.md) | Makefile の日本語説明コメント追加 | 小規模で独立して実施可能 |

## 実施方針

### フェーズ1: `/match` 再実行 UX 改善

#### [025](plans/025-match-restart-confirm.md)

- 同一 channel で `/match` を再実行した際に確認 UI を表示する
- 「閉じて開始」「キャンセル」の分岐と権限制御を追加する
- Bot の回帰テストで再実行時の挙動を固定する

### フェーズ2: TOKEN 運用導線の整備

#### [026](plans/026-token-store-management.md)

- `BOT_TOKEN` の確認、上書き、削除を自己解決できる導線を整える
- Credential Manager 前提の運用手順を README とガイドへ反映する
- 必要なら削除用のストア操作も追加する

### フェーズ3: 起動 UI の仕上げ

#### [027](plans/027-launch-ui-polish.md)

- バナー枠内の情報配置を見直す
- パス表示項目を整理する
- Discord 接続成功後の案内文を追加する

### フェーズ4: 開発者向け補足

#### [028](plans/028-makefile-help-comments.md)

- `Makefile` の各ターゲットに短い日本語説明コメントを付ける
- 必要に応じて README の開発者向け説明を補う

## 完了済み・クローズ済みプラン

- `docs/plans/fixed/001`〜`013`: 実装済み
- `docs/plans/fixed/014`〜`016`: 不採用でクローズ
- `docs/plans/fixed/017`〜`024`: 実装済み

不採用理由や過去の実施内容は `docs/plans/fixed/` 配下の各ファイルを参照してください。
