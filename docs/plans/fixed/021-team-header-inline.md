# 021-team-header-inline

## Context

チーム振り分け結果の Discord embed で、各チームのヘッダーに平均スコアを含めていたため、
`Inline: true` の3列表示時に見出しが折り返され、見た目が崩れていました。

見出しは短く保ち、平均スコアは各列の本文先頭へ移すことで、横並びレイアウトを維持したまま
可読性を改善します。

## 変更ファイル

| ファイル | 対応内容 |
|---------|---------|
| `internal/bot/bot.go` | `buildAssignEmbed` 系の整形を見直し、`Field.Name` を短縮 |
| `internal/bot/bot_test.go` | embed の見出しと本文レイアウトに関する回帰テストを追加 |

## 実施順序

1. assign embed 生成処理を helper に整理
2. `Field.Name` を `チームA` 形式へ短縮
3. 平均スコアを `Field.Value` 先頭へ移動
4. テストを追加して表示崩れの再発を防止

## 要件

### 要件1: 見出しの短縮

- チーム field の `Name` は `チームA` のような短い表示にする
- `Inline: true` の3列表示は維持する

### 要件2: 平均スコアの表示位置変更

- 平均スコアは `Field.Value` の先頭行に配置する
- メンバー一覧や VC 招待リンクより前に表示する

### 要件3: テストの追加

- `Field.Name` に平均スコア文字列が含まれないこと
- `Field.Value` の先頭に平均スコアが入ること

## 検証方法

- `go test ./internal/bot/... -run TestBuildAssignEmbed`
- `/assign` 実行後に見出しが折り返されないことを手動確認

## 実装結果

- `internal/bot/bot.go`
  - チーム field 名を `チームA` のような短い表示へ変更
  - 平均スコアを `Value` 先頭へ移動
  - `buildAssignEmbed` などの helper を切り出し
- `internal/bot/bot_test.go`
  - チーム field 名に平均スコア文字列が入らないことを追加検証
  - `Value` の先頭に平均スコアが入ることを追加検証

## 動作確認結果

- `go test ./...` 全件パス
- `go build ./...` 成功
- `/assign` 実行後の embed で `チームA` などの見出しが折り返されず、平均スコアが各列先頭に表示されることを確認済み

## 次期改善事項

- チームごとの補足情報が今後増える場合は、列幅を圧迫しない配置ルールを先に決めておく
