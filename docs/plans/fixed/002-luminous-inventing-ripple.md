# 実装プラン: `/match` コマンド次期修正（修正1〜5）

## Context

`feature/match-command` ブランチにて実装・マージ済みの `/match` コマンドについて、
Discord 動作確認で判明した問題とフィードバックを元に修正する：

1. 振り分け後に Embed が閉じて毎回エントリーし直す手間が生じる
2. 人数不足時にもセッションが終了してしまう（+ 最低人数が 5 → 10 人に変更）
3. Discord 側に `/menu` コマンドのキャッシュが残る可能性がある
4. 少人数環境での振り分けテストができない（環境変数 ON 時のみ有効）
5. 中止時の Embed が視覚的にわかりにくい

---

## 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `internal/bot/bot.go` | 修正1〜5・テストモード全て |
| `internal/model/recruitment.go` | `MakeTeams` 最低人数を 10 人に変更 |
| `internal/model/recruitment_test.go` | 最低人数変更に伴うテスト更新・追加 |

---

## 修正1: 「締め切り」→「振り分け」ボタン

**対象**: `internal/bot/bot.go`

### 変更箇所A: `buildRecruitComponents`（447〜482行目）

```go
// Before
discordgo.Button{Label: "🔒 締め切り", CustomID: "close", Style: discordgo.DangerButton, Disabled: disabled}
// After
discordgo.Button{Label: "🎲 振り分け", CustomID: "assign", Style: discordgo.DangerButton, Disabled: disabled}
```

### 変更箇所B: `onInteractionCreate`（91〜106行目）

```go
// Before
case "close": b.handleClose(s, i)
// After
case "assign": b.handleAssign(s, i)
```

### 変更箇所C: `handleClose` → `handleAssign` にリネームして挙動変更

振り分け成功時の処理から以下を削除し、`IsOpen=true` を維持：
- `b.recruitment.IsOpen = false`
- `b.updateRecruitEmbed(s, true)` の呼び出し（ボタン無効化なし）

振り分け成功後も Embed は開いたまま、ボタンも有効なまま維持する。

---

## 修正2: 人数不足時も Embed を閉じない（最低 10 人）

5v5 形式のため、振り分けには最低 **10 人**が必要。

### `internal/model/recruitment.go` の変更

```go
// Before
func (r *Recruitment) MakeTeams(players []ScoredPlayer) [][]ScoredPlayer {
    if len(players) < 2 {
        return nil
    }
    target := len(players) / 5 * 5
    if target == 0 {
        return nil
    }

// After
func (r *Recruitment) MakeTeams(players []ScoredPlayer) [][]ScoredPlayer {
    if len(players) < 10 {  // 5v5 には最低 10 人必要
        return nil
    }
    target := len(players) / 5 * 5
```

### `internal/bot/bot.go` の `handleAssign` 変更

```go
// After（handleAssign）
if teams == nil {
    b.respondEphemeralText(s, i,
        fmt.Sprintf("チーム分けには10人以上必要です（現在 %d 人）", len(b.recruitment.Entries)))
    return  // IsOpen 変更・Embed 更新なしで終了
}
// 成功時のみチーム結果を投稿
```

### ユニットテスト更新（`recruitment_test.go`）

既存の `TestMakeTeams` の 1 人テストを **9 人でも nil** を返すよう更新：
```go
// 9 人では nil（10 人未満）
if teams := r.MakeTeams(ninePlayerSlice); teams != nil {
    t.Fatalf("expected nil for 9 players (< 10)")
}
```

---

## 修正3: `registerCommands` を `BulkOverwrite` 方式に変更

**背景**: `/menu` は Go 移行コミット（18e3267）でソースから削除済み。
ただし、`ApplicationCommandCreate` はコマンドを追加するだけで既存の古いコマンドを削除しない。
Discord API の `ApplicationCommandBulkOverwrite` を使えば、指定したコマンド一覧で **全コマンドを置換** できるため、特定コマンド名をソースに書かずに古いキャッシュを自動除去できる。

**対象**: `internal/bot/bot.go` の `registerCommands`

```go
func (b *Bot) registerCommands() error {
    appID := b.session.State.User.ID

    cmd := &discordgo.ApplicationCommand{
        Name:        "match",
        Description: "マッチングの募集を開始します",
    }
    if os.Getenv("OW_CUSTOMMATCH_BOT_TEST_MODE") == "true" {
        cmd.Options = []*discordgo.ApplicationCommandOption{ /* 修正4 参照 */ }
    }

    // BulkOverwrite でこのリスト以外の古いコマンドを自動削除
    _, err := b.session.ApplicationCommandBulkOverwrite(appID, "", []*discordgo.ApplicationCommand{cmd})
    return err
}
```

---

## 修正4: テストモード（環境変数 `OW_CUSTOMMATCH_BOT_TEST_MODE=true` 時のみ有効）

### 概要

| 項目 | 内容 |
|------|------|
| 有効条件 | 環境変数 `OW_CUSTOMMATCH_BOT_TEST_MODE=true` |
| `fill` オプション | 環境変数 OFF 時はコマンドに表示されない |
| ダミー人数 | `rand.Intn(41) + 20`（20〜60 人のランダム） |
| ダミーランク | bronze〜grandmaster をランダム付与（division も乱数） |
| データ永続化 | **しない**（メモリのみ）|

### `Bot` struct にフィールド追加

```go
type Bot struct {
    session              *discordgo.Session
    players              *model.PlayerDataManager
    recruitment          *model.Recruitment
    pendingRegistrations map[string]string
    testDummies          map[string]model.PlayerInfo  // テストモード用（メモリのみ）
}
```

`New()` 内で `testDummies: make(map[string]model.PlayerInfo)` を初期化。

### コマンド定義（`registerCommands` 内）

```go
if os.Getenv("OW_CUSTOMMATCH_BOT_TEST_MODE") == "true" {
    cmd.Options = []*discordgo.ApplicationCommandOption{
        {
            Type:        discordgo.ApplicationCommandOptionBoolean,
            Name:        "fill",
            Description: "ダミープレイヤーをランダム追加してテスト振り分けを行います（20〜60人）",
            Required:    false,
        },
    }
}
```

### `handleMatchStart` にダミー注入処理を追加

```go
// 新規マッチ開始時に testDummies をリセット
b.testDummies = make(map[string]model.PlayerInfo)

// fill オプション判定（OW_CUSTOMMATCH_BOT_TEST_MODE=true 時のみ options が存在）
fillMode := false
if os.Getenv("OW_CUSTOMMATCH_BOT_TEST_MODE") == "true" {
    for _, opt := range i.ApplicationCommandData().Options {
        if opt.Name == "fill" {
            fillMode = opt.BoolValue()
        }
    }
}

if fillMode {
    allRanks := []string{"bronze", "silver", "gold", "platinum", "diamond", "master", "grandmaster"}
    allDivisions := []string{"1", "2", "3", "4", "5"}
    count := rand.Intn(41) + 20 // 20〜60 人
    for n := 0; n < count; n++ {
        dummyID := fmt.Sprintf("dummy-%d", n+1)
        r := allRanks[rand.Intn(len(allRanks))]
        div := allDivisions[rand.Intn(len(allDivisions))]
        info := model.PlayerInfo{
            ID:          dummyID,
            Name:        fmt.Sprintf("ダミー%d", n+1),
            HighestRank: model.Rank{Rank: r, Division: div},
        }
        b.testDummies[dummyID] = info
        b.recruitment.Entries = append(b.recruitment.Entries, model.Entry{
            UserID: dummyID,
            Name:   fmt.Sprintf("ダミー%d", n+1),
        })
    }
}
```

### `handleAssign` でダミー参照

`scoredPlayers` 構築ループ内で `b.players.GetByID` の前に `b.testDummies` を参照：

```go
for _, e := range b.recruitment.Entries {
    var info *model.PlayerInfo
    if dummy, ok := b.testDummies[e.UserID]; ok {
        info = &dummy
    } else {
        info = b.players.GetByID(e.UserID)
    }
    // ...
}
```

チーム結果投稿時にダミーが含まれていれば末尾に「（テストモード）」を付記。

### Embed の参加者表示

`recruitParticipantList` 内で `dummy-` prefix のユーザーは mention 形式でなく名前で表示：

```go
for _, e := range b.recruitment.Entries {
    if strings.HasPrefix(e.UserID, "dummy-") {
        users = append(users, e.Name)
    } else {
        users = append(users, "<@"+e.UserID+">")
    }
}
```

---

## 修正5: 中止時の Embed を視覚的に変更

**対象**: `internal/bot/bot.go`

### `buildRecruitEmbed` の色パラメータ対応

```go
// シグネチャ変更（color を追加）
func (b *Bot) buildRecruitEmbed(title string, color int) *discordgo.MessageEmbed {
    ...
    return &discordgo.MessageEmbed{
        Title:       title,
        Description: description,
        Color:       color,
        ...
    }
}
```

通常時は `0x2ECC71`（緑）、中止時は `0xE74C3C`（赤）を渡す。

### `updateRecruitTitle` を `updateClosedEmbed` にリネーム

```go
func (b *Bot) updateClosedEmbed(s *discordgo.Session, title string) error {
    embed := b.buildRecruitEmbed(title, 0xE74C3C) // 赤
    components := b.buildRecruitComponents(true)
    ...
}
```

`handleCancel` から `b.updateClosedEmbed(s, "🚫 募集は中止されました")` を呼ぶ。

既存の `updateRecruitEmbed` は `0x2ECC71`（緑）を引き続き使用：
```go
func (b *Bot) updateRecruitEmbed(s *discordgo.Session, disabled bool) error {
    embed := b.buildRecruitEmbed("🎮 マッチング募集", 0x2ECC71)
    ...
}
```

---

## 実装順序

1. 修正3（`registerCommands` の `/menu` 削除）← 副作用なし、最初に実施
2. 修正1（ボタン名・CustomID・ハンドラー名変更）
3. 修正2（`MakeTeams` 最低 10 人 + `handleAssign` の ephemeral 返却）
4. 修正5（中止 Embed の色変更）
5. 修正4（テストモード・fill オプション）

---

## 実施記録（2026-02-25）

### 実装担当

- コーディング: codex エージェント（gpt-5.3-codex）
- レビュー・追加修正: Claude Code

### 実施内容

修正3→1→2→5→4 の順で実装完了。`go test ./...` は全件パス。

#### codex 実装時のプランとの差異（レビューで修正済み）

| 差異 | 内容 | 対応 |
|------|------|------|
| `handleAssign` 成功時に `InteractionRespond` なし | Discord は全インタラクションに 3 秒以内の応答を要求するため、チーム投稿後にアプリ未応答エラーが発生する | `InteractionResponseDeferredMessageUpdate` でサイレント確認応答を追加 |
| ダミー名が英語 | `"Dummy01"` 形式（プランでは `"ダミー1"` 指定） | `"ダミー%d"` 形式に修正 |
| ダミーランク生成がRankDataから動的取得 | プランでは固定リスト（bronze〜grandmaster）を指定していたが、codex は `b.testRankPool()` で RankData から動的取得する方式を選択 | 動作上の問題なしとしてそのまま採用 |

### 発生した問題

#### 🚨 問題1: ビルドコマンドの作業ディレクトリ誤り

**現象**: Discord 上で以下の 3 つが観測された
1. 振り分けボタン押下時に「チーム分け可能な人数が不足しています（5人単位で編成します）。」が表示される（旧メッセージ）
2. 中止ボタン押下後も Embed の色が変わらない
3. `/match fill:True` 実行後もダミーメンバーが追加されない

**原因**: ビルドコマンドを絶対パス指定せずに実行したため、古いバイナリ（`09:48` タイムスタンプ）のまま Bot を起動していた。ソースの変更は正しく行われており、テストも全件通過していたが、バイナリに反映されていなかった。

```bash
# NG（作業ディレクトリが /home/user/workspace/repos だったため無効）
go build -o bin/ow-custommatch-bot ./cmd/ow-custommatch-bot/

# OK（絶対パスで指定）
go build -o /home/user/workspace/repos/ow-custommatch-bot/bin/ow-custommatch-bot /home/user/workspace/repos/ow-custommatch-bot/cmd/ow-custommatch-bot/
```

**対応**: 正しいパスでリビルド済み（`11:41` タイムスタンプ）。ただし Discord 側の再確認は未実施。

### 現在のステータス

| 項目 | 状態 |
|------|------|
| ソース変更 | ✅ 完了 |
| ユニットテスト | ✅ 全件パス |
| バイナリリビルド | ✅ 完了（`bin/ow-custommatch-bot` 11:41 更新） |
| Discord 動作確認 | ⬜ 未完了（リビルド後の再確認が必要） |

---

## 検証方法

### ユニットテスト

```bash
go test ./...
```

更新・追加するテスト：
- `TestMakeTeams`：9 人でも nil を返すことを確認するケースを追加

### 手動テスト手順

| # | 操作 | 期待結果 |
|---|------|---------|
| 1 | Bot 起動（通常） | `/menu` が Discord コマンドリストに表示されない |
| 2 | Bot 起動（通常） | `/match` の `fill` オプションが表示されない |
| 3 | `/match` 実行 | Embed（緑）+ ボタン（「🎲 振り分け」「🚫 中止」）が表示 |
| 4 | 発案者が「振り分け」（9人以下） | ephemeral で「10人以上必要です（現在 N 人）」、Embed は変化なし |
| 5 | 10人エントリー後「振り分け」 | チーム結果が投稿される。Embed・ボタンは閉じない |
| 6 | 再度「振り分け」 | 同じメンバーで再振り分け可能 |
| 7 | 発案者が「中止」 | Embed が**赤色**で「🚫 募集は中止されました」、ボタン無効化 |
| 8 | `OW_CUSTOMMATCH_BOT_TEST_MODE=true` で Bot 起動 | `/match` に `fill` オプションが表示される |
| 9 | `/match fill:True` 実行 | Embed にダミー参加者（20〜60人）が名前で表示される |
| 10 | fill モードで「振り分け」 | チーム結果が「（テストモード）」付きで投稿 |
