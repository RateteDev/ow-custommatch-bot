# 実装プラン: `/match` コマンドによる募集システム

---

## 実施済み作業結果（2026-02-25）

ブランチ `feature/match-command` にて以下を実装・マージ済み。

### 実装内容

| ファイル | 変更内容 |
|---------|---------|
| `internal/model/recruitment.go` | `Recruitment` 構造体に `OrganizerID`, `MessageID`, `ChannelID`, `IsOpen` フィールド追加、`RemoveEntry` メソッド追加 |
| `internal/bot/bot.go` | `/menu` 削除、`/match` + 全ボタン・セレクトメニューハンドラー実装、`Bot` 構造体に `pendingRegistrations` 追加 |
| `internal/model/recruitment_test.go` | `RemoveEntry` のユニットテスト（存在するユーザー／存在しないユーザー）追加 |

### Discord 動作確認結果

- `/match` 実行 → Embed + ボタン4つの表示: **OK**
- ランク未登録ユーザーでエントリー → ランク・ディビジョン選択フロー: **OK**
- ランク登録済みユーザーでエントリー → 参加者リスト更新: **OK**
- 重複エントリー → ephemeral エラー: **OK**
- 取り消し: **OK**
- 発案者以外が締め切り/中止 → ephemeral エラー: **OK**
- 中止 → Embed 更新・ボタン無効化: **OK**

### 確認された問題・改善点

1. **「締め切り」ボタンの挙動** - 振り分け後に Embed が閉じられるため連続セッションで毎回エントリーし直す手間がある
2. **人数不足時の Embed クローズ** - チーム分け失敗時もボタンが Disabled になりセッションが終了してしまう
3. **少人数でのテストが困難** - チーム分けは5人単位のため動作確認できる場面が限られる
4. **`/menu` コマンドの残留** - Discord 側コマンドキャッシュに `/menu` が残っている場合は手動削除が必要

---

## 次期修正・実装事項

### 修正1: 「締め切り」→「振り分け」ボタンへの変更

**背景**: 振り分け後も同じエントリーメンバーで再度チーム分けしたいケースがある。「締め切り」は `IsOpen=false` にしてしまうため連続利用には不向き。

**変更内容**:
- ボタン名を `🔒 締め切り` → `🎲 振り分け` に変更（CustomID: `assign`）
- `handleClose` → `handleAssign` にリネーム
- 振り分け実行後も **`IsOpen=true` を維持**し、Embed・ボタンは閉じない
- エントリーはそのまま保持（次の振り分けでも同じメンバーで実行できる）
- 「中止」ボタンのみが募集を終了する手段となる

**変更ファイル**: `internal/bot/bot.go`

---

### 修正2: 人数不足時も Embed を閉じない

**背景**: `MakeTeams` が `nil` を返した場合（5人未満）、現状は Embed のボタンが Disabled になりセッションが終了する。

**変更内容**:
- `handleAssign`（旧 `handleClose`）内で `teams == nil` のとき:
  - `IsOpen` を変更しない（`true` のまま）
  - Embed のボタンを Disabled にしない
  - ephemeral で「チーム分けには5人以上必要です（現在 N 人）」と返す
- チーム分け成功時のみ結果を投稿する

**変更ファイル**: `internal/bot/bot.go`

---

### 修正3: `/menu` コマンドの完全削除

**背景**: コードからは削除済みだが、Discord API 側にコマンドキャッシュが残る場合がある。

**変更内容**:
- 起動時またはコマンド登録時に、既存の全コマンドを取得して `/menu` が残っていれば `ApplicationCommandDelete` で削除する処理を追加する

**変更ファイル**: `internal/bot/bot.go`（`registerCommands` 内）

---

### 検討4: テストモードの実装

**背景**: チーム分けは5人単位のため、少人数環境（開発・テスト時）では振り分け動作を確認できない。

**案A: `/match test` サブコマンド**
- `test` オプション付きで起動した場合、チーム分けの最小人数を2人に緩和するモードを使用

**案B: `--test-mode` 起動フラグ**
- Bot 起動時に `--test-mode` フラグを渡すと `MakeTeams` の閾値を変更

**案C: ダミーエントリー注入**
- `/match` コマンドに `fill` オプションを追加し、ダミープレイヤー（ランクはランダム）を5人単位で自動追加する

**推奨**: 案Cが最もリスクが低く、本番コードへの影響が限定的。`fill` オプション付きの場合のみダミーを注入し、振り分け結果のメッセージにも「（テストモード）」と明記する。

---

## Context

現在の `/menu` コマンドは Embed を表示するだけで、実際の募集・参加・チーム分け機能が UI として提供されていない。
`internal/model/recruitment.go` にはロジックが実装済みだが、Discord の UI（ボタン・セレクトメニュー・インタラクション）と繋がっていない。

このプランでは `/menu` を廃止し、`/match` コマンドを起点とした完全な募集フローと、エントリー時のランク登録フローを実装する。

---

## 実装スコープ

| 機能 | 詳細 |
|------|------|
| `/match` スラッシュコマンド | 募集開始。Embed + ボタン4つを投稿 |
| 「エントリー」ボタン | 全員が押せる。ランク未登録なら登録フローへ |
| 「取り消し」ボタン | エントリー済みの人が自分のエントリーをキャンセル |
| 「締め切り」ボタン | 発案者のみ有効。チーム分けを実行して結果を投稿 |
| 「中止」ボタン | 発案者のみ有効。募集を中止してEmbedを更新 |
| ランク登録フロー | ランク未登録ユーザーに ephemeral でセレクトメニューを送信。完了後自動エントリー |
| `/menu` 削除 | コマンド登録・ハンドラーを削除 |

---

## データ構造の変更

### `internal/model/recruitment.go`

`Recruitment` 構造体にフィールドを追加：

```go
type Recruitment struct {
    Entries     []Entry
    RankData    RankDataFile
    OrganizerID string // 発案者の Discord UserID
    MessageID   string // Discord メッセージID（Embed 更新用）
    ChannelID   string // チャンネルID
    IsOpen      bool   // 募集中かどうか
}
```

追加メソッド：

```go
// エントリーを取り消す（成功したら true）
func (r *Recruitment) RemoveEntry(userID string) bool
```

---

## bot.go の変更

### Bot 構造体に追加

```go
type Bot struct {
    session              *discordgo.Session
    players              *model.PlayerDataManager
    recruitment          *model.Recruitment
    pendingRegistrations map[string]string // userID -> 選択中のランク（一時状態）
}
```

### コマンド登録（`registerCommands`）

- `/menu` を削除
- `/match` を新規登録

```go
{Name: "match", Description: "マッチングの募集を開始します"}
```

### インタラクションハンドラー（`onInteractionCreate`）

```go
switch i.Type {
case discordgo.InteractionApplicationCommand:
    switch i.ApplicationCommandData().Name {
    case "match": b.handleMatchStart(s, i)
    }
case discordgo.InteractionMessageComponent:
    switch i.MessageComponentData().CustomID {
    case "entry":          b.handleEntry(s, i)
    case "cancel_entry":   b.handleCancelEntry(s, i)
    case "close":          b.handleClose(s, i)
    case "cancel":         b.handleCancel(s, i)
    case "rank_select":    b.handleRankSelect(s, i)
    case "division_select": b.handleDivisionSelect(s, i)
    }
}
```

---

## 各ハンドラーの処理

### `/match` ハンドラー（`handleMatchStart`）

1. `IsOpen=true` なら ephemeral でエラー
2. `recruitment` フィールドを初期化（`OrganizerID`・`ChannelID`・`IsOpen=true` セット）
3. 募集 Embed + ボタン4つを送信
4. 応答からメッセージIDを取得して `MessageID` に保存

**Embed 内容:**
```
タイトル: 🎮 マッチング募集
説明:  <@発案者> が募集を開始しました
フィールド - 参加者（0人）: （なし）
```

**ボタン（1行目）:**
- `[✅ エントリー]` (CustomID: `entry`, スタイル: Success)
- `[❌ 取り消し]` (CustomID: `cancel_entry`, スタイル: Secondary)

**ボタン（2行目）:**
- `[🔒 締め切り]` (CustomID: `close`, スタイル: Danger)
- `[🚫 中止]` (CustomID: `cancel`, スタイル: Secondary)

### エントリーハンドラー（`handleEntry`）

1. `IsOpen` チェック → 閉じていたら ephemeral でエラー
2. `AddEntry` → false（重複）なら ephemeral で「既にエントリー済みです」
3. ランク未登録チェック: `b.players.GetByID(userID)` が nil または `HighestRank.Rank == ""`
   → **ランク登録フローへ（後述）**
4. 登録済みならそのまま参加者 Embed を更新
5. ephemeral で「エントリーしました」

### ランク登録フロー

#### 1. ランクセレクトメニュー送信

```
タイトル: 📝 ランク登録
説明: チーム分けのためランクを登録してください。登録後、自動的にエントリーされます。
セレクトメニュー（rank_select）:
  - Bronze / Silver / Gold / Platinum / Diamond / Master / Grandmaster / Top 500
```

#### 2. ランク選択（`handleRankSelect`）

- `pendingRegistrations[userID] = selectedRank` に保存
- Top500 の場合 → 即 `PlayerInfo` 保存 → `AddEntry` → Embed 更新 → ephemeral 更新
- それ以外 → ephemeral を**ディビジョン選択メニューに更新**（InteractionResponseUpdateMessage）

```
セレクトメニュー（division_select）:
  - 5（一番下） / 4 / 3 / 2 / 1（一番上）
```

#### 3. ディビジョン選択（`handleDivisionSelect`）

1. `pendingRegistrations[userID]` からランク取得
2. `PlayerInfo{ID, Name, HighestRank: Rank{Rank, Division}}` を `players.Add()` で保存
3. `pendingRegistrations` からエントリーを削除
4. `AddEntry` でエントリーに追加
5. 募集 Embed を更新
6. ephemeral を「✅ ランクを登録し、エントリーしました！」に更新

### エントリー取り消しハンドラー（`handleCancelEntry`）

1. `IsOpen` チェック
2. `RemoveEntry` → false なら ephemeral で「エントリーしていません」
3. Embed を更新
4. ephemeral で「エントリーを取り消しました」

### 締め切りハンドラー（`handleClose`）

1. `i.Member.User.ID == recruitment.OrganizerID` でなければ ephemeral でエラー
2. `IsOpen=false` に設定
3. エントリー者の PlayerInfo を参照してスコア計算 → `MakeTeams` 実行
4. チーム分け結果を新規メッセージで投稿（全員に見える）
5. 元の Embed のボタンを全て Disabled に更新

### 中止ハンドラー（`handleCancel`）

1. `OrganizerID` チェック
2. `IsOpen=false` に設定
3. 元の Embed を「🚫 募集は中止されました」に更新、ボタンを Disabled 化

---

## Embed 更新ヘルパー

```go
// 参加者リストを最新にして Embed を更新する
func (b *Bot) updateRecruitEmbed(s *discordgo.Session, disabled bool)
```

`session.ChannelMessageEditComplex` で `recruitment.ChannelID` / `MessageID` の Embed を更新する。

---

## セレクトメニューのオプション定義

### ランク（`rank_select`）
| Label | Value |
|-------|-------|
| Bronze | bronze |
| Silver | silver |
| Gold | gold |
| Platinum | platinum |
| Diamond | diamond |
| Master | master |
| Grandmaster | grandmaster |
| Top 500 | top500 |

### ディビジョン（`division_select`）
| Label | Value |
|-------|-------|
| 5（一番下） | 5 |
| 4 | 4 |
| 3 | 3 |
| 2 | 2 |
| 1（一番上） | 1 |

---

## テスト方針

### ユニットテスト（`recruitment_test.go` に追加）

- `RemoveEntry`: 存在するユーザーを削除 → true
- `RemoveEntry`: 存在しないユーザーを削除 → false

### 手動テスト手順

1. Bot 起動 → `/match` 実行 → Embed + ボタンが表示されること
2. ランク登録済みユーザーで「エントリー」 → 参加者リストが更新されること
3. ランク未登録ユーザーで「エントリー」 → ランク選択メニューが ephemeral で表示されること
4. ランク・ディビジョン選択 → 自動エントリー・Embed 更新されること
5. 同一ユーザーが再度「エントリー」 → ephemeral でエラーが出ること
6. 「取り消し」 → エントリーが消えること
7. 発案者以外が「締め切り」/「中止」 → ephemeral でエラーが出ること
8. 発案者が「締め切り」 → チーム分け結果が投稿されること
9. 発案者が「中止」 → Embed が中止表示になること

---

## 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `internal/model/recruitment.go` | `Recruitment` 構造体にフィールド追加、`RemoveEntry` メソッド追加 |
| `internal/bot/bot.go` | `/menu` 削除、`/match` + 全ボタン・セレクトメニューハンドラー追加、`Bot` 構造体に `pendingRegistrations` 追加 |
| `internal/model/recruitment_test.go` | `RemoveEntry` のテストを追加 |
