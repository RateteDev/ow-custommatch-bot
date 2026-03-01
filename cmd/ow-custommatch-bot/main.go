package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
var runtimeGOOS = runtime.GOOS
var hasInteractiveConsole = detectInteractiveConsole

const botTokenPlaceholder = "YOUR_DISCORD_BOT_TOKEN"

type cliOptions struct {
	showHelp    bool
	showVersion bool
	testMode    bool
}

type requiredEnvErr string

func (e requiredEnvErr) Error() string {
	return fmt.Sprintf("%s is required", string(e))
}

type ansiStyle struct {
	enabled bool
}

func (a ansiStyle) paint(code, s string) string {
	if !a.enabled || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (a ansiStyle) bold(s string) string        { return a.paint("1", s) }
func (a ansiStyle) dim(s string) string         { return a.paint("2", s) }
func (a ansiStyle) cyan(s string) string        { return a.paint("36", s) }
func (a ansiStyle) blue(s string) string        { return a.paint("34", s) }
func (a ansiStyle) green(s string) string       { return a.paint("32", s) }
func (a ansiStyle) yellow(s string) string      { return a.paint("33", s) }
func (a ansiStyle) red(s string) string         { return a.paint("31", s) }
func (a ansiStyle) magenta(s string) string     { return a.paint("35", s) }
func (a ansiStyle) whiteOnBlue(s string) string { return a.paint("37;44", s) }

type startupUI struct {
	out   io.Writer
	err   io.Writer
	style ansiStyle
}

func newStartupUI(out, err io.Writer) startupUI {
	return startupUI{
		out:   out,
		err:   err,
		style: ansiStyle{enabled: detectColorEnabled(out)},
	}
}

func (ui startupUI) printBanner(ver string) {
	fmt.Fprintln(ui.out)
	fmt.Fprintln(ui.out, ui.style.magenta("+- OW CUSTOMMATCH BOT --------------------------------------+"))
	fmt.Fprintf(ui.out, "%s %s\n", ui.style.bold("  Overwatch Custom Match Assistant"), ui.style.dim("v"+ver))
	fmt.Fprintln(ui.out, ui.style.magenta("+-----------------------------------------------------------+"))
	fmt.Fprintln(ui.out)
}

func (ui startupUI) printPaths(exeDir, logPath, dbPath string) {
	label := func(name string) string {
		return ui.style.cyan(fmt.Sprintf("%-4s", name))
	}
	fmt.Fprintf(ui.out, "%s %s\n", label("exe"), exeDir)
	fmt.Fprintf(ui.out, "%s %s\n", label("log"), logPath)
	fmt.Fprintf(ui.out, "%s %s\n", label("db"), dbPath)
	fmt.Fprintln(ui.out)
}

func (ui startupUI) stepStart(i, total int, title string) {
	fmt.Fprintf(
		ui.out,
		"%s %s %-28s %s\n",
		ui.style.blue(fmt.Sprintf("[%d/%d]", i, total)),
		ui.style.dim(">"),
		title,
		ui.style.dim("START"),
	)
}

func (ui startupUI) stepOK(i, total int, title string) {
	fmt.Fprintf(
		ui.out,
		"%s %s %-28s %s\n",
		ui.style.blue(fmt.Sprintf("[%d/%d]", i, total)),
		ui.style.dim(">"),
		title,
		ui.style.green("OK"),
	)
}

func (ui startupUI) stepFail(i, total int, title string) {
	fmt.Fprintf(
		ui.out,
		"%s %s %-28s %s\n",
		ui.style.blue(fmt.Sprintf("[%d/%d]", i, total)),
		ui.style.dim(">"),
		title,
		ui.style.red("FAIL"),
	)
}

func (ui startupUI) ready() {
	fmt.Fprintln(ui.out)
	fmt.Fprintf(ui.out, "%s Discord接続を開始します。終了するには %s を押してください。\n\n",
		ui.style.green("READY"),
		ui.style.bold("Ctrl+C"),
	)
}

func (ui startupUI) printErrorLine(msg string) {
	fmt.Fprintf(ui.err, "%s %s\n", ui.style.red("ERROR"), formatErrorMessageText(msg))
}

func formatErrorMessageText(msg string) string {
	rs := []rune(msg)
	if len(rs) == 0 {
		return msg
	}

	var b strings.Builder
	b.Grow(len(msg) + 16)
	for i, r := range rs {
		b.WriteRune(r)
		if r != '。' && r != '、' {
			continue
		}
		if i+1 >= len(rs) {
			continue
		}
		if rs[i+1] == '\n' {
			continue
		}
		b.WriteByte('\n')
	}
	return b.String()
}

type consoleLogWriter struct {
	out   io.Writer
	style ansiStyle
}

func (w *consoleLogWriter) Write(p []byte) (int, error) {
	s := string(p)
	if !w.style.enabled {
		_, err := io.WriteString(w.out, s)
		return len(p), err
	}

	var b strings.Builder
	for _, part := range strings.SplitAfter(s, "\n") {
		if part == "" {
			continue
		}
		b.WriteString(styleConsoleLogLine(part, w.style))
	}
	_, err := io.WriteString(w.out, b.String())
	return len(p), err
}

func styleConsoleLogLine(line string, style ansiStyle) string {
	if !style.enabled {
		return line
	}

	hasNL := strings.HasSuffix(line, "\n")
	body := strings.TrimSuffix(line, "\n")

	if i := strings.Index(body, " "); i > 0 && looksLikeLogTimestamp(body[:i]) {
		body = style.dim(body[:i]) + body[i:]
	}

	body = strings.ReplaceAll(body, "[INFO]", style.blue("[INFO]"))
	body = strings.ReplaceAll(body, "[WARN]", style.yellow("[WARN]"))
	body = strings.ReplaceAll(body, "[ERROR]", style.red("[ERROR]"))
	body = strings.ReplaceAll(body, "... OK", "... "+style.green("OK"))
	body = strings.ReplaceAll(body, "... 開始", "... "+style.yellow("開始"))
	body = strings.ReplaceAll(body, "Ctrl+C", style.bold("Ctrl+C"))

	body = colorizeProgressToken(body, style)
	body = strings.ReplaceAll(body, "Logged in as", style.green("Logged in as"))
	body = strings.ReplaceAll(body, "is running with", style.cyan("is running with"))

	if hasNL {
		return body + "\n"
	}
	return body
}

func colorizeProgressToken(s string, style ansiStyle) string {
	for i := 0; i < len(s); i++ {
		if s[i] != '[' {
			continue
		}
		endRel := strings.IndexByte(s[i:], ']')
		if endRel <= 0 {
			continue
		}
		end := i + endRel
		token := s[i : end+1]
		if !isProgressToken(token) {
			continue
		}
		return s[:i] + style.cyan(token) + s[end+1:]
	}
	return s
}

func isProgressToken(token string) bool {
	if len(token) < 5 || token[0] != '[' || token[len(token)-1] != ']' {
		return false
	}
	body := token[1 : len(token)-1]
	left, right, ok := strings.Cut(body, "/")
	if !ok || left == "" || right == "" {
		return false
	}
	if _, err := strconv.Atoi(left); err != nil {
		return false
	}
	if _, err := strconv.Atoi(right); err != nil {
		return false
	}
	return true
}

func looksLikeLogTimestamp(s string) bool {
	if len(s) != len("2006/01/02") {
		return false
	}
	return s[4] == '/' && s[7] == '/'
}

func detectColorEnabled(w io.Writer) bool {
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func detectInteractiveConsole(stdin io.Reader, stdout io.Writer) bool {
	inFile, ok := stdin.(*os.File)
	if !ok {
		return false
	}
	outFile, ok := stdout.(*os.File)
	if !ok {
		return false
	}

	inInfo, err := inFile.Stat()
	if err != nil {
		return false
	}
	outInfo, err := outFile.Stat()
	if err != nil {
		return false
	}

	return (inInfo.Mode()&os.ModeCharDevice) != 0 && (outInfo.Mode()&os.ModeCharDevice) != 0
}

func shouldPauseOnErrorExit(code int, stdin io.Reader, stdout io.Writer) bool {
	if code == 0 {
		return false
	}
	if runtimeGOOS != "windows" {
		return false
	}
	return hasInteractiveConsole(stdin, stdout)
}

func pauseOnErrorExit(code int, stdin io.Reader, stdout io.Writer) {
	if !shouldPauseOnErrorExit(code, stdin, stdout) {
		return
	}

	fmt.Fprint(stdout, "\nエラー終了しました。Enterキーを押すとウィンドウを閉じます...")
	_, _ = bufio.NewReader(stdin).ReadString('\n')
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
	if key == "BOT_TOKEN" && strings.EqualFold(value, botTokenPlaceholder) {
		return "", requiredEnvErr(key)
	}
	if value == "" {
		return "", requiredEnvErr(key)
	}
	return value, nil
}

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

func saveTokenToEnv(envPath, token string) error {
	existing, err := os.ReadFile(envPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read env file: %w", err)
	}
	line := "BOT_TOKEN=" + token
	if len(existing) == 0 {
		return os.WriteFile(envPath, []byte(line+"\n"), 0o644)
	}
	content := strings.TrimRight(string(existing), "\n")
	lines := strings.Split(content, "\n")
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
	content = strings.Join(lines, "\n")
	return os.WriteFile(envPath, []byte(content+"\n"), 0o644)
}

func setupLogger(exeDir string) (*os.File, string, error) {
	logDir := filepath.Join(exeDir, ".logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("create log dir: %w", err)
	}

	logPath := filepath.Join(logDir, time.Now().Format("2006-01-02T15-04-05")+".log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", fmt.Errorf("open log file: %w", err)
	}

	log.SetOutput(f)
	return f, logPath, nil
}

func enableConsoleLogging(logFile *os.File, consoleOut io.Writer, color bool) {
	log.SetOutput(io.MultiWriter(&consoleLogWriter{
		out:   consoleOut,
		style: ansiStyle{enabled: color},
	}, logFile))
}

func parseCLIArgs(args []string) (cliOptions, error) {
	var opts cliOptions

	fs := flag.NewFlagSet(appName, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&opts.showHelp, "help", false, "show help")
	fs.BoolVar(&opts.showHelp, "h", false, "show help")
	fs.BoolVar(&opts.showVersion, "version", false, "show version")
	fs.BoolVar(&opts.testMode, "test", false, "テストモードで起動")
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
  --test        テストモードで起動（テスト用ダミーデータを使用）
`, appName, exeName)
}

func describeStartupError(envPath, requiredKey, _ string, err error) string {
	guidePath := filepath.Join(filepath.Dir(envPath), "使い方.html")
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Sprintf(
			"設定ファイルが見つかりません: %s\nexe と同じフォルダに .env を配置し、例のように設定してください: %s=%s\n詳しい手順は同じフォルダの 使い方.html をご確認ください: %s",
			envPath,
			requiredKey,
			botTokenPlaceholder,
			guidePath,
		)
	}

	var envErr requiredEnvErr
	if errors.As(err, &envErr) {
		return fmt.Sprintf(
			"必須設定 %s が未設定です。%s に `%s=%s` を設定してください。\n`%s=%s` のままでも未設定扱いです。\n詳しい手順は同じフォルダの 使い方.html をご確認ください: %s",
			string(envErr),
			envPath,
			string(envErr),
			botTokenPlaceholder,
			string(envErr),
			botTokenPlaceholder,
			guidePath,
		)
	}

	msg := err.Error()
	if strings.Contains(msg, "invalid env format at line") || strings.Contains(msg, "empty env key at line") {
		return fmt.Sprintf(
			".env の書式が正しくありません（%s）。\n各行は `KEY=VALUE` 形式で記述してください（例: %s=%s）。\n詳しい手順は同じフォルダの 使い方.html をご確認ください: %s",
			msg,
			requiredKey,
			botTokenPlaceholder,
			guidePath,
		)
	}

	return fmt.Sprintf(
		"設定の読み込み中にエラーが発生しました（%s）。\n設定内容を確認し、詳しい手順は同じフォルダの 使い方.html をご確認ください: %s",
		msg,
		guidePath,
	)
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ui := newStartupUI(stdout, stderr)

	opts, err := parseCLIArgs(args)
	if err != nil {
		ui.printErrorLine(fmt.Sprintf("引数エラー: %v", err))
		fmt.Fprint(stderr, "\n"+cliUsageText(appName))
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
		ui.printErrorLine(fmt.Sprintf("起動に失敗しました: 実行ディレクトリを取得できませんでした: %v", err))
		return 1
	}

	logFile, logPath, err := setupLogger(exeDir)
	if err != nil {
		ui.printErrorLine(fmt.Sprintf("起動に失敗しました: ログを初期化できませんでした: %v", err))
		return 1
	}
	defer logFile.Close()

	dbPath := filepath.Join(exeDir, dbFileName)
	ui.printBanner(version)
	ui.printPaths(exeDir, logPath, dbPath)

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

	ui.stepOK(1, 4, "ログ初期化")

	log.Printf("[INFO] %s %s", appName, version)
	log.Printf("[INFO] 実行ディレクトリ: %s", exeDir)
	log.Printf("[INFO] [1/4] ログ初期化 ... OK")
	log.Printf("[INFO] ログファイル: %s", logPath)

	envPath := filepath.Join(exeDir, envFileName)
	log.Printf("[INFO] [2/4] 設定ファイル読込 (%s) ... 開始", envPath)
	if err := loadEnvFile(envPath); err != nil {
		ui.stepFail(2, 4, "設定ファイル読込")
		log.Printf("[ERROR] 設定ファイル読込に失敗: %v", err)
		ui.printErrorLine("起動に失敗しました: " + describeStartupError(envPath, "BOT_TOKEN", dbFileName, err))
		return 1
	}
	ui.stepOK(2, 4, "設定ファイル読込")
	log.Printf("[INFO] [2/4] 設定ファイル読込 ... OK")

	log.Printf("[INFO] [3/4] 必須設定チェック ... 開始")
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
	log.Printf("[INFO] [3/4] 必須設定チェック ... OK")

	log.Printf("[INFO] [4/4] Bot初期化 (DB: %s) ... 開始", dbPath)

	b, err := bot.New(dbPath)
	if err != nil {
		ui.stepFail(4, 4, "Bot初期化")
		log.Printf("[ERROR] Bot初期化に失敗: %v", err)
		ui.printErrorLine(fmt.Sprintf("起動に失敗しました: Bot初期化に失敗しました（DB: %s）: %v", dbPath, err))
		return 1
	}
	ui.stepOK(4, 4, "Bot初期化")
	log.Printf("[INFO] [4/4] Bot初期化 ... OK")
	ui.ready()
	log.Printf("[INFO] Discord接続を開始します。終了するには Ctrl+C を押してください。")
	enableConsoleLogging(logFile, stdout, ui.style.enabled)

	if err := b.Run(botToken); err != nil {
		log.Printf("[ERROR] Bot実行エラー: %v", err)
		ui.printErrorLine(fmt.Sprintf("実行中にエラーが発生しました: %v", err))
		return 1
	}

	log.Printf("[INFO] Botを終了しました。")
	return 0
}

func main() {
	code := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	pauseOnErrorExit(code, os.Stdin, os.Stdout)
	os.Exit(code)
}
