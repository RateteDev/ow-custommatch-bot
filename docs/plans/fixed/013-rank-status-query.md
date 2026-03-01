# Context

ランク登録は 30 日で有効期限が切れるが、現在ユーザーが自分のランク登録状況（登録ランク・有効期限）を
確認する手段がない。

次の募集でエントリーしようとして初めて「ランクの再登録が必要」と言われるより、
事前に確認・更新できる方が体験が良い。

`/my_rank` スラッシュコマンドを追加し、登録ランク・登録日・有効期限をエフェメラルメッセージで返す。

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `internal/bot/bot.go` | `/my_rank` コマンド登録・ハンドラー追加 |

# 実施順序

依存関係なし。

# 要件1: `/my_rank` コマンドの登録

`registerCommands()` に追加する。

```go
myRankCmd := &discordgo.ApplicationCommand{
    Name:        "my_rank",
    Description: "登録済みのランク情報を確認します",
}
```

# 要件2: `handleMyRank` ハンドラー実装

```go
func (b *Bot) handleMyRank(s *discordgo.Session, i *discordgo.InteractionCreate) {
    userID, _ := interactionUser(i)
    player := b.players.GetByID(userID)

    if player == nil || player.HighestRank.Rank == "" {
        _ = b.respondEphemeralText(s, i, "ランクが登録されていません。エントリー時または /register_rank で登録できます。")
        return
    }

    rankLabel := player.HighestRank.Rank
    if player.HighestRank.Division != "" {
        rankLabel += " " + player.HighestRank.Division
    }

    expiredMsg := ""
    if b.isRankRegistrationExpired(player) {
        expiredMsg = "\n⚠️ 有効期限が切れています。次回エントリー時に再登録が必要です。"
    }

    description := fmt.Sprintf(
        "ランク: **%s**\n登録日: %s\n有効期限: %s%s",
        rankLabel,
        player.RankUpdatedAt[:10],
        expiryDate(player.RankUpdatedAt),
        expiredMsg,
    )

    // エフェメラル Embed で返す
}
```

# 要件3: `onInteractionCreate` にルーティング追加

```go
case "my_rank":
    b.handleMyRank(s, i)
```

# 検証方法

## ユニットテスト

- ランク未登録ユーザーへの応答メッセージ確認
- 登録済みユーザーへの正しいランク・期限表示確認
- 期限切れユーザーへの警告メッセージ確認

## 手動確認

1. ランク未登録の状態で `/my_rank` → 未登録メッセージが返ること
2. `/register_rank` でランク登録後、`/my_rank` で正しく表示されること
3. 他のユーザーには表示されない（エフェメラル）ことを確認

## 実装結果

- `internal/bot/bot.go`:
  - `registerCommands`: `/my_rank` コマンド登録追加
  - `onInteractionCreate`: ルーティング追加
  - `handleMyRank`: エフェメラルEmbed でランク情報・有効期限を表示
  - 日時パースエラー時のフォールバックメッセージ追加
- `go test ./...` 全件パス ✅
- `go build ./...` 成功 ✅

## 次期改善事項

- 使い方.html のコマンド一覧に `/my_rank` を追記する
