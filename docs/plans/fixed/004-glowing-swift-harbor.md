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
| `cmd/matchybot/main.go` | 要件2（vcConfigPath を New() に渡す） |

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
- **カテゴリ名**: `MatchyBot`（固定）
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

### `cmd/matchybot/main.go` の変更

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
| 2 | 初回「振り分け」押下 | `MatchyBot` カテゴリと `チームA`〜 VC が作成される |
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
| `cmd/matchybot/main.go` | `vcConfigFileName` 定数・`vcConfigPath` 生成・`bot.New` 第3引数追加 |
| `internal/bot/bot.go` | `Bot.vcConfig` フィールド・`New` シグネチャ変更・`handleMatchStart` で `r.GuildID = i.GuildID`・`handleAssign` にVC確保＋招待リンク並列生成追加・`ensureVCChannels` / `isDiscord404` 新規追加 |
| `internal/bot/bot_test.go` | `bot.New()` 第3引数追加に合わせてテスト修正 |
| `.gitignore` | `bin/vc_config.json` 追記 |

### 動作確認（手動テスト）

| # | 操作 | 結果 |
|---|------|------|
| 1 | 「中止」ボタン押下 | Embed が赤色になりボタンが消えることを確認 ✅ |
| 2 | 初回「振り分け」押下 | `MatchyBot` カテゴリと `チームA`〜 VC が作成された ✅ |
| 3 | 振り分け結果 Embed | 各チーム Field の末尾に `📢 VCに参加` リンクが表示された ✅ |
| 4 | リンクをクリック | 対応するVCチャンネルに参加できた ✅ |

---

## 追記実装結果（2026-02-25）

### 修正13: 振り分けボタンのインタラクションエラー ✅ 実装済み

- `handleAssign` の冒頭で `InteractionResponseDeferredMessageUpdate` を返すように変更
- defer 応答後のエラー通知は `FollowupMessageCreate`（ephemeral）で返すように変更
- Discord手動確認で、通常権限時にインタラクションエラーが表示されないことを確認

### 修正14: 余りメンバーの表示 ✅ 実装済み

- `Recruitment.MakeTeamsWithRemainder(players)` を追加（既存 `MakeTeams` は後方互換維持）
- 振り分け結果 Embed に `余りメンバー（N人）` Field を追加（余りがある場合のみ）
- `internal/model/recruitment_test.go` に余りメンバーのユニットテストを追加
- Discord手動確認で余りメンバー表示を確認

### 修正15: カテゴリ削除時の VC チャンネルリセット ✅ 実装済み

- `ensureVCChannels` の `categoryMissing` 時に保存済み `VCChannelIDs` を順に削除（`404` は無視）
- その後 `CategoryID` / `VCChannelIDs` をリセットしてカテゴリ・VCを全再作成
- 仕様補足:
  - カテゴリ欠損時は「削除して再作成」
  - カテゴリが存在し一部VCのみ欠損時は「不足分のみ再作成」
- Discord手動確認でカテゴリ削除後の再作成動作を確認

### 検証結果（今回追記分）

```bash
go test ./...
go build ./...
```

- 上記いずれも成功（2026-02-25）

---

## 次回改善事項（更新）

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

### 修正16: マッチングアルゴリズムの改善

現在のチーム分けはスコア分散最小化を目的とした簡易ランダム探索であり、
メンバー構成の偏り（ロール/実力帯の極端な偏り）が出る可能性がある。

次回は以下を検討する。

- 評価関数の見直し（平均値だけでなく最大差・標準偏差も考慮）
- 試行回数/探索戦略の改善（再現性のため乱数seed管理を含む）
- 将来的なロール考慮（DPS/Tank/Support）を見据えた拡張しやすい構造化

### 修正17: ランクデータの登録（再登録）コマンド実装

現在はランク登録導線が限定的で、登録済みユーザーの更新（再登録）を明示的に行いにくい。

次回は以下を検討する。

- ランク登録コマンドの明示化（新規登録）
- 既存ユーザーのランク再登録/更新コマンド追加
- 入力途中キャンセル/上書き時のUIメッセージ改善
- 手動確認観点の整備（登録→更新→振り分け反映）
