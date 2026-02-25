# Context

現在の MatchyBot は以下の実行時データを JSON で扱っています。

- `player_data.json`（プレイヤー情報、読み書きあり）
- `vc_config.json`（VC 設定、読み書きあり）
- `rank.json`（ランクマスタ、読み取り専用）

今回の方針は以下です。

- 配布形態は `zip`（`exe` 単体配布は行わない）
- データの移行性を重視し、永続化ファイルは **実行ファイルと横並び** に配置する
- ランクマスタは外部ファイルではなく **`go:embed` でバイナリへ埋め込む**
- プレイヤー情報 / VC 設定は **SQLite** へ移行し、DB ファイルは初回起動時に自動作成する
- 保存先上書き用の環境変数は追加しない（運用/実装コスト削減）

狙いは、配布物の簡素化（`rank.json` 排除）と、JSON 直書きより安全な永続化（SQLite）への移行を、既存運用の移行性を保ちながら実現することです。

# 変更ファイル

- `cmd/matchybot/main.go`
  - 実行ファイル横の DB パス解決に変更
  - `rank.json` パス依存の削除
  - Bot 初期化引数の更新（SQLite + 埋め込みランク対応）
- `cmd/matchybot/main_test.go`
  - 実行ファイル基準パス運用のテスト補強
  - `rank.json` 不要化に伴う前提の更新
- `internal/bot/bot.go`
  - `bot.New(...)` の初期化シグネチャ見直し
  - プレイヤー / VC 設定のストア抽象（または SQLite マネージャ）への接続
- `internal/model/rank_data_manager.go`（または新規 `internal/model/rank_data.go`）
  - `go:embed` によるランクマスタ読み込み実装
  - JSON ファイルパス読み込み API の置換/併存整理
- `internal/model/player_data_manager.go`
  - JSON 実装の責務見直し（互換インポート用途へ縮退 or Store 抽象化）
- `internal/model/vc_config.go`
  - JSON 実装の責務見直し（互換インポート用途へ縮退 or Store 抽象化）
- `internal/model/*sqlite*.go`（新規）
  - SQLite 接続、スキーマ初期化、CRUD 実装
  - `players` / `vc_config` 永続化の実装
- `internal/model/*_test.go`（新規/既存更新）
  - SQLite 永続化のユニットテスト
  - JSON→SQLite 初回移行テスト
- `README.md`
  - 配布物/配置手順の更新（`rank.json` 配布不要、`matchybot.db` 自動作成）
- `docs/developer.md`
  - ビルド/実行時必要ファイルの更新
  - SQLite ドライバ依存（CGO 有無）に応じた注意事項追記
- `go.mod`, `go.sum`
  - SQLite ドライバ追加（候補: pure Go ドライバ）

# 実施順序

1. **テストで現行挙動の期待値を固定**
   - 既存 JSON 実装の振る舞い（プレイヤー登録・VC 設定保存）を characterization test として残す
2. **ランクマスタの `go:embed` 化**
   - 外部 `rank.json` 依存を切り離し、実行時ファイルを減らす
3. **SQLite 永続化層を追加（既存 JSON をまだ残す）**
   - スキーマ/CRUD/初回 DB 作成/ユニットテスト
4. **JSON→SQLite 初回移行処理を追加**
   - 既存 `player_data.json`, `vc_config.json` から取り込み
5. **Bot / main の接続先を SQLite に切替**
   - 実行ファイル横の DB を利用し、`rank.json` パスを撤去
6. **README / 開発者向けドキュメント更新**
   - zip 配布前提の手順に更新
7. **回帰確認**
   - `go test ./...` / `go build ./...` / 実機手動確認

# 要件1

## 要件1: ランクマスタを `go:embed` へ移行し、外部 `rank.json` 依存を撤廃する

### 目的

- 配布物から `rank.json` を外す
- `zip` 展開後の必須ファイル数を減らす
- ランクマスタ更新頻度が低い前提に合わせる

### 実装方針

- `go:embed` は `..` を辿れないため、**埋め込み対象JSONを Go ソースと同階層配下へ配置**する
  - 例: `internal/model/rankdata/rank.json` をマスタとして配置する
- `go:embed` でバイナリに埋め込む
- 既存の `LoadRankData(path string)` を以下のいずれかへ整理する
  - `LoadEmbeddedRankData() (RankDataFile, error)` を新設して Bot 初期化で使用する
  - 互換性のため `LoadRankData` は残し、テストやツール用途へ限定

### コードイメージ（例）

```go
// internal/model/rank_data_embedded.go
package model

import (
    _ "embed"
    "encoding/json"
)

//go:embed rankdata/rank.json
var embeddedRankJSON []byte

func LoadEmbeddedRankData() (RankDataFile, error) {
    var r RankDataFile
    if err := json.Unmarshal(embeddedRankJSON, &r); err != nil {
        return r, err
    }
    return r, nil
}
```

※ 既存 `data/rank.json` を残す場合は二重管理になるため、どちらか一方をマスタに統一すること。

### テスト

- 埋め込みランクデータを読めること
- `Ranks` が空でないこと（最低1件以上）
- 不正データ時のエラー系はユニットテスト上で byte 差し替え可能な構造なら追加

# 要件2

## 要件2: `players` / `vc_config` を SQLite 永続化へ移行する（DB 自動作成）

### 目的

- JSON 全体書き換えを廃止し、整合性/拡張性を向上
- 将来の検索・履歴追加に備える

### 実装方針

- SQLite ドライバは **pure Go 系を優先**（zip 配布/ビルド運用を簡素化するため）
- DB ファイルは `matchybot.db` とし、実行ファイルと同ディレクトリに作成
- 起動時に接続し、`CREATE TABLE IF NOT EXISTS ...` で初期化
- `players` は `id` を主キーにした UPSERT で保存
- `vc_config` は単一設定のため、最小構成は key-value または単行テーブル

### スキーマ案（初版）

```sql
CREATE TABLE IF NOT EXISTS players (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  main_role TEXT NOT NULL DEFAULT '',
  highest_rank TEXT NOT NULL DEFAULT '',
  highest_division TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS vc_config (
  singleton_key TEXT PRIMARY KEY,
  category_id TEXT NOT NULL DEFAULT '',
  vc_channel_ids_json TEXT NOT NULL DEFAULT '[]'
);
```

補足:

- `vc_channel_ids_json` は初版では JSON 文字列保存で十分
- 将来正規化が必要になった場合に別テーブル化を検討

### テスト

- 初回接続時に DB ファイル / テーブルが作成されること
- プレイヤー新規保存 → 取得 → 再起動相当の再接続後に取得できること
- 既存プレイヤー更新（ランク変更）が反映されること
- VC 設定保存/読込が従来挙動と互換であること

# 要件3

## 要件3: 既存 JSON から SQLite への初回移行処理を追加する（移行性重視）

### 目的

- 既存利用者の `player_data.json` / `vc_config.json` を失わずに移行する
- フォルダごとコピーによる移行性を維持する

### 実装方針

- 起動時に DB を初期化した後、以下を判定する
  - DB の `players` / `vc_config` が空
  - かつ JSON ファイルが存在する
- 条件を満たす場合のみ **一度だけ JSON からインポート**する
- インポート成功後の JSON ファイル扱いは以下のどちらか（要決定）
  - 推奨: 残置（バックアップ扱い、以後は読み書きしない）
  - 代替: `.migrated` 等へリネーム（実装コスト増）

このプランでは、まず **残置** を採用する。

### テスト

- 既存 JSON がある初回起動で DB へ取り込まれること
- 2回目起動で重複インポートしないこと
- JSON が存在しない新規環境でも正常起動できること

# 要件4

## 要件4: `main` / `bot` 初期化を新構成へ切り替える（実行ファイル横配置を維持）

### 目的

- 実行ファイル横の `.env` / `matchybot.db` を前提に統一
- `rank.json` パスの受け渡しを削除

### 実装方針

- 既存 `executableDir()` は継続利用（この方針と整合）
- `main.go` では以下を生成
  - `envPath := <exeDir>/.env`
  - `dbPath := <exeDir>/matchybot.db`
- `bot.New(...)` シグネチャを更新
  - 例: `bot.New(dbPath string)`（内部で埋め込みランクと SQLite を初期化）
  - もしくは依存注入しやすい形で `bot.NewWithDeps(...)` を追加

### テスト

- `cwd` が異なる前提でも `.env` / DB パス解決が `executableDir()` 基準であること（テスト可能な範囲で関数分割して確認）
- 初期化エラー時のメッセージが原因特定しやすいこと

# 要件5

## 要件5: ドキュメントと配布手順を更新する（zip 配布前提）

### 目的

- 利用者/開発者向け手順を新構成に一致させる
- 不要な `rank.json` 説明を除去する

### 実装方針

- `README.md`
  - 実行時に必要なファイルを `.env` のみに更新（DB は自動生成）
  - `rank.json` 配置手順を削除
  - `matchybot.db` が初回起動で生成されることを明記
  - `zip` 展開先は書き込み可能フォルダ推奨を明記
- `docs/developer.md`
  - 開発時の確認手順を更新
  - SQLite ドライバ依存に関する注意（必要なら）を追記

### 手動確認項目

- `bin/` に `.env` のみ配置して起動できること
- `bin/matchybot.db` が生成されること
- 既存 `player_data.json` / `vc_config.json` を置いた状態で起動し、DB へ移行されること

# 検証方法

## ユニットテスト

```bash
go test ./...
```

重点確認:

- `internal/model/` の SQLite CRUD
- JSON→SQLite 初回移行
- 埋め込みランク読込
- `cmd/matchybot` のパス解決/環境変数読込（既存テストを維持・拡張）

## ビルド確認

```bash
go build ./...
go build -o bin/matchybot ./cmd/matchybot/
```

## 手動テスト

1. 新規環境（`.env` のみ、DB/JSON なし）で起動し、`matchybot.db` 自動生成を確認
2. 既存 JSON 環境（`player_data.json`, `vc_config.json` あり）で起動し、データ移行後に従来動作することを確認
3. Discord 上でランク登録 → エントリー → チーム分け → VC 作成/再利用を確認
4. 再起動後にプレイヤー情報・VC 設定が維持されることを確認

## 実装結果

- 実装ファイル一覧（主な変更）
  - `internal/model/rank_data_embedded.go` を追加し、ランクマスタを `go:embed` で読込
  - `internal/model/rankdata/rank.json` をランクマスタの正本として配置
  - `internal/model/sqlite_store.go` / `internal/model/sqlite_store_test.go` を追加し、SQLite 永続化層（`players` / `vc_config`）を実装
  - `internal/model/player_data_manager.go` / `internal/model/vc_config.go` を SQLite バックエンド対応
  - `internal/bot/bot.go` の `bot.New(...)` を SQLite 初期化前提（`dbPath` のみ）へ変更
  - `cmd/matchybot/main.go` を `.env` + `matchybot.db` 前提に変更（実行ファイル横に DB 自動作成）
  - `README.md` / `docs/developer.md` を現行配布仕様（zip 配布、`rank` 埋め込み、SQLite 自動生成）へ更新
  - `data/rank.json` を削除（`internal/model/rankdata/rank.json` に一本化）
- 仕様変更 / プランとの差分
  - 要件3で計画していた **JSON→SQLite 初回移行処理は実装しない方針** に変更（後方互換不要のため）
  - `player_data.json` / `vc_config.json` は読み込まず、SQLite を唯一の永続化形式とした
- 動作確認結果
  - ユニットテスト: `go test ./...` ✅
  - ビルド確認: `go build ./...` ✅
  - 手動テスト（Discord）: `rank.json` なし起動、ランク登録、エントリー、振り分け成功 ✅
  - 手動テスト（Discord）: `bin/` をクリアして `.env` + バイナリのみで起動、`matchybot.db` 自動生成 ✅
  - 手動テスト（Discord）: 再起動後もデータ保持（ユーザー確認）✅

## 次期改善事項

- `SQLiteStore` のライフサイクル管理を `Bot` 側で明示し、終了時に `Close()` を呼ぶ設計に整理する
- `b.players.Data` / `b.vcConfig.Data` への並行アクセスに対する排他制御（`sync.Mutex` 等）を検討する
- SQLite スキーマのバージョン管理（`PRAGMA user_version` 等）を導入し、将来のテーブル変更に備える
- `docs/plans/fixed/005-embedded-stable-sqlite.md` の「手動テスト」欄は最終仕様に合わせて（JSON移行確認項目を削除して）整合を取る余地がある
