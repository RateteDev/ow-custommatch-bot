# Context

現在、Windows 向け配布 zip は `make package-win` を手動実行して生成している。
リリースの都度手作業が必要であり、配布漏れや手順ミスのリスクがある。

GitHub Actions を用いて、`v*` タグのプッシュ時に自動で以下を実行する CI/CD を整備する。

1. テスト実行（`go test ./...`）
2. Windows 向け zip 生成（`make package-win`）
3. GitHub Release の作成・zip のアップロード

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `.github/workflows/release.yml`（新規） | タグプッシュ時の自動リリースワークフロー |

# 実施順序

- 要件1（ワークフロー作成）のみ。依存関係なし。
- 要件2（バージョン埋め込み: plan 010）が完了している場合、ldflags でバージョンを渡す。

# 要件1: GitHub Actions ワークフロー作成

`.github/workflows/release.yml` を作成する。

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install 7z
        run: sudo apt-get install -y p7zip-full

      - name: Test
        run: go test ./...

      - name: Package Windows
        run: make package-win VERSION=${{ github.ref_name }}

      - name: Create Release
        uses: softprops/action-gh-release@v2
        with:
          files: dist/ow-custommatch-bot-win64.zip
```

# 要件2: Makefile への VERSION 引数対応

`make package-win` が `VERSION` 変数を受け取れるよう Makefile を修正する（plan 010 と連携）。

# 検証方法

## 手動確認

- `v0.0.1` などのテストタグをプッシュし、Actions が実行されることを確認
- GitHub Releases に zip がアップロードされていることを確認
- zip を展開し、`ow-custommatch-bot.exe --version` が正しいバージョンを表示することを確認

## 実装結果

- `.github/workflows/release.yml` を新規作成
  - `go-version-file: go.mod` で Go バージョンを自動取得（プランの固定値 `1.22` から改善）
  - plan 010 の ldflags 連携済み（`VERSION=${{ github.ref_name }}`）
- 手動確認: テストタグによる Actions 実行は未検証（マージ後に実施予定）

## 次期改善事項

- テストタグ `v0.0.1-test` での動作確認を実施する
