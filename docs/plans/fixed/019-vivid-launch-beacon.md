# Context

テストモードの切替を `.env` の `OW_CUSTOMMATCH_BOT_TEST_MODE=true/false` で行っていたが、
対話環境での起動時に選択メニューを表示する方式に置き替える。

- `.env` によるテストモード制御を廃止
- バナー・パス表示後に起動モード選択メニューを表示（対話環境のみ）
- タイムアウト（5 秒）後はデフォルトの本番モードで自動起動
- `--test` フラグでも同等の効果を得られるようにする（非対話環境での起動支援）

# 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `cmd/ow-custommatch-bot/main.go` | `cliOptions` に `testMode` 追加、`--test` フラグ追加、`promptStartupMode()` 追加、`run()` にモード選択フロー追加 |
| `cmd/ow-custommatch-bot/main_test.go` | `promptStartupMode` のユニットテスト追加、`parseCLIArgs` テストに `--test` ケース追加 |
| `.env.example` | `OW_CUSTOMMATCH_BOT_TEST_MODE` 行を削除 |

# 実施順序

要件1（CLIフラグ）→ 要件2（promptStartupMode）→ 要件3（run フロー変更）→ 要件4（.env.example 更新）→ 要件5（テスト）

# 要件1: `cliOptions` に `testMode` を追加・`--test` フラグ対応

```go
type cliOptions struct {
    showHelp    bool
    showVersion bool
    testMode    bool
}
```

`parseCLIArgs` にフラグ追加：

```go
fs.BoolVar(&opts.testMode, "test", false, "テストモードで起動")
```

`cliUsageText` のオプション説明にも追記する：

```
  --test        テストモードで起動（テスト用ダミーデータを使用）
```

# 要件2: `promptStartupMode()` の追加

対話環境でのみ呼び出される起動モード選択関数。
goroutine での stdin 読み込みと `time.After` を組み合わせてタイムアウトを実現する。

```go
func promptStartupMode(ui startupUI, stdin io.Reader, timeout time.Duration) bool {
    fmt.Fprintln(ui.out)
    fmt.Fprintln(ui.out, ui.style.bold("  起動モードを選択してください"))
    fmt.Fprintln(ui.out, "    [1] 本番モード  (デフォルト)")
    fmt.Fprintln(ui.out, "    [2] テストモード")
    fmt.Fprintf(ui.out, "  %d秒後に [1] で自動起動します。> ", int(timeout.Seconds()))

    ch := make(chan string, 1)
    go func() {
        scanner := bufio.NewScanner(stdin)
        if scanner.Scan() {
            ch <- strings.TrimSpace(scanner.Text())
        }
        close(ch)
    }()

    select {
    case input, ok := <-ch:
        fmt.Fprintln(ui.out)
        return ok && input == "2"
    case <-time.After(timeout):
        fmt.Fprintln(ui.out)
        return false
    }
}
```

# 要件3: `run()` への起動モード選択フロー追加

バナー・パス表示（`ui.printPaths`）の直後、ステップ表示の前に挿入する。
これによりステップ番号 [1/4]〜[4/4] は変更しない。

```go
ui.printBanner(version)
ui.printPaths(exeDir, logPath, dbPath)

// --- 起動モード選択（ここから）---
testMode := opts.testMode
if !testMode && hasInteractiveConsole(stdin, stdout) {
    testMode = promptStartupMode(ui, stdin, 5*time.Second)
}
if testMode {
    if err := os.Setenv("OW_CUSTOMMATCH_BOT_TEST_MODE", "true"); err != nil {
        log.Printf("[WARN] テストモード環境変数の設定に失敗: %v", err)
    }
    fmt.Fprintf(stdout, "  %s テストモードで起動します。\n\n", ui.style.yellow("TEST"))
} else {
    fmt.Fprintf(stdout, "  %s 本番モードで起動します。\n\n", ui.style.green("PROD"))
}
// --- 起動モード選択（ここまで）---

ui.stepOK(1, 4, "ログ初期化")
// ... 以降は既存フロー
```

# 要件4: `.env.example` の更新

`OW_CUSTOMMATCH_BOT_TEST_MODE=false` 行を削除し、1行のみにする。

```
BOT_TOKEN=YOUR_DISCORD_BOT_TOKEN
```

# 検証方法

## ユニットテスト

- `parseCLIArgs([]string{"--test"})` → `opts.testMode == true`
- `promptStartupMode`:
  - stdin に `"2\n"` → `true` 返却
  - stdin に `"1\n"` → `false` 返却
  - stdin に `"\n"`（空 Enter）→ `false` 返却
  - タイムアウト（timeout=1ms, stdin に何も書かない）→ `false` 返却

## 手動確認

- ターミナルから `./ow-custommatch-bot` 起動
  → バナー・パス表示後にモード選択メニューが表示されること
  → `2` + Enter でテストモード起動すること
  → 何も入力せず 5 秒待機で本番モード起動すること
- `./ow-custommatch-bot --test` 起動
  → メニューなしでテストモード起動すること
- Discord で `/match` コマンドを実行
  → テストモード時は `fill_mode` オプションが表示されること
  → 本番モード時は表示されないこと

## 実装結果

### 実装ファイル一覧

- `cmd/ow-custommatch-bot/main.go`
  - `cliOptions` に `testMode bool` フィールド追加
  - `parseCLIArgs` に `--test` フラグ追加
  - `cliUsageText` にオプション説明追加
  - `promptStartupMode()` 追加（対話モード選択、5秒タイムアウト）
  - `run()` にモード選択フロー追加（`ui.printPaths` 直後、ステップ表示前）
- `cmd/ow-custommatch-bot/main_test.go`
  - `TestParseCLIArgs` に `--test` ケース追加
  - `TestPromptStartupMode` 追加（\"2\"→true、\"1\"→false、空→false、タイムアウト→false）
- `.env.example`
  - `OW_CUSTOMMATCH_BOT_TEST_MODE=false` 行を削除（1行のみに）

### 動作確認結果

- `go test ./...` 全件パス ✅
- `go build ./...` ビルド成功 ✅

## 次期改善事項

- モード選択メニューのカウントダウン表示（残り秒数をリアルタイム更新）
- 選択肢のキーバインドを拡張（e.g. `p` for PROD、`t` for TEST）
