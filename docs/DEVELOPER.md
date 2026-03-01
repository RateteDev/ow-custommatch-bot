# 開発者向けガイド

このドキュメントは、ow-custommatch-bot をソースコードから変更・ビルドする開発者向けです。

以降の開発・ビルド・実行・テスト手順は、原則として `Makefile` 経由で実行します。
（個別の `go build` / `go test` コマンドは日常運用では使用しない前提）

## 必要なもの

- Go 1.26 以上

## セットアップ

初回のみ、依存パッケージを取得してください。

```bash
go mod download
```

## 開発用コマンド（Makefile）

| コマンド | 内容 |
|---|---|
| `make test` | `go test ./...` を実行。全パッケージのユニットテストを確認する |
| `make build` | `go build` でローカル用バイナリを `bin/ow-custommatch-bot` に出力 |
| `make run` | `make build` 後に `bin/ow-custommatch-bot` を起動する |
| `make release-win-exe` | `GOOS=windows GOARCH=amd64` でクロスコンパイルし `dist/ow-custommatch-bot.exe` を生成 |
| `make tag VERSION=v1.2.3` | アノテーション付き Git タグを作成して push。`v*` タグを検知した GitHub Actions が Windows exe のビルド・GitHub Release 作成・リリースノート自動生成を行う |

## 実行時に必要なファイル

- `ow-custommatch-bot.db` — 初回起動時に自動生成されます。

**補足:**

- ランクマスタは `go:embed` でバイナリに埋め込まれています。
- SQLite DB・VC 設定・ログは `%LOCALAPPDATA%\ow-custommatch-bot\` 配下に作成されます。
- `BOT_TOKEN` は Windows Credential Manager の `ow-custommatch-bot/BOT_TOKEN` に保存されます。

## 保存済みトークンの削除（開発者向け）

利用者向け画面には削除導線を出していません。開発や検証で保存済みトークンを消したい場合は、Windows のコマンドプロンプトまたは PowerShell で以下を実行してください。

```powershell
cmdkey /delete:ow-custommatch-bot/BOT_TOKEN
```

削除後に通常起動すると、初回起動と同様に `BOT_TOKEN` の入力を求められます。

## バージョン管理・リリース手順

バージョンの唯一の情報源は **Git Tag** です。ソースファイルにバージョン定数は持たず、ビルド時に `ldflags` で自動注入されます。

### バージョン番号の決め方（Semantic Versioning）

| 変更の種類 | バージョン例 | 具体例 |
|---|---|---|
| patch | `v0.1.1` | 不具合修正、軽微な調整 |
| minor | `v0.2.0` | 新コマンド追加、UI 改善 |
| major | `v1.0.0` | API 仕様の大幅変更 |

### リリース手順

```bash
# 1. テストが全件パスすることを確認
make test

# 2. タグを作成して push（GitHub Actions が自動でリリース作成）
make tag VERSION=v0.2.0
```

> **リリース後に更新するファイル**
> `assets/使い方.html` 内の直接ダウンロードリンクをバージョン番号に合わせて書き換えてください。
>
> ```
> https://github.com/RateteDev/ow-custommatch-bot/releases/download/vX.Y.Z/ow-custommatch-bot.exe
> ```

`make tag` を実行すると以下が行われます：
- `git tag -a v0.2.0 -m "Release v0.2.0"` でタグ作成
- `git push origin v0.2.0` でリモートに push
- GitHub Actions (`release.yml`) が起動し、Windows exe のビルド・GitHub Release 作成・リリースノート自動生成が行われます

> **リリースノートの編集**
> GitHub Actions が自動生成するリリースノートはコミットタイトルの羅列であり、そのままでは自然な文章になりません。
> GitHub Release が作成された後、Codex または Claude Code に「このリリースノートを利用者向けの日本語に書き直して」と依頼してリライトしてください。

### バージョン文字列の仕組み

| 状況 | 表示例 |
|---|---|
| タグが付いたコミット | `v0.2.0` |
| タグなし開発ビルド | `dev-a1b2c3d` |
| Git 外環境 | `dev` |

## 配布素材（Windows）

Windows 配布用の説明ファイルは以下に配置します。

- `assets/windows/使い方.html`
