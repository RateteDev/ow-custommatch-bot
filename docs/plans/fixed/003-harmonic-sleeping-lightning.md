# 実装プラン: 修正6〜9（ephemeral 削除・ファイルログ・振り分け Embed・複数募集）

## Context

現在の実装で確認された以下の課題を解消する：

1. エントリー成功・取り消し成功・中止成功の ephemeral メッセージが不要（Embed 更新で視覚的に確認できる）
2. ログがプロセスの標準出力のみで永続化されない
3. 振り分け結果がプレーンテキストで視覚的にわかりにくい
4. Bot 全体で1つしか募集を管理できず、複数チャンネルで同時募集できない

---

## 変更ファイル

| ファイル | 対象要件 |
|---------|---------|
| `cmd/ow-custommatch-bot/main.go` | 要件2（ファイルログ） |
| `internal/bot/bot.go` | 要件1・3・4（全て） |

---

## 実施順序

依存関係とレビューのしやすさから以下の順で実施：

```
要件2 → 要件1 → 要件3 → 要件4
```

---

## 要件2: ファイルログ機構（main.go）

### 変更内容

`cmd/ow-custommatch-bot/main.go` に `setupLogger(exeDir string) (io.Closer, error)` 関数を追加。
`main()` 内の `executableDir()` 解決直後に呼び出す。

**ロジック:**
1. `filepath.Join(exeDir, ".logs")` ディレクトリを `os.MkdirAll` で作成（冪等）
2. `time.Now().Format("2006-01-02T15-04-05") + ".log"` でファイルを `os.O_CREATE|os.O_WRONLY|os.O_APPEND` で開く
3. `log.SetOutput(io.MultiWriter(os.Stdout, f))` で標準出力とファイルに同時出力

**追加 import:** `"io"`, `"time"`

**失敗時の挙動:** `log.Fatalf` で起動失敗（設定不備として扱う）

**補足:** `.gitignore` に `.logs/` を追記する

---

## 要件1: 成功時 ephemeral の削除（bot.go）

### 削除する ephemeral の一覧

| 行（現在） | 関数 | 内容 |
|------------|------|------|
| 199 | `handleEntry` | `"✅ エントリーしました！"` |
| 325 | `handleCancelEntry` | `"エントリーを取り消しました"` |
| 418 | `handleCancel` | `"募集を中止しました"` |

### 変更方針

成功時の `respondEphemeralText` 呼び出しを `InteractionResponseDeferredMessageUpdate` によるサイレント確認応答に置き換える（`handleAssign` と同じパターン）。

ヘルパーメソッド `ackInteraction` を追加して各ハンドラーで再利用する：

```go
func (b *Bot) ackInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) error {
    return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseDeferredMessageUpdate,
    })
}
```

**注意:** `handleEntry` の「ランク未登録フロー」（`respondRankRegistrationPrompt` を呼ぶパス）は引き続き ephemeral を返す。そのパスは変更しない。

---

## 要件3: 振り分け結果を Embed で表示（bot.go）

### 変更対象

`handleAssign`（369〜400行付近）の `s.ChannelMessageSend` による送信部分を `s.ChannelMessageSendEmbed` に置き換える。

### Embed 設計

```
Title:  "🎲 チーム振り分け結果"
Color:  0x3498DB（青）
Fields: チームごとに1 Field
  - Name:   "チームA" / "チームB" / ...
  - Value:  メンバーのメンション or 名前（"\n" 区切り）
  - Inline: true（横並び、最大3列）
Footer: testModeResult == true の場合のみ "テストモード"
```

**注意:** `Field.Value` が空文字列だと Discord API エラー（400）になるため、メンバーゼロの場合は `"（なし）"` を入れる。

---

## 要件4: 複数募集対応（bot.go 全体リファクタリング）

### キー戦略

**チャンネルID をキーとする**（1チャンネル = 1募集の制約）

- 全ハンドラーで `i.ChannelID` が常に利用可能
- `handleMatchStart` 実行時点でキーが確定する
- 別チャンネルでは独立して募集を進行できる

### Bot struct の変更

```go
// Before
type Bot struct {
    session              *discordgo.Session
    players              *model.PlayerDataManager
    recruitment          *model.Recruitment
    pendingRegistrations map[string]string
    testDummies          map[string]model.PlayerInfo
}

// After
type pendingRegEntry struct {
    rank      string
    channelID string
}

type Bot struct {
    session              *discordgo.Session
    players              *model.PlayerDataManager
    rankData             model.RankDataFile                    // NewRecruitment 用にキャッシュ
    recruitments         map[string]*model.Recruitment        // channelID -> *Recruitment
    pendingRegistrations map[string]pendingRegEntry           // userID -> {rank, channelID}
    testDummies          map[string]map[string]model.PlayerInfo // channelID -> dummyID -> PlayerInfo
}
```

### New() の変更

`model.NewRecruitment(ranks)` の呼び出しを削除し、`rankData` フィールドに保存。
`recruitments`・`testDummies` を `make` で初期化。

### handleMatchStart の変更

```go
if r, ok := b.recruitments[channelID]; ok && r.IsOpen {
    // エラー ephemeral: "このチャンネルでは既に募集が開始されています"
    return
}
r := model.NewRecruitment(b.rankData)
b.recruitments[channelID] = r
b.testDummies[channelID] = make(map[string]model.PlayerInfo)
```

### 各ハンドラーでの取得パターン

```go
channelID := i.ChannelID
r, ok := b.recruitments[channelID]
if !ok || !r.IsOpen {
    // エラー ephemeral
    return
}
```

### ランク登録フローの変更

- `handleRankSelect`: `b.pendingRegistrations[userID] = pendingRegEntry{rank: selectedRank, channelID: i.ChannelID}`
- `handleDivisionSelect`: `entry.channelID` から `b.recruitments[entry.channelID]` で対象募集を取得
- `handleDivisionSelect` 冒頭で `r.IsOpen` を確認（ランク登録待ちの間に募集が閉じた場合の対処）

### Embed・Component ビルダーのシグネチャ変更

`b.recruitment` を直接参照していた関数を `r *model.Recruitment` 引数受け取りに変更：

| 現在 | 変更後 |
|------|--------|
| `(b *Bot) updateRecruitEmbed(s, disabled)` | `(b *Bot) updateRecruitEmbed(s, r, disabled)` |
| `(b *Bot) updateClosedEmbed(s, title)` | `(b *Bot) updateClosedEmbed(s, r, title)` |
| `(b *Bot) buildRecruitEmbed(title, color)` | `buildRecruitEmbed(r, title, color)`（レシーバ不要） |
| `(b *Bot) recruitParticipantList()` | `recruitParticipantList(r)`（レシーバ不要） |

### testDummies の変更

- `injectTestDummies()` → `injectTestDummies(channelID string, r *model.Recruitment)` に変更
- `handleAssign` 内: `b.testDummies[e.UserID]` → `b.testDummies[i.ChannelID][e.UserID]`
- `testRankPool()` → `testRankPool(r *model.Recruitment)` に変更

### 変更完了後の確認

`b.recruitment`（単数形）というシンボルが `bot.go` に残っていないことを確認する。

---

## 検証方法

### ユニットテスト

```bash
go test ./...
```

要件4のリファクタリング後、既存の `recruitment_test.go` が全件パスすることを確認。

### 手動テスト

| # | 操作 | 期待結果 |
|---|------|---------|
| 1 | Bot 起動 | `bin/.logs/YYYY-MM-DDTHH-MM-SS.log` が生成され、起動ログが書き込まれる |
| 2 | 「エントリー」ボタン押下 | ephemeral 非表示・Embed の参加者リストが更新される |
| 3 | 「取り消し」ボタン押下 | ephemeral 非表示・Embed の参加者リストが更新される |
| 4 | 「中止」ボタン押下 | ephemeral 非表示・Embed が赤色になる |
| 5 | `fill:true` で「振り分け」押下 | チーム振り分け結果が Embed（青）でチームごとに Field 表示される |
| 6 | テストモードの振り分け | Footer に「テストモード」が表示される |
| 7 | チャンネルA で `/match` 後、チャンネルB でも `/match` | 両方独立して Embed が表示される |
| 8 | チャンネルA でエントリー | チャンネルA の Embed のみが更新される |
| 9 | 開催中のチャンネルA で再度 `/match` | 「既に募集が開始されています」が返る |
| 10 | チャンネルA で「中止」後に再度 `/match` | 新規募集が開始できる |

---

## 実装結果

2026-02-25 に実装・動作確認済み。全手動テスト項目をパス。

### 付随修正
- `internal/model/recruitment.go`: `MakeTeams` の最小人数を 2 → 10 に変更（5人チームが最低2チーム必要なため）
- `internal/model/recruitment_test.go`: 上記に合わせてテストを修正

### 確認済みコマンド
```bash
go build ./... && go test ./...  # 全件パス
```

---

## 次期改善事項

### 修正10: 中止後のボタン削除

中止時に Embed は赤色に更新・ボタンは無効化されるが、パッと見で押してしまいそうになる。
無効化ではなくボタン自体を削除（Components を空配列）にしたい。

対象: `handleCancel` → `updateClosedEmbed` のコンポーネント部分。

### 修正11: チーム振り分け結果 Embed の改善

現状の Embed（タイトル・チームごとの Field・テストモード Footer）で課題あり。
具体的な改善内容は次セッションで対話しながら詰める。

### 修正12: 振り分け後の VC 誘導（検討中）

チーム振り分け後に各チームの VC チャンネルへの参加リンク、または強制移動ボタンを表示すると便利かもしれない。

以下の点について次セッションで相談予定：
- VC チャンネルを事前に作成しておく運用 vs 振り分け時に一時作成してクリアする運用
- 強制移動の場合 `GUILD_VOICE_STATES` Intent と Move Members 権限が必要になる点
- 参加リンク（招待 URL）方式の場合の有効期限・最大使用回数の扱い
