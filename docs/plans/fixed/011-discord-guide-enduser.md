# Context

ユーザー向けの操作ガイドは `assets/windows/使い方.html` に集約する方針。
現在の `使い方.html` は最低限の 5 行のみで、操作フローや注意事項が欠けている。

また、配布 zip を入手しなくても Web ブラウザから手順を参照できるよう
GitHub Pages で `使い方.html` を公開する。

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `assets/windows/使い方.html` | 操作ガイドを充実させる（セットアップ・コマンド操作フロー・注意事項） |
| `.github/workflows/pages.yml`（新規） | GitHub Actions で Pages に `使い方.html` をデプロイ |

# 実施順序

要件1（HTML 拡充）→ 要件2（Pages デプロイ）の順で実施。

# 要件1: `使い方.html` の拡充

以下のセクションを追加・整理する。シンプルな HTML で読みやすく記述する。

## セットアップ（利用者向け）

1. Discord Developer Portal でアプリケーションを作成しトークンを取得する
2. `.env` の `BOT_TOKEN=` にトークンを貼り付ける
3. `ow-custommatch-bot.exe` を起動する
4. Discord サーバーに Bot を招待する（`bot` + `applications.commands` スコープ）

## コマンド一覧

| コマンド | 説明 |
|---------|------|
| `/match` | マッチング募集を開始する（発案者のみ） |
| `/register_rank` | チーム分け用ランクを登録・更新する |
| `/my_rank` | 登録済みランクと有効期限を確認する（plan 013 実装後） |

## `/match` の操作フロー

1. 発案者が `/match` → 募集 Embed が投稿される
2. 参加者が「✅ エントリー」を押す
   - ランク未登録 → ランク登録フローが開始（登録後に自動エントリー）
   - ランク登録が 30 日以上前 → 再登録フローが開始
3. 「❌ 取り消し」でエントリーを取り消せる
4. 10 人以上集まったら発案者が「🎲 振り分け」を押す
   - チーム分け結果と VC 招待リンクが投稿される
5. 発案者が「🚫 中止」で募集を終了できる

## ランク登録について

- エントリー時または `/register_rank` コマンドで登録できる
- ランクとディビジョン（1〜5）を選択する（Top 500 はディビジョン不要）
- 登録は **30 日間有効**。期限切れの場合は次回エントリー時に再登録を求められる

## チーム分けの仕組み

- 5 の倍数人数でチームを編成する。端数は「余りメンバー」として別表示
- ランクスコアを基準にバランスを取って振り分ける
- 「🔁 再振り分け」で何度でも再抽選できる

## 注意事項

- 1 チャンネルにつき同時に開ける募集は 1 つのみ
- 「🎲 振り分け」「🚫 中止」は発案者のみ実行可能

## トラブルシュート

- `failed to load env` → 実行ファイルと同じフォルダに `.env` があるか確認
- `BOT_TOKEN is required` → `.env` の `BOT_TOKEN` が空または初期値のままでないか確認
- スラッシュコマンドが表示されない → Bot 招待時に `applications.commands` スコープを付けたか確認（反映まで数分かかる場合あり）

# 要件2: GitHub Pages デプロイ

`.github/workflows/pages.yml` を作成し、`main` ブランチへのプッシュ時に
`assets/windows/使い方.html` を `index.html` として GitHub Pages に公開する。

```yaml
name: Deploy Pages

on:
  push:
    branches: [main]
    paths:
      - 'assets/windows/使い方.html'

permissions:
  contents: read
  pages: write
  id-token: write

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - uses: actions/checkout@v4

      - name: Prepare pages content
        run: |
          mkdir -p _site
          cp assets/windows/使い方.html _site/index.html

      - uses: actions/configure-pages@v4

      - uses: actions/upload-pages-artifact@v3
        with:
          path: _site

      - uses: actions/deploy-pages@v4
        id: deployment
```

- GitHub リポジトリ設定で Pages の Source を「GitHub Actions」に変更する必要がある

# 検証方法

## 手動確認

- `使い方.html` をブラウザで開き、全セクションが読みやすく表示されることを確認
- `main` へのプッシュ後、Actions が成功し Pages URL で閲覧できることを確認
- Pages URL を README に追記する

## 実装結果

- `assets/windows/使い方.html`: 全セクション（セットアップ・コマンド一覧・操作フロー・ランク登録・チーム分け・注意事項・トラブルシュート・終了方法）を追加
  - インラインCSS でモダンなカードレイアウトを実装
  - レスポンシブ対応済み
- `.github/workflows/pages.yml`: main ブランチへのプッシュ時に Pages デプロイ
- `go test ./...` 全件パス ✅
- `go build ./...` 成功 ✅

## 次期改善事項

- GitHub リポジトリ設定で Pages の Source を「GitHub Actions」に変更する必要あり
- Pages URL を README に追記する
- plan 013（`/my_rank`）実装後にコマンド一覧を更新する
