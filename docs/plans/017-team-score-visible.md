# Context

チーム振り分け結果では各チームのメンバーと VC 招待リンクのみが表示される。
バランス調整の根拠（ランクスコア）がユーザーに全く見えないため、
「なぜこの振り分けになったのか」が不透明である。

各チームのフィールドタイトルに平均スコアを付記し、
チーム間のバランスを数値で確認できるようにする。

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `internal/bot/bot.go` | `handleAssign` 内のフィールド名生成部分を修正 |

# 実施順序

依存関係なし。

# 要件1: チームフィールド名に平均スコアを付記

`handleAssign` 内で `teams` を処理する際、チームごとの平均スコアを算出してフィールド名に追加する。

```go
// 変更前
fields = append(fields, &discordgo.MessageEmbedField{
    Name:   "チーム" + teamLabel(idx),
    Value:  value,
    Inline: true,
})

// 変更後
avgScore := teamAverageScore(team)
fields = append(fields, &discordgo.MessageEmbedField{
    Name:   fmt.Sprintf("チーム%s（平均スコア: %.0f）", teamLabel(idx), avgScore),
    Value:  value,
    Inline: true,
})
```

```go
func teamAverageScore(team []model.ScoredPlayer) float64 {
    if len(team) == 0 {
        return 0
    }
    total := 0.0
    for _, p := range team {
        total += p.Score
    }
    return total / float64(len(team))
}
```

# 要件2: スコアの意味をフッターで説明（任意）

Embed フッターに「スコアはランクに基づく内部値です」などの補足を追加し、
ユーザーが数値の意味を誤解しないようにする。

# 検証方法

## ユニットテスト

- `teamAverageScore` のユニットテスト
  - 空チーム → 0
  - [3000, 2000] → 2500

## 手動確認

- テストモード（fill=true）で振り分けを実行し、各チームのフィールド名に平均スコアが表示されることを確認
- チーム間のスコア差が小さいことを目視で確認
