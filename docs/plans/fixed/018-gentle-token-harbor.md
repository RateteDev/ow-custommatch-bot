# Context

現在、`BOT_TOKEN` が未設定・プレースホルダーのままだと即エラー終了する。
対話環境（ターミナル直接起動）であればユーザーにトークン入力を促し、
入力値を `.env` に自動保存することで初回セットアップの手間を減らす。

非対話環境（systemd / CI 等）では従来通りエラー終了する。

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `cmd/ow-custommatch-bot/main.go` | `run()` に `stdin io.Reader` 追加、`promptBotToken()` / `saveTokenToEnv()` 追加、BOT_TOKEN 未設定時フロー変更 |
| `cmd/ow-custommatch-bot/main_test.go` | `promptBotToken` / `saveTokenToEnv` のユニットテスト追加、既存の `run()` 呼び出しを新シグネチャに更新 |

# 実施順序

要件1（run シグネチャ変更）→ 要件2（promptBotToken）→ 要件3（saveTokenToEnv）→ 要件4（run フロー変更）→ 要件5（テスト）

# 要件1: `run()` に `stdin io.Reader` を追加

```go
// 変更前
func run(args []string, stdout, stderr io.Writer) int

// 変更後
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int
```

`main()` の呼び出しも更新する。

```go
func main() {
    code := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
    pauseOnErrorExit(code, os.Stdin, os.Stdout)
    os.Exit(code)
}
```

# 要件2: `promptBotToken()` の追加

対話環境でトークン入力を促す。入力はエコーあり（平文表示）。

```go
func promptBotToken(ui startupUI, stdin io.Reader) (string, error) {
    fmt.Fprint(ui.out, "  BOT_TOKEN を入力してください: ")
    line, err := bufio.NewReader(stdin).ReadString('\n')
    if err != nil && err != io.EOF {
        return "", fmt.Errorf("read token: %w", err)
    }
    token := strings.TrimSpace(line)
    if token == "" || strings.EqualFold(token, botTokenPlaceholder) {
        return "", fmt.Errorf("トークンが入力されませんでした")
    }
    return token, nil
}
```

# 要件3: `saveTokenToEnv()` の追加

`.env` への BOT_TOKEN 保存。既存ファイルの有無・BOT_TOKEN 行の有無で処理を分ける。

```go
func saveTokenToEnv(envPath, token string) error {
    existing, err := os.ReadFile(envPath)
    if err != nil && !errors.Is(err, fs.ErrNotExist) {
        return fmt.Errorf("read env file: %w", err)
    }

    line := "BOT_TOKEN=" + token

    if len(existing) == 0 {
        // ファイルなし or 空 → 新規作成
        return os.WriteFile(envPath, []byte(line+"\n"), 0o644)
    }

    // 既存ファイル: BOT_TOKEN 行を更新、なければ末尾に追記
    lines := strings.Split(string(existing), "\n")
    found := false
    for i, l := range lines {
        key, _, ok := strings.Cut(strings.TrimSpace(l), "=")
        if ok && strings.TrimSpace(key) == "BOT_TOKEN" {
            lines[i] = line
            found = true
            break
        }
    }
    if !found {
        lines = append(lines, line)
    }

    content := strings.Join(lines, "\n")
    if !strings.HasSuffix(content, "\n") {
        content += "\n"
    }
    return os.WriteFile(envPath, []byte(content), 0o644)
}
```

# 要件4: `run()` 内の BOT_TOKEN 未設定時フロー変更

現在の「エラー → return 1」を、対話環境の場合のみプロンプトに差し替える。

```go
botToken, err := requiredEnv("BOT_TOKEN")
if err != nil {
    if hasInteractiveConsole(stdin, stdout) {
        fmt.Fprintln(stdout)
        token, promptErr := promptBotToken(ui, stdin)
        if promptErr != nil {
            ui.stepFail(3, 4, "必須設定チェック")
            log.Printf("[ERROR] トークン入力に失敗: %v", promptErr)
            ui.printErrorLine("起動に失敗しました: トークンが入力されませんでした。")
            return 1
        }
        if saveErr := saveTokenToEnv(envPath, token); saveErr != nil {
            // 保存失敗は警告のみ（起動は続行）
            log.Printf("[WARN] .env への保存に失敗しました: %v", saveErr)
        } else {
            log.Printf("[INFO] BOT_TOKEN を %s に保存しました", envPath)
        }
        botToken = token
    } else {
        ui.stepFail(3, 4, "必須設定チェック")
        log.Printf("[ERROR] 必須設定チェックに失敗: %v", err)
        ui.printErrorLine("起動に失敗しました: " + describeStartupError(envPath, "BOT_TOKEN", dbFileName, err))
        return 1
    }
}
ui.stepOK(3, 4, "必須設定チェック")
```

# 検証方法

## ユニットテスト

- `promptBotToken`: 正常入力 → token 返却、空入力 → error 返却、EOF → error 返却
- `saveTokenToEnv`:
  - .env なし → 新規ファイル作成・BOT_TOKEN 行が含まれること
  - .env あり・BOT_TOKEN 行なし → 末尾に追記されること
  - .env あり・BOT_TOKEN 行あり（placeholder）→ 該当行が更新されること

## 手動確認

- `.env` を削除または `BOT_TOKEN=YOUR_DISCORD_BOT_TOKEN` の状態でターミナル起動
  → プロンプトが表示されること
  → 入力後に `.env` が保存・更新されること
  → Bot が起動すること
- パイプ実行（非対話）: `echo "" | ./ow-custommatch-bot`
  → プロンプトなしでエラー終了すること

## 実装結果

### 実装ファイル一覧

- `cmd/ow-custommatch-bot/main.go`
  - `run()` シグネチャに `stdin io.Reader` 追加（引数順: args, stdin, stdout, stderr）
  - `main()` 呼び出しも `os.Stdin` を渡すよう更新
  - `promptBotToken()` 追加（BOT_TOKEN 未設定時の対話入力）
  - `saveTokenToEnv()` 追加（.env への BOT_TOKEN 保存・更新）
  - `run()` 内の BOT_TOKEN 未設定時フロー変更（対話環境はプロンプト、非対話はエラー終了）
- `cmd/ow-custommatch-bot/main_test.go`
  - `TestPromptBotToken` 追加（正常入力・空入力・プレースホルダー・EOF）
  - `TestSaveTokenToEnv` 追加（新規作成・追記・行更新）
  - 既存 `run()` 呼び出しを新シグネチャに更新

### 動作確認結果

- `go test ./...` 全件パス ✅
- `go build ./...` ビルド成功 ✅

## 次期改善事項

- トークン入力時にエコーを非表示にする（`golang.org/x/term` 等を使用）
- `.env` 保存パスをカスタマイズ可能にする
