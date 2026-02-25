# 実装プラン: 修正10・12（中止ボタン削除・VC誘導）

## Context

前プラン（003）で確認された以下の課題を解消する：

1. 中止後のボタンが無効化のままでパッと見で押してしまいそう
2. チーム振り分け後に各チームのVCへの導線がなく、手動で探す必要がある

---

## 変更ファイル

| ファイル | 対象要件 |
|---------|---------|
| `internal/bot/bot.go` | 要件1・2（全て） |
| `internal/model/vc_config.go`（新規） | 要件2（VCConfig 永続化） |
| `internal/model/recruitment.go` | 要件2（GuildID フィールド追加） |
| `cmd/ow-custommatch-bot/main.go` | 要件2（vcConfigPath を New() に渡す） |

---

## 実施順序

```
要件1 → 要件2（モデル層） → 要件2（bot層）
```

---

## 要件1: 中止後のボタン削除（bot.go）✅ 実装済み

`updateClosedEmbed` 内の `b.buildRecruitComponents(true)` を
`[]discordgo.MessageComponent{}` に変更済み（2026-02-25）。

---

## 要件2: 振り分け後の VC 誘導（bot.go + model）

### 方針

- **クリック移動**（強制移動なし）: 招待リンクをFieldに埋め込む
- **一時作成 + 永続化**: 初回振り分け時にカテゴリ・VCを作成し ID を保存、次回以降は再利用
- **カテゴリ名**: `ow-custommatch-bot`（固定）
- **VC名**: `チームA`, `チームB`, ...（`teamLabel` に合わせる）

### 招待リンク仕様

| パラメータ | 値 | 理由 |
|-----------|-----|------|
| `MaxAge` | `86400`（24時間） | 永久リンクを避ける |
| `MaxUses` | `0`（無制限） | チームメンバー全員が使用できるように |
| `Unique` | `true` | 毎回新しいリンクを生成 |

---

### 新規ファイル: `internal/model/vc_config.go`

```go
type VCConfig struct {
    CategoryID   string   `json:"category_id"`
    VCChannelIDs []string `json:"vc_channel_ids"`
}

type VCConfigManager struct {
    path string
    Data VCConfig
}

func NewVCConfigManager(path string) *VCConfigManager
func (m *VCConfigManager) Load() error   // ファイルなければ空データで正常終了
func (m *VCConfigManager) Save() error
```

永続化ファイル: `bin/vc_config.json`（`.gitignore` に追記）

---

### `internal/model/recruitment.go` の変更

`Recruitment` 構造体に `GuildID string` フィールドを追加。

```go
// Before
type Recruitment struct {
    OrganizerID string
    ChannelID   string
    MessageID   string
    IsOpen      bool
    // ...
}

// After
type Recruitment struct {
    OrganizerID string
    ChannelID   string
    MessageID   string
    GuildID     string   // ← 追加
    IsOpen      bool
    // ...
}
```

---

### `cmd/ow-custommatch-bot/main.go` の変更

```go
const vcConfigFileName = "vc_config.json"   // 追加

vcConfigPath := filepath.Join(exeDir, vcConfigFileName)
b, err := bot.New(playerDataPath, rankDataPath, vcConfigPath)  // 第3引数追加
```

---

### `internal/bot/bot.go` の変更

#### Bot struct

```go
type Bot struct {
    session              *discordgo.Session
    players              *model.PlayerDataManager
    rankData             model.RankDataFile
    recruitments         map[string]*model.Recruitment
    pendingRegistrations map[string]pendingRegEntry
    testDummies          map[string]map[string]model.PlayerInfo
    vcConfig             *model.VCConfigManager   // ← 追加
}
```

#### New() シグネチャ変更

```go
func New(playersPath, rankPath, vcConfigPath string) (*Bot, error)
```

内部で `model.NewVCConfigManager(vcConfigPath)` を呼び出し、`Load()` で既存設定を読み込む。

#### handleMatchStart の変更

```go
r.GuildID = i.GuildID
```

#### ensureVCChannels メソッド（新規）

```go
func (b *Bot) ensureVCChannels(s *discordgo.Session, guildID string, teamCount int) ([]string, error)
```

処理フロー:

```
1. b.vcConfig.Data.CategoryID が空 or Channel が存在しない
   → GuildChannelCreateComplex でカテゴリ作成
   → b.vcConfig.Data.CategoryID を更新

2. len(b.vcConfig.Data.VCChannelIDs) < teamCount
   → 不足分のVC（Type: ChannelTypeGuildVoice）をカテゴリ配下に作成
   → b.vcConfig.Data.VCChannelIDs に追記

3. b.vcConfig.Save()

4. b.vcConfig.Data.VCChannelIDs[:teamCount] を返す
```

**フォールバック**: `s.Channel(id)` が `404` を返す場合は再作成。

#### handleAssign の変更

振り分け Embed 構築後、以下を追加：

```
1. b.ensureVCChannels(s, r.GuildID, len(teams)) で vcChannelIDs 取得
2. 各チームの Field.Value 末尾に "\n[📢 VCに参加](招待URL)" を追記
   ※ ackInteraction より前に実行
```

招待リンク生成は `goroutine` で並列化してレイテンシを最小化する。

---

## 検証方法

### ユニットテスト

```bash
go test ./...
```

`VCConfigManager` の Load/Save と `Recruitment.GuildID` の追加に対して既存テストが全件パスすることを確認。

### 手動テスト

| # | 操作 | 期待結果 |
|---|------|---------|
| 1 | 「中止」ボタン押下 | Embed が赤色になり、ボタンが消える（無効化ではなく削除）|
| 2 | 初回「振り分け」押下 | `ow-custommatch-bot` カテゴリと `チームA`〜 VC が作成される |
| 3 | 振り分け結果 Embed | 各チーム Field の末尾に `📢 VCに参加` リンクが表示される |
| 4 | リンクをクリック | 対応するVCチャンネルに参加できる |
| 5 | 2回目の「振り分け」押下 | 既存 VC を再利用し、新規作成されない |
| 6 | VC を手動削除後に振り分け | フォールバックで再作成される |
| 7 | チーム数が前回より多い | 不足分のVCのみ追加作成される |

---

## 実装結果

2026-02-25 に codex エージェントが実装・`go test ./...` / `go build ./...` ともに全件パス。

### 実装ファイル一覧

| ファイル | 変更内容 |
|---------|---------|
| `internal/model/vc_config.go`（新規） | `VCConfig` / `VCConfigManager`（Load/Save）追加 |
| `internal/model/recruitment.go` | `Recruitment` に `GuildID string` 追加 |
| `cmd/ow-custommatch-bot/main.go` | `vcConfigFileName` 定数・`vcConfigPath` 生成・`bot.New` 第3引数追加 |
| `internal/bot/bot.go` | `Bot.vcConfig` フィールド・`New` シグネチャ変更・`handleMatchStart` で `r.GuildID = i.GuildID`・`handleAssign` にVC確保＋招待リンク並列生成追加・`ensureVCChannels` / `isDiscord404` 新規追加 |
| `internal/bot/bot_test.go` | `bot.New()` 第3引数追加に合わせてテスト修正 |
| `.gitignore` | `bin/vc_config.json` 追記 |

### 動作確認（手動テスト）

| # | 操作 | 結果 |
|---|------|------|
| 1 | 「中止」ボタン押下 | Embed が赤色になりボタンが消えることを確認 ✅ |
| 2 | 初回「振り分け」押下 | `ow-custommatch-bot` カテゴリと `チームA`〜 VC が作成された ✅ |
| 3 | 振り分け結果 Embed | 各チーム Field の末尾に `📢 VCに参加` リンクが表示された ✅ |
| 4 | リンクをクリック | 対応するVCチャンネルに参加できた ✅ |

---

## 次期改善事項

### 修正13: 振り分けボタンのインタラクションエラー

「振り分け」ボタン押下時にDiscord側でインタラクションエラーが表示されるが、
処理（VCチャンネル作成・招待リンク生成・Embed送信）は正常に完了している。

**原因推定**: `ensureVCChannels` + 招待リンク並列生成の合計レイテンシが
Discord のインタラクション応答制限（3秒）を超えている。

**対応方針**: `handleAssign` の冒頭で先に `s.InteractionRespond` で
`InteractionResponseDeferredMessageUpdate`（またはローディング応答）を返し、
処理完了後に Follow-up メッセージで結果を送る形に変更する。

### 修正14: 余りメンバーの表示

チーム分け後に余ったメンバーが現状表示されていない。
振り分け結果 Embed の末尾または独立した Field として「余りメンバー」を表示する。

- `MakeTeams` の戻り値を `(teams [][]ScoredPlayer, remainder []ScoredPlayer, err error)` に変更（要検討）
- または `Recruitment` に余りメンバーを保持する仕組みを追加する

### 修正15: カテゴリ削除時の VC チャンネルリセット

現状、カテゴリが見つからない場合はカテゴリのみ再作成し `VCChannelIDs` は維持する。
カテゴリが削除された場合は配下の VC も孤立していることが多いため、
`VCChannelIDs` をクリアして全て1から作り直す方針に変更する。

対象: `ensureVCChannels` の `categoryMissing` 処理ブロック。

```go
// categoryMissing == true のとき
b.vcConfig.Data.CategoryID = ""
b.vcConfig.Data.VCChannelIDs = []string{}  // ← 追加
```

### 修正11: チーム振り分け結果 Embed の見た目改善

動作確認済み。VCリンク追加後の実際の表示を踏まえて改善を検討する。

**有力案: 2列レイアウト（スペーサーField）**

2チームごとにゼロ幅スペースField（`Inline: true`, Name/Value = `"\u200b"`）を挿入し、
強制的に2列表示にする。

```
チームA  チームB  [空]
チームC  チームD  [空]
```

VCリンク追加でFieldの高さが増えると自動的に縦並びになる可能性があるため、
実際の表示を確認してから要否を判断する。

**その他の改善案**:
- Field名に人数表示（`チームA（6人）`）
- 区切り線セパレーター（案B）
- Description テキスト形式（案C）
- チームごとに別Embed・複数投稿（案D）
