package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RateteDev/ow-custommatch-bot/internal/bot"
)

const (
	envFileName = ".env"
	dbFileName  = "ow-custommatch-bot.db"
	appName     = "ow-custommatch-bot"
)

var version = "dev"

type cliOptions struct {
	showHelp    bool
	showVersion bool
}

type requiredEnvErr string

func (e requiredEnvErr) Error() string {
	return fmt.Sprintf("%s is required", string(e))
}

func executableDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.Dir(exePath), nil
}

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open env file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("invalid env format at line %d", lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return fmt.Errorf("empty env key at line %d", lineNo)
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan env file: %w", err)
	}
	return nil
}

func requiredEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", requiredEnvErr(key)
	}
	return value, nil
}

func setupLogger(exeDir string, consoleOut io.Writer) (io.Closer, string, error) {
	logDir := filepath.Join(exeDir, ".logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("create log dir: %w", err)
	}

	logPath := filepath.Join(logDir, time.Now().Format("2006-01-02T15-04-05")+".log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("open log file: %w", err)
	}

	log.SetOutput(io.MultiWriter(consoleOut, f))
	return f, logPath, nil
}

func parseCLIArgs(args []string) (cliOptions, error) {
	var opts cliOptions

	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&opts.showHelp, "help", false, "show help")
	fs.BoolVar(&opts.showHelp, "h", false, "show help")
	fs.BoolVar(&opts.showVersion, "version", false, "show version")
	if err := fs.Parse(args); err != nil {
		return cliOptions{}, err
	}
	if fs.NArg() > 0 {
		return cliOptions{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	return opts, nil
}

func cliUsageText(exeName string) string {
	return fmt.Sprintf(`%s
使い方:
  %s [--help] [--version]

起動前の準備:
  1. 実行ファイルと同じフォルダに .env を配置してください
  2. .env に BOT_TOKEN=... を設定してください

オプション:
  --help, -h    このヘルプを表示
  --version     バージョンを表示
`, appName, exeName)
}

func describeStartupError(envPath, requiredKey, _ string, err error) string {
	if os.IsNotExist(err) {
		return fmt.Sprintf(
			"設定ファイルが見つかりません: %s\nexe と同じフォルダに .env を配置し、例のように設定してください: %s=your-token",
			envPath,
			requiredKey,
		)
	}

	var envErr requiredEnvErr
	if errors.As(err, &envErr) {
		return fmt.Sprintf(
			"必須設定 %s が未設定です。%s に `%s=your-token` を追記してください。",
			string(envErr),
			envPath,
			string(envErr),
		)
	}

	return err.Error()
}

func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseCLIArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "引数エラー: %v\n\n%s", err, cliUsageText(appName))
		return 2
	}
	if opts.showHelp {
		fmt.Fprint(stdout, cliUsageText(appName))
		return 0
	}
	if opts.showVersion {
		fmt.Fprintf(stdout, "%s %s\n", appName, version)
		return 0
	}

	exeDir, err := executableDir()
	if err != nil {
		fmt.Fprintf(stderr, "起動に失敗しました: 実行ディレクトリを取得できませんでした: %v\n", err)
		return 1
	}

	closer, logPath, err := setupLogger(exeDir, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "起動に失敗しました: ログを初期化できませんでした: %v\n", err)
		return 1
	}
	defer closer.Close()

	log.Printf("[INFO] %s %s", appName, version)
	log.Printf("[INFO] 実行ディレクトリ: %s", exeDir)
	log.Printf("[INFO] [1/4] ログ初期化 ... OK")
	log.Printf("[INFO] ログファイル: %s", logPath)

	envPath := filepath.Join(exeDir, envFileName)
	log.Printf("[INFO] [2/4] 設定ファイル読込 (%s) ... 開始", envPath)
	if err := loadEnvFile(envPath); err != nil {
		log.Printf("[ERROR] 設定ファイル読込に失敗: %v", err)
		fmt.Fprintf(stderr, "起動に失敗しました: %s\n", describeStartupError(envPath, "BOT_TOKEN", dbFileName, err))
		return 1
	}
	log.Printf("[INFO] [2/4] 設定ファイル読込 ... OK")

	log.Printf("[INFO] [3/4] 必須設定チェック ... 開始")
	botToken, err := requiredEnv("BOT_TOKEN")
	if err != nil {
		log.Printf("[ERROR] 必須設定チェックに失敗: %v", err)
		fmt.Fprintf(stderr, "起動に失敗しました: %s\n", describeStartupError(envPath, "BOT_TOKEN", dbFileName, err))
		return 1
	}
	log.Printf("[INFO] [3/4] 必須設定チェック ... OK")

	dbPath := filepath.Join(exeDir, dbFileName)
	log.Printf("[INFO] [4/4] Bot初期化 (DB: %s) ... 開始", dbPath)

	b, err := bot.New(dbPath)
	if err != nil {
		log.Printf("[ERROR] Bot初期化に失敗: %v", err)
		fmt.Fprintf(stderr, "起動に失敗しました: Bot初期化に失敗しました（DB: %s）: %v\n", dbPath, err)
		return 1
	}
	log.Printf("[INFO] [4/4] Bot初期化 ... OK")
	log.Printf("[INFO] Discord接続を開始します。終了するには Ctrl+C を押してください。")

	if err := b.Run(botToken); err != nil {
		log.Printf("[ERROR] Bot実行エラー: %v", err)
		fmt.Fprintf(stderr, "実行中にエラーが発生しました: %v\n", err)
		return 1
	}

	log.Printf("[INFO] Botを終了しました。")
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
