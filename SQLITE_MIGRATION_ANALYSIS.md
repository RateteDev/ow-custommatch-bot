# JSON永続化からSQLite永続化への切り替え分析レポート

## 結論（先に要点）

- **現時点の実装規模では、JSONのままでも運用可能**です。
- ただし、今後以下を強化したい場合は **SQLite化の効果が大きい**です。
  - 同時操作時の安全性（整合性・破損耐性）
  - データ件数増加時の検索性能
  - 永続化対象の拡張（履歴、統計、複数サーバー設定など）
- このリポジトリでは、**まずは可変データのみSQLite化するハイブリッド構成**（`players` / `vc_config` をSQLite、`rank.json` はJSONのまま）を推奨いたします。

## 現状の永続化実装（リポジトリ分析）

### 1. JSONで永続化している対象

- `players`（プレイヤー情報）
  - 実装: `internal/model/player_data_manager.go`
  - `NewPlayerDataManager()` でロード、ファイル未存在時は空データを作って保存
  - `Add()` / `Save()` 時に **ファイル全体をJSONで書き直し**
- `vc_config`（VCカテゴリ・VCチャンネルIDの設定）
  - 実装: `internal/model/vc_config.go`
  - `Load()` / `Save()` でJSON読み書き
  - `ensureVCChannels()`（`internal/bot/bot.go`）の最後で保存
- `rank`（ランク定義）
  - 実装: `internal/model/rank_data_manager.go`
  - **読み取り専用のJSONマスタ**（書き込みはしていない）

### 2. 現状の挙動上の特徴

- プレイヤー検索は `GetByID()` による **線形探索 O(n)**
- プレイヤー更新は `savePlayerRank()` 内で
  - 既存ユーザー: メモリ更新後 `players.Save()`
  - 新規ユーザー: `players.Add()`（内部で `Save()`）
- JSON保存は `os.WriteFile()` で直接書き込み（テンポラリファイル + rename ではない）

## SQLite化のメリット（このコードベースに対して）

### 1. 整合性・破損耐性の向上

現状は保存時にファイル全体を書き換えるため、プロセス停止タイミング次第でJSON破損リスクがございます。SQLiteはトランザクション管理が標準であり、**途中失敗時の整合性維持**に強いです。

### 2. 同時アクセスへの耐性向上

Discord Bot は複数ユーザー操作が同時に発生し得ます。現在の実装は永続化層に排他制御がなく、JSONファイル書き換えは競合時に弱い構造です。SQLiteは単一writer制約はあるものの、**DB側のロック制御**があり、JSON直書きより安全に扱えます。

### 3. 検索・更新性能の改善（将来拡張時）

`players` が増えると `GetByID()` の線形探索は効率が落ちます。SQLiteなら `PRIMARY KEY(user_id)` で即時検索でき、`UPSERT` による更新/新規登録の統一も容易です。

### 4. 機能追加に強い

今後、以下のような要件が出た場合にSQLiteは有利です。

- プレイヤー登録日時・更新日時の保持
- 参加履歴、勝敗履歴、レート推移
- サーバー（Guild）ごとの設定分離
- 管理コマンドによる検索/一覧/集計

## SQLite化のデメリット・注意点

### 1. 実装複雑性の増加

JSON管理は単純明快ですが、SQLite化では以下が必要になります。

- DB初期化
- スキーマ管理
- マイグレーション
- エラーハンドリング（ロック/接続/SQLエラー）
- テストの増強

### 2. 配布・ビルド方針との相性確認が必要

README上、このプロジェクトは **実行ファイル配布（exe）前提**です。SQLiteドライバの選定次第でビルド運用が変わります。

- `github.com/mattn/go-sqlite3`: CGO依存（Windowsクロスビルドや配布手順が重くなりやすい）
- pure Go系ドライバ（例: `modernc.org/sqlite`）: CGO不要だが、依存サイズや挙動確認が必要

この点は、JSON→SQLite移行の技術論よりも、**配布運用への影響**として重要です。

### 3. 人手での編集しやすさは下がる

JSONは目視編集しやすい一方、SQLiteは基本的にツールで閲覧します。運用者が直接編集するケースがあるなら、運用手順の整備が必要です。

## このリポジトリにおける推奨方針

### 推奨判断

- **今すぐ全面移行（rank含む）は不要**
- **可変データのみSQLite化を中期対応として検討**
- **短期的にはJSONの弱点補強でも十分な効果がある**

### 理由

- `rank.json` は読み取り専用の静的マスタであり、SQLite化のメリットが小さい
- 実際に更新されるのは `players` と `vc_config`
- 現状の主な課題は「検索性能」よりも「保存の安全性・競合耐性」の可能性が高い

## 段階的な改善案（おすすめ順）

### 案A: まずJSONのまま堅牢化（低コスト）

- 保存時にテンポラリファイルへ書いて `rename`（原子的更新に近づける）
- `PlayerDataManager` / `VCConfigManager` に `sync.Mutex` を追加
- テスト追加（保存中断時、同時書き込み想定の防御）

この対応だけでも、現状リスクはかなり下げられます。

### 案B: `players` と `vc_config` をSQLite化（本命）

- `rank.json` は現状維持
- `players` テーブル、`vc_config` テーブルを導入
- `savePlayerRank()` は `UPSERT` に置換
- `GetByID()` は `SELECT` に置換

### 案C: 全面SQLite化（将来要件が増えたら）

- `rank` もDB化して管理画面/更新機能を作る場合に検討
- 現状ではコスト先行になりやすいです

## SQLite化する場合の設計イメージ（最小構成）

### テーブル例

```sql
CREATE TABLE IF NOT EXISTS players (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  main_role TEXT NOT NULL DEFAULT '',
  highest_rank TEXT NOT NULL DEFAULT '',
  highest_division TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS vc_config (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
```

補足:

- `vc_config` は最初は key-value でも十分です
- 将来的に guild 単位対応するなら `guild_id` を持つ構造にする方がよいです

## TDDを意識した移行手順（提案）

本リポジトリのルールに合わせ、**テスト駆動での段階移行**を推奨いたします。

### 1. 先に振る舞いテストを固定化（Characterization Test）

対象:

- プレイヤー新規追加
- 既存プレイヤー更新（ランク上書き）
- 再起動後の再読込
- `vc_config` の空ファイル/未存在ファイルの挙動

### 2. 永続化インターフェースを導入

例（概念）:

- `PlayerStore`（`GetByID`, `Save/Upsert`）
- `VCConfigStore`（`Load`, `Save`）

Bot本体をJSON実装へ直接依存させず、差し替え可能にします。

### 3. SQLite実装を追加し、テストを流用

- `t.TempDir()` 配下に `test.db` を作成
- スキーマ初期化込みでテスト
- JSON実装と同じテスト観点を満たすことを確認

### 4. JSON→SQLite移行パスを用意

- 起動時に `players.json` が存在し、DBが空ならインポート
- または、明示的な移行コマンドを用意

### 5. 切替後にJSON実装を段階的に縮退

- 互換期間を設ける（必要なら読み取りのみ残す）

## 最終的な見解

- **「将来の拡張を見据えるならSQLite化は良い判断」**です。
- 一方で、**現時点のスコープだけを見ると必須ではありません**。
- 実務的には、以下の順序が最も安全かつ費用対効果が高いです。

1. 先にJSON保存の堅牢化（atomic write相当 + mutex）
2. テストで挙動固定
3. 可変データのみSQLite化（`players`, `vc_config`）
4. `rank.json` は静的マスタとして維持

