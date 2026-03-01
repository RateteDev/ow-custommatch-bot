# Context

現在、進行中の募集情報（`Recruitment` 構造体）はメモリのみで管理されており、
Bot を再起動すると進行中の募集が全て消える。

再起動後に発案者が再度 `/match` を実行しようとしても
「このチャンネルではすでに募集が開始されています」とはならないが、
既存の Embed のボタンは機能しなくなる（古い MessageID に紐づいたイベントを処理できない）。

SQLite に募集状態を保存し、Bot 再起動時に復元することで連続性を確保する。

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `internal/model/sqlite_store.go` | `recruitments` テーブルの追加・CRUD メソッド |
| `internal/model/recruitment.go` | JSON シリアライズ用タグの整理（必要に応じて） |
| `internal/bot/bot.go` | 募集作成・更新・終了時の永続化、起動時の復元 |

# 実施順序

要件1（DB スキーマ）→ 要件2（ストアメソッド）→ 要件3（Bot 側の永続化・復元）の順で実施。

# 要件1: `recruitments` テーブル追加

```go
// sqlite_store.go の initSchema に追加
CREATE TABLE IF NOT EXISTS recruitments (
    channel_id   TEXT PRIMARY KEY,
    guild_id     TEXT NOT NULL,
    organizer_id TEXT NOT NULL,
    message_id   TEXT NOT NULL,
    is_open      INTEGER NOT NULL DEFAULT 1,
    has_assigned INTEGER NOT NULL DEFAULT 0,
    entries_json TEXT NOT NULL DEFAULT '[]',
    created_at   TEXT NOT NULL
);
```

# 要件2: CRUD メソッドの追加

```go
// SaveRecruitment: 募集の作成・更新（upsert）
func (s *SQLiteStore) SaveRecruitment(r RecruitmentRow) error

// LoadOpenRecruitments: is_open=1 の募集を全件取得
func (s *SQLiteStore) LoadOpenRecruitments() ([]RecruitmentRow, error)

// CloseRecruitment: is_open=0 に更新
func (s *SQLiteStore) CloseRecruitment(channelID string) error
```

# 要件3: Bot 側の永続化・復元

- `handleMatchStart` で `SaveRecruitment` を呼ぶ
- `handleEntry` / `handleCancelEntry` でエントリーリスト更新後に `SaveRecruitment` を呼ぶ
- `handleCancel` / タイムアウト時に `CloseRecruitment` を呼ぶ
- `Bot.New()` 内で `LoadOpenRecruitments()` を呼び、`b.recruitments` を復元する

# 検証方法

## ユニットテスト

- `SaveRecruitment` → `LoadOpenRecruitments` で同じデータが返ること
- `CloseRecruitment` 後は `LoadOpenRecruitments` に含まれないこと

## 手動確認

- 募集開始・数人エントリー後に Bot を再起動する
- 再起動後に再度エントリーボタンを押し、エントリーリストが正しく更新されること
- 再起動前のエントリーが引き継がれていること

---

## 不採用理由

カスタムマッチの主催者が Bot を起動・管理する運用を前提としているため、
Bot 再起動をまたぐ募集の継続は現実的なユースケースとして発生しにくい。

SQLite スキーマ追加・CRUD 実装・復元ロジックと実装規模が大きく、
バグ混入リスクに対してメリットが小さいと判断した。
再起動が必要な場合は発案者が `/match` を打ち直せば足りる。
