# Context

README.md に `/menu` コマンドへの言及（「Discord で手動テスト項目を実施する」欄、動作確認手順）があるが、
実際に Bot に登録されているコマンドは `/match` と `/register_rank` のみ。
`/menu` は過去の実装から削除されたか、未実装のまま記載が残っていると考えられる。

古い記述がユーザーを混乱させるため、README を現行の実装に合わせて修正する。

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `README.md` | `/menu` への言及を削除し、`/match` / `/register_rank` の説明に統一 |

# 実施順序

依存関係なし。単独で対応可能。

# 要件1: README の `/menu` 記述を削除・修正

- 「動作確認」手順の「`/menu` を実行」を「`/match` を実行」に変更
- 「現在の実装範囲」セクションを実際の実装（`/match`, `/register_rank`）に合わせて更新
- コマンド一覧を現行の 2 コマンドで明記する

# 検証方法

## 手動確認

- README 全文を通読し、`/menu` の記述が残っていないことを確認
- 記載されているコマンド名が `bot.go` の `registerCommands()` と一致することを確認

## 実装結果

- `README.md`: `/menu` への全言及を `/match` / `/register_rank` に修正
  - セットアップ手順の `applications.commands` 説明を一般化
  - 動作確認手順を `/match` 前提に更新
  - 「現在の実装範囲」を `/match` / `/register_rank` に更新
- 手動確認: `/menu` の残存なし ✅

## 次期改善事項

- なし
