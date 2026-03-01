# Context

現在の募集 Embed は「参加者（X人）」とだけ表示される。
チーム分けには 10 人以上必要だという仕様が Discord UI 上で全くわからないため、
人数が足りないのに振り分けボタンを押して「10人以上必要です」とエラーになるケースがある。

参加者数フィールドに「10人以上で振り分け可能」という情報を追加し、
10 人未満では振り分けボタンを視覚的に押しにくくする（無効化 or ラベル変更）。

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `internal/bot/bot.go` | `buildRecruitEmbed` / `buildRecruitComponents` の修正 |

# 実施順序

依存関係なし。

# 要件1: Embed フィールド名に最小人数を表示

`buildRecruitEmbed` 内のフィールド名を変更する。

```go
// 変更前
Name: fmt.Sprintf("参加者（%d人）", entryCount),

// 変更後（10人未満）
Name: fmt.Sprintf("参加者（%d人 / 振り分けには10人以上必要）", entryCount),

// 変更後（10人以上）
Name: fmt.Sprintf("参加者（%d人 / 振り分け可能✅）", entryCount),
```

# 要件2: 振り分けボタンを 10 人未満では Disabled にする

`buildRecruitComponents` に参加者数を引数として渡し、10 人未満では `assign` ボタンを `Disabled: true` にする。

```go
func (b *Bot) buildRecruitComponents(r *model.Recruitment, disabled bool) []discordgo.MessageComponent {
    canAssign := r != nil && len(r.Entries) >= 10
    // ...
    discordgo.Button{
        Label:    assignLabel,
        CustomID: "assign",
        Style:    discordgo.DangerButton,
        Disabled: disabled || !canAssign,
    },
```

# 検証方法

## ユニットテスト

- `bot_test.go` に `buildRecruitEmbed` / `buildRecruitComponents` のテストを追加
  - 9 人エントリー時: フィールド名に「10人以上必要」を含む / assign ボタンが Disabled
  - 10 人エントリー時: フィールド名に「振り分け可能」を含む / assign ボタンが Enabled

## 手動確認

- 9 人エントリー状態で Embed を確認し、フィールド名と振り分けボタンの状態を目視確認
- 10 人エントリー後、振り分けボタンが押せるようになることを確認
