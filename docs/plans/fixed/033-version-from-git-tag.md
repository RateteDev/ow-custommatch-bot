# 033-version-from-git-tag

## Context

現在のバージョン埋め込みは `git describe --tags --always --dirty` を基準にしていますが、
利用者向け表示・配布バイナリ・リリース起点の責務が明確に分かれていません。

今後は Git Tag を正式なバージョンソースにし、
バイナリ埋め込み、表示、配布、リリース作成を同じ起点から派生させたいという要望があります。

配布物の識別と再現性を高めるため、
バージョン決定ルールを Git Tag ベースへ整理します。

## 変更ファイル

| ファイル | 対応内容 |
|---------|---------|
| `Makefile` | Tag ベースの `VERSION` 解決ルールを整理 |
| `cmd/ow-custommatch-bot/main.go` | 埋め込みバージョンの前提に合わせて表示を調整 |
| `cmd/ow-custommatch-bot/main_test.go` | バージョン表示の期待値を更新 |
| `README.md` | リリース時のバージョン起点を補足 |
| `.github/workflows/release.yml` | Tag ベース方針に追従 |

## 実施順序

1. タグ付きビルドとタグ無しビルドの挙動を定義する
2. Makefile の `VERSION` ルールを実装する
3. テストと表示を合わせる
4. release workflow も同じ `VERSION` を参照するよう揃える

## 要件

### 要件1: 正式版は Git Tag をそのまま使う

- Tag 付きコミットからのビルドでは、その Tag 名を `VERSION` として使う
- 例: `v1.0.0` タグなら、バナー・`--version`・配布名の基準も `v1.0.0` とする

### 要件2: タグ無しビルドの扱いを明確にする

- タグ無しローカルビルドでは `dev-<shortsha>` を使う
- 例: `dev-f5b6d4c`
- 正式版の `v1.0.0` と混同しない形式に固定する

### 要件3: バイナリ埋め込みとリリースを同じ起点にする

- `go build` 時の埋め込み値と release 用ファイルのバージョン起点を揃える
- GitHub Release も同じ Tag を元に作成する

### 要件4: 既存の開発フローを壊しにくくする

- タグがない開発環境でも `make build` が失敗しない
- CI でも同じルールでバージョンが決まるようにする

### 要件5: テストと確認手順を用意する

- タグ付きビルドの表示確認
- タグ無しビルドの表示確認
- Makefile からビルドした成果物に同じバージョンが埋め込まれること

## 検証方法

- `go test ./cmd/ow-custommatch-bot/...`
- `make build` 後に `--version` を確認
- テスト用タグを切った状態で `make build` し、タグ名が反映されることを確認
- `release.yml` が Tag 起点の成果物名になっていることを確認

## 結果

- 2026-03-01: 完了
- `Makefile` の `VERSION` 解決は exact tag を優先し、タグなしでは `dev-<shortsha>`、`git` 利用不可時のみ `dev` を返す単一起点へ整理済み
- `.github/workflows/release.yml` は `make release-win-exe` に統一し、Tag 解決に必要な `fetch-depth: 0` を追加済み
- `README.md` に正式版は Git Tag 起点、開発ビルドは `dev-<shortsha>` 形式であることを追記し、`go test ./cmd/ow-custommatch-bot/...` と `make` の dry-run/実ビルドで確認済み
