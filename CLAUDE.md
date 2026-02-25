# MatchyBot — CLAUDE.md

## プロジェクト構造

```
cmd/matchybot/          エントリーポイント（main.go）
internal/bot/           Discord Bot 本体・全ハンドラー（bot.go）
internal/model/         データモデル・ビジネスロジック
  recruitment.go        募集データ・チーム分けロジック
  player_data_manager.go プレイヤーデータ（JSON 永続化）
  rank_data_manager.go  ランクデータ読み込み
  vc_config.go          VC チャンネル設定（JSON 永続化）
bin/                    ビルド成果物・実行時データファイル（.gitignore 対象）
docs/plans/             未着手・進行中のプランファイル
docs/plans/fixed/       実装完了済みのプランファイル（実装結果・次期改善事項を追記）
```

## 開発フロー

### 1. プランファイルの作成

実装前に `docs/plans/<id>-<name>.md` を作成し、以下のセクションを記述する。

```
# Context        課題の背景
# 変更ファイル    対象ファイルと対応要件の対応表
# 実施順序       要件間の依存関係
# 要件N          変更内容の詳細（コードスニペット含む）
# 検証方法       ユニットテスト + 手動テスト一覧
```

### 2. codex エージェントによる実装

プランファイルをもとに `subagent` スキル（codex CLI）へ実装を依頼する。

```
/subagent [プランの要件詳細をそのまま渡す]
```

- プランに記載した変更内容・注意事項を過不足なく伝える
- 「`go test ./...` で全件パス・`go build ./...` でビルド成功を確認すること」を必ず含める

### 3. 実装レビュー

codex の出力を確認したうえで、自分でも以下を実行して検証する。

```bash
go test ./...    # 全テストパス
go build ./...   # ビルドエラーなし
```

レビュー観点：
- プランの要件が全て実装されているか
- API の型・引数が正しいか（discordgo のシグネチャ等）
- goroutine / channel の使い方に競合がないか
- エラーハンドリングがプランの方針通りか

### 4. 動作確認

```bash
go build -o bin/matchybot ./cmd/matchybot/
./bin/matchybot
```

起動ログ（`Logged in as ...` / `MatchyBot (Go) is running`）を確認後、
Discord で手動テスト項目を実施する。

### 5. プランファイルへの実装結果記録

実装・動作確認が完了したら、プランファイルを `docs/plans/fixed/` へコピー（またはそのまま使用）し、
以下のセクションを追記して完了とする。

```
## 実装結果
  - 実装ファイル一覧（変更内容）
  - 動作確認結果（手動テスト ✅ / ❌）

## 次期改善事項
  - 動作確認で発見した不具合・課題
  - 将来的な改善アイデア
```

---

## ビルド・テストコマンド

```bash
go test ./...                              # 全パッケージのユニットテスト
go build ./...                             # 全パッケージのビルド確認
go build -o bin/matchybot ./cmd/matchybot/ # 実行バイナリのビルド
./bin/matchybot                            # Bot 起動
```

---

## プランファイルの命名規則

```
docs/plans/<3桁連番>-<ランダム英単語3つ>.md
例: 004-glowing-swift-harbor.md
```

完了後は `docs/plans/fixed/` に移動し、実装結果・次期改善事項を追記する。

---

## 注意事項

- **インタラクション応答制限（3秒）**: Discord API はボタン押下から 3 秒以内に応答を返さないとエラーになる。
  重い処理（API 呼び出し・並列処理）の前に `InteractionResponseDeferredMessageUpdate` で先に応答を返すこと。
- **`bin/` 配下のデータファイル**: `player_data.json` / `rank.json` / `vc_config.json` は `.gitignore` 対象。
  `bin/.gitkeep` のみコミット対象。
- **テスト対象外**: discordgo の API 呼び出しを含む関数はユニットテスト不可。
  `internal/model/` のロジック層（`MakeTeams`・`Load`/`Save` 等）をテスト対象とする。
