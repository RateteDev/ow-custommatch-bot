# Context

現在、募集は発案者が明示的に「🚫 中止」を押すか、振り分け実行後に終了しない限り永続する。
発案者が離席・忘却した場合、チャンネルに古い募集 Embed が残り続ける。

一定時間（デフォルト 2 時間）エントリー操作がなければ自動で募集を終了する
タイムアウト機能を追加する。

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `internal/model/recruitment.go` | `LastActivityAt` フィールドの追加 |
| `internal/bot/bot.go` | タイムアウト監視 goroutine の追加・各操作での `LastActivityAt` 更新 |

# 実施順序

- 要件1（モデル変更）→ 要件2（タイムアウト監視）→ 要件3（活動時刻更新）の順で実施

# 要件1: `Recruitment` に `LastActivityAt` を追加

```go
type Recruitment struct {
    // ... 既存フィールド ...
    LastActivityAt time.Time // 最後にエントリー操作が行われた時刻
}
```

# 要件2: タイムアウト監視 goroutine

`Bot` に定期チェック goroutine を追加する。

```go
const recruitmentTimeoutDuration = 2 * time.Hour
const recruitmentCheckInterval  = 5 * time.Minute

func (b *Bot) startTimeoutWatcher(ctx context.Context) {
    ticker := time.NewTicker(recruitmentCheckInterval)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            b.checkRecruitmentTimeouts()
        }
    }
}

func (b *Bot) checkRecruitmentTimeouts() {
    now := b.nowUTC()
    for channelID, r := range b.recruitments {
        if !r.IsOpen {
            continue
        }
        if now.Sub(r.LastActivityAt) < recruitmentTimeoutDuration {
            continue
        }
        r.IsOpen = false
        _ = b.updateClosedEmbed(b.session, r, "⏰ 募集は時間切れにより終了しました")
        log.Printf("recruitment timed out: channel=%s", channelID)
    }
}
```

- `Bot.Run()` 内で goroutine を起動し、`SIGINT/SIGTERM` 受信時にキャンセルする

# 要件3: エントリー操作時に `LastActivityAt` を更新

- `handleEntry` / `handleCancelEntry` 成功後に `r.LastActivityAt = b.nowUTC()` を設定
- `handleMatchStart` 時にも初期値として設定する

# 検証方法

## ユニットテスト

- `checkRecruitmentTimeouts` に対して：
  - `LastActivityAt` が 2 時間以内 → `IsOpen` が変わらないこと
  - `LastActivityAt` が 2 時間超え → `IsOpen = false` になること

## 手動確認

- テスト用に `recruitmentTimeoutDuration` を 30 秒に短縮して動作確認
- 30 秒後に募集 Embed が「⏰ 時間切れ」表示になることを確認

---

## 不採用理由

このボットはカスタムマッチの主催者が自分で Bot を起動し、コマンドを打って運用する前提。
主催者がいる間しか Bot は使われないため、タイムアウトの恩恵を受ける場面が少ない。

goroutine・context 管理・時刻追跡と実装コストが高く、
「手動で中止する」という運用で十分だと判断した。
