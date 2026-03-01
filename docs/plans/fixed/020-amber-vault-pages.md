# Context

現在の配布パッケージは `ow-custommatch-bot.exe` + `.env` + `使い方.html` の3点セットであり、
「exe をダブルクリックするだけ」とはならない。

本プランでは以下の3点を実装し、**exe 単体で完結する Windows 配布**を実現する。

1. **AppData 移行**: db・ログを `%LOCALAPPDATA%\ow-custommatch-bot\` に移動
2. **Windows Credential Manager**: BOT_TOKEN をOSのセキュアストアに保存
3. **GitHub Pages 案内**: エラーメッセージ・ヘルプのガイド参照先を Web URL に変更

`.env` は完全に廃止する。後方互換は不要。

---

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `cmd/ow-custommatch-bot/main.go` | AppData パス解決・トークン解決ロジック差し替え・不要関数削除・GitHub Pages URL 導入 |
| `cmd/ow-custommatch-bot/token_windows.go` | 新規: Windows Credential Manager 操作（build tag: `windows`） |
| `cmd/ow-custommatch-bot/token_other.go` | 新規: 非 Windows スタブ（build tag: `!windows`） |
| `cmd/ow-custommatch-bot/main_test.go` | .env 関連テスト削除・AppData/トークン解決/エラーメッセージのテスト追加 |
| `go.mod` / `go.sum` | `github.com/danieljoos/wincred` 追加 |
| `Makefile` | `package-win` から `.env` と `使い方.html` の同梱を削除 |

---

# 実施順序

要件1（AppData）→ 要件2（Credential Manager）→ 要件3（GitHub Pages）の順で実施する。
各要件完了後に `go test ./...` でテストを確認すること。

---

# 要件1: AppData 移行

## 概要

db・ログの保存先を exe ディレクトリから `%LOCALAPPDATA%\ow-custommatch-bot\` へ変更する。
exe 自体は Desktop など任意の場所に置いてよい。

## 追加する関数

`main.go` に `appDataDir` 関数を追加する。

```go
// appDataDir は OS に応じたアプリデータディレクトリを返す。
// Windows: %LOCALAPPDATA%\<name>
// 非 Windows: ~/.local/share/<name>
func appDataDir(name string) (string, error) {
    if runtimeGOOS == "windows" {
        local := os.Getenv("LOCALAPPDATA")
        if local == "" {
            return "", fmt.Errorf("LOCALAPPDATA 環境変数が設定されていません")
        }
        return filepath.Join(local, name), nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "", fmt.Errorf("ホームディレクトリを取得できません: %w", err)
    }
    return filepath.Join(home, ".local", "share", name), nil
}
```

`runtimeGOOS` はすでに `var runtimeGOOS = runtime.GOOS` として定義済みなので利用する。

## `run()` の変更

`run()` 冒頭の `executableDir()` 呼び出しは `.env` 参照に使っていたが、
`.env` を廃止するため **`exeDir` の取得は不要になる。削除すること。**

代わりに `dataDir` を取得・作成する。

```go
dataDir, err := appDataDir(appName)
if err != nil {
    ui.printErrorLine(fmt.Sprintf("起動に失敗しました: %v", err))
    return 1
}
if err := os.MkdirAll(dataDir, 0o755); err != nil {
    ui.printErrorLine(fmt.Sprintf("起動に失敗しました: データディレクトリを作成できません: %v", err))
    return 1
}
```

以降の db・ログパスを `exeDir` ベースから `dataDir` ベースに変更する。

```go
// Before
logFile, logPath, err := setupLogger(exeDir)
dbPath := filepath.Join(exeDir, dbFileName)
ui.printPaths(exeDir, logPath, dbPath)

// After
logFile, logPath, err := setupLogger(dataDir)
dbPath := filepath.Join(dataDir, dbFileName)
ui.printPaths(dataDir, logPath, dbPath)
```

`setupLogger` の引数名はドキュメント上は `exeDir` だが実装上はシグネチャを変えず、
引数に `dataDir` を渡すだけでよい。

## テスト

`TestAppDataDir` を追加する。

```go
func TestAppDataDir(t *testing.T) {
    origGOOS := runtimeGOOS
    t.Cleanup(func() { runtimeGOOS = origGOOS })

    t.Run("windows", func(t *testing.T) {
        runtimeGOOS = "windows"
        t.Setenv("LOCALAPPDATA", `C:\Users\user\AppData\Local`)
        dir, err := appDataDir("myapp")
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        want := `C:\Users\user\AppData\Local\myapp`
        if dir != want {
            t.Fatalf("got %q, want %q", dir, want)
        }
    })

    t.Run("windows LOCALAPPDATA empty", func(t *testing.T) {
        runtimeGOOS = "windows"
        t.Setenv("LOCALAPPDATA", "")
        if _, err := appDataDir("myapp"); err == nil {
            t.Fatalf("expected error when LOCALAPPDATA is empty")
        }
    })

    t.Run("non-windows", func(t *testing.T) {
        runtimeGOOS = "linux"
        dir, err := appDataDir("myapp")
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if !strings.HasSuffix(dir, filepath.Join("share", "myapp")) {
            t.Fatalf("unexpected path: %q", dir)
        }
    })
}
```

---

# 要件2: Windows Credential Manager によるトークン管理

## 概要

BOT_TOKEN の管理を `.env` から Windows Credential Manager に完全移行する。
初回起動時に対話プロンプトでトークンを入力し、Credential Manager へ保存する。
以降の起動では Credential Manager から自動読み込みする。

## 削除する関数（main.go から完全削除）

以下の関数は `.env` 依存であるため、**すべて削除する**。

- `loadEnvFile`
- `saveTokenToEnv`
- `requiredEnv`
- `describeStartupError`

定数 `envFileName = ".env"` および `botTokenPlaceholder` も不要になるため削除する。

## 依存ライブラリの追加

```bash
go get github.com/danieljoos/wincred
```

`github.com/danieljoos/wincred` は Pure Go（CGO 不要）で Windows Credential Manager を操作できる。

## `token_windows.go`（新規）

```go
//go:build windows

package main

import (
    "fmt"

    "github.com/danieljoos/wincred"
)

const credentialTarget = "ow-custommatch-bot"

// readTokenFromStore は Windows Credential Manager から BOT_TOKEN を読み込む。
// 登録がない場合は空文字とエラーを返す。
func readTokenFromStore() (string, error) {
    cred, err := wincred.GetGenericCredential(credentialTarget)
    if err != nil {
        return "", fmt.Errorf("credential not found: %w", err)
    }
    token := string(cred.CredentialBlob)
    if token == "" {
        return "", fmt.Errorf("credential blob is empty")
    }
    return token, nil
}

// saveTokenToStore は BOT_TOKEN を Windows Credential Manager に保存する。
func saveTokenToStore(token string) error {
    cred := wincred.NewGenericCredential(credentialTarget)
    cred.UserName = "BOT_TOKEN"
    cred.CredentialBlob = []byte(token)
    return cred.Write()
}
```

## `token_other.go`（新規）

```go
//go:build !windows

package main

import "fmt"

// readTokenFromStore は Windows 以外では常にエラーを返す（Credential Manager 非対応）。
func readTokenFromStore() (string, error) {
    return "", fmt.Errorf("Credential Manager は Windows 専用です")
}

// saveTokenToStore は Windows 以外では常にエラーを返す（Credential Manager 非対応）。
func saveTokenToStore(_ string) error {
    return fmt.Errorf("Credential Manager は Windows 専用です")
}
```

## テスト用オーバーライド変数

`main.go` にテスト用のオーバーライド変数を追加し、`run()` 内ではこの変数経由で呼び出す。

```go
// テスト時に差し替え可能にするためのパッケージレベル変数
var (
    readTokenFromStoreFn = readTokenFromStore
    saveTokenToStoreFn   = saveTokenToStore
)
```

## トークン解決ロジック（`run()` 内）

`.env` 関連の処理をすべて取り除き、以下の2段階に単純化する。

**優先順位:**
1. Windows Credential Manager（`readTokenFromStoreFn`）
2. 対話プロンプト → Credential Manager に保存

```go
// --- [2/3] トークン読み込み ---
ui.stepStart(2, 3, "トークン読み込み")
log.Printf("[INFO] [2/3] トークン読み込み ... 開始")

var botToken string

// 1. Credential Manager
if token, err := readTokenFromStoreFn(); err == nil && token != "" {
    botToken = token
    log.Printf("[INFO] BOT_TOKEN を Credential Manager から読み込みました")
}

// 2. 対話プロンプト
if botToken == "" {
    if !hasInteractiveConsole(stdin, stdout) {
        ui.stepFail(2, 3, "トークン読み込み")
        log.Printf("[ERROR] BOT_TOKEN が見つかりません")
        ui.printErrorLine(describeTokenError())
        return 1
    }
    fmt.Fprintln(ui.out)
    token, err := promptBotToken(ui, stdin)
    if err != nil {
        ui.stepFail(2, 3, "トークン読み込み")
        log.Printf("[ERROR] トークン入力に失敗: %v", err)
        ui.printErrorLine("起動に失敗しました: トークンが入力されませんでした。")
        return 1
    }
    if saveErr := saveTokenToStoreFn(token); saveErr != nil {
        log.Printf("[WARN] Credential Manager への保存に失敗: %v", saveErr)
    } else {
        log.Printf("[INFO] BOT_TOKEN を Credential Manager に保存しました")
    }
    botToken = token
}

ui.stepOK(2, 3, "トークン読み込み")
log.Printf("[INFO] [2/3] トークン読み込み ... OK")
```

## ステップ数の変更

`.env` 読み込みと必須設定チェックが統合されるため、ステップ数を 4 → 3 に変更する。

```
[1/3] ログ初期化
[2/3] トークン読み込み
[3/3] Bot 初期化
```

`stepStart` / `stepOK` / `stepFail` の呼び出し引数を対応して修正すること。

## `describeTokenError` の追加

Credential Manager に未登録かつ非インタラクティブな環境向けのエラーメッセージ。

```go
func describeTokenError() string {
    return fmt.Sprintf(
        "BOT_TOKEN が設定されていません。\nインタラクティブな環境で起動してトークンを入力してください。\n詳しい手順は %s をご確認ください。",
        guideURL,
    )
}
```

## 削除する既存テスト（main_test.go）

以下のテスト関数は `.env` 依存であるため**すべて削除する**。

- `TestLoadEnvFileSuccess`
- `TestLoadEnvFileErrors`
- `TestRequiredEnv`
- `TestSaveTokenToEnv`
- `TestDescribeStartupError`
- `TestExecutableDir`（`exeDir` 取得が不要になるため削除）

## 追加するテスト（main_test.go）

```go
func TestTokenResolution(t *testing.T) {
    origRead := readTokenFromStoreFn
    origSave := saveTokenToStoreFn
    t.Cleanup(func() {
        readTokenFromStoreFn = origRead
        saveTokenToStoreFn = origSave
    })

    t.Run("Credential Manager から読み込み成功", func(t *testing.T) {
        readTokenFromStoreFn = func() (string, error) { return "cred-token", nil }
        // トークン解決ロジックを resolveToken() 関数に切り出している場合はその関数をテスト。
        // 切り出していない場合は run() の統合テストで確認する。
    })

    t.Run("Credential Manager 失敗・非インタラクティブ → エラー", func(t *testing.T) {
        readTokenFromStoreFn = func() (string, error) { return "", fmt.Errorf("not found") }
        // hasInteractiveConsole が false を返す環境で run() を呼び、
        // 戻り値が 1（エラー終了）であることを確認する。
    })

    t.Run("prompt 後に Credential Manager へ保存", func(t *testing.T) {
        readTokenFromStoreFn = func() (string, error) { return "", fmt.Errorf("not found") }
        saved := ""
        saveTokenToStoreFn = func(token string) error {
            saved = token
            return nil
        }
        // promptBotToken を stdin モックで呼び出し、
        // saveTokenToStoreFn に渡ったトークンを検証する。
        _ = saved
    })
}
```

> **推奨**: `run()` 内のトークン解決ロジックを `resolveToken(stdin io.Reader, ui startupUI) (string, error)` に切り出すと、テストが書きやすくなる。codex の裁量に委ねる。

---

# 要件3: GitHub Pages への案内変更

## 概要

エラーメッセージ・ヘルプテキストで参照していたローカルの `使い方.html` パスを
GitHub Pages の URL に置き換える。

## 定数の追加

`main.go` の定数ブロックに追加する。

```go
const guideURL = "https://ratetedev.github.io/ow-custommatch-bot/"
```

## `cliUsageText` の修正

`.env` セットアップ手順を削除し、Credential Manager と GitHub Pages URL を案内する。

```go
func cliUsageText(exeName string) string {
    return fmt.Sprintf(`%s
使い方:
  %s [--help] [--version] [--test]

初回起動時にトークン入力を求められます。入力したトークンは
Windows Credential Manager に安全に保存され、次回以降は自動で読み込まれます。

詳しい手順: %s

オプション:
  --help, -h    このヘルプを表示
  --version     バージョンを表示
  --test        テストモードで起動（テスト用ダミーデータを使用）
`, appName, exeName, guideURL)
}
```

## `TestCLIUsageText` の更新

`.env` / `BOT_TOKEN` の検証を削除し、`guideURL` の検証に置き換える。

```go
func TestCLIUsageText(t *testing.T) {
    text := cliUsageText("ow-custommatch-bot")
    for _, want := range []string{"使い方", "--help", "--version", "--test", guideURL} {
        if !strings.Contains(text, want) {
            t.Fatalf("usage text missing %q: %s", want, text)
        }
    }
}
```

## Makefile の修正

`package-win` ターゲットから `.env` と `使い方.html` の同梱を削除する。

```makefile
# 削除する行
test -f "$(WIN_GUIDE_PATH)" || (echo "missing file: $(WIN_GUIDE_PATH)" && exit 1)
test -f "$(ENV_TEMPLATE_PATH)" || (echo "missing file: $(ENV_TEMPLATE_PATH)" && exit 1)
cp "$(ENV_TEMPLATE_PATH)" "$(WIN_PACKAGE_DIR)/.env"
cp "$(WIN_GUIDE_PATH)" "$(WIN_PACKAGE_DIR)/使い方.html"
```

合わせて、使われなくなった変数定義も削除する。

```makefile
# 削除する変数
WIN_GUIDE_PATH := assets/windows/使い方.html
ENV_TEMPLATE_PATH := .env.example
```

---

# 検証方法

## ユニットテスト

```bash
go test ./...
```

以下が通ること:
- `TestAppDataDir` (windows / LOCALAPPDATA empty / non-windows)
- `TestTokenResolution` (Credential Manager 成功 / 非インタラクティブエラー / prompt→保存)
- `TestCLIUsageText` (guideURL が含まれていること)
- 既存テスト（削除対象以外）全件パス

## ビルド確認

```bash
go build ./...
GOOS=windows GOARCH=amd64 go build ./cmd/ow-custommatch-bot/
```

どちらもエラーなしであること。

## 手動テスト（Windows 実機）

| # | 手順 | 期待結果 |
|---|------|---------|
| 1 | 初回: exe のみを配置してダブルクリック | トークン入力プロンプトが表示される |
| 2 | トークンを入力して起動 | Bot が起動し、`Logged in as ...` が表示される |
| 3 | 2回目: 同じ exe を再起動 | プロンプトなしで自動起動する |
| 4 | db・ログが `%LOCALAPPDATA%\ow-custommatch-bot\` に存在する | ✅ |
| 5 | Windows Credential Manager に `ow-custommatch-bot` が登録されている | ✅ |
| 6 | exe と同じフォルダに `.env` がなくても起動できる | ✅ |
| 7 | `--help` でガイド URL が表示される | `https://ratetedev.github.io/ow-custommatch-bot/` |
