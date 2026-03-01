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
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/RateteDev/ow-custommatch-bot/internal/bot"
)

const (
	dbFileName       = "ow-custommatch-bot.db"
	appName          = "ow-custommatch-bot"
	guideURL         = "https://ratetedev.github.io/ow-custommatch-bot/"
	portalURL        = "https://discord.com/developers/applications"
	tokenStoreTarget = appName + "/BOT_TOKEN"
)

var version = "dev"
var runtimeGOOS = runtime.GOOS
var hasInteractiveConsole = detectInteractiveConsole
var (
	readTokenFromStoreFn   = readTokenFromStore
	saveTokenToStoreFn     = saveTokenToStore
	deleteTokenFromStoreFn = deleteTokenFromStore
	newBotFn               = func(dbPath string) (botRunner, error) { return bot.New(dbPath) }
)

var (
	errTokenNotFound         = errors.New("bot token not found")
	errTokenStoreUnsupported = errors.New("credential store is not supported")
	errTokenInputEmpty       = errors.New("bot token is empty")
)

func displayVersion(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "dev"
	}
	if strings.HasPrefix(raw, "dev-") || raw == "dev" {
		return raw
	}
	if strings.HasPrefix(raw, "v") {
		if base, ok := stripLegacyGitDescribeSuffix(raw); ok {
			return base
		}
		return raw
	}
	if isPlainSemver(raw) {
		return "v" + raw
	}
	if isHexCommit(raw) {
		return "dev-" + raw
	}
	return raw
}

func stripLegacyGitDescribeSuffix(s string) (string, bool) {
	dirtySuffix := "-dirty"
	trimmed := s
	if strings.HasSuffix(trimmed, dirtySuffix) {
		trimmed = strings.TrimSuffix(trimmed, dirtySuffix)
	}

	lastDash := strings.LastIndexByte(trimmed, '-')
	if lastDash <= 0 || lastDash+2 >= len(trimmed) || trimmed[lastDash+1] != 'g' {
		return "", false
	}
	if !isHexCommit(trimmed[lastDash+2:]) {
		return "", false
	}

	countDash := strings.LastIndexByte(trimmed[:lastDash], '-')
	if countDash <= 0 {
		return "", false
	}
	for i := countDash + 1; i < lastDash; i++ {
		if trimmed[i] < '0' || trimmed[i] > '9' {
			return "", false
		}
	}

	return trimmed[:countDash], true
}

func isPlainSemver(s string) bool {
	if s == "" {
		return false
	}

	for i := 0; i < len(s); i++ {
		if (s[i] < '0' || s[i] > '9') && s[i] != '.' {
			return false
		}
	}
	return strings.Contains(s, ".")
}

func isHexCommit(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}

	for i := 0; i < len(s); i++ {
		switch {
		case s[i] >= '0' && s[i] <= '9':
		case s[i] >= 'a' && s[i] <= 'f':
		case s[i] >= 'A' && s[i] <= 'F':
		default:
			return false
		}
	}
	return true
}

type cliOptions struct {
	showHelp    bool
	showVersion bool
	testMode    bool
}

type startupAction int

const (
	startupActionStartProd startupAction = iota
	startupActionStartTest
	startupActionOverwriteToken
)

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

type botRunner interface {
	Run(token string) error
	SetReadyNotifier(func())
}

func newStartupUI(out, err io.Writer) startupUI {
	return startupUI{
		out:   out,
		err:   err,
		style: ansiStyle{enabled: detectColorEnabled(out)},
	}
}

func (ui startupUI) hyperlink(label, url string) string {
	if label == "" {
		return url
	}
	text := label
	if ui.style.enabled {
		text = ui.style.blue(label)
	}
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

func (ui startupUI) externalLink(label, url string) string {
	if strings.TrimSpace(url) == "" {
		return label
	}
	return ui.hyperlink(label, url)
}

func (ui startupUI) printBanner(ver string) {
	ver = displayVersion(ver)
	fmt.Fprintln(ui.out)
	fmt.Fprintln(ui.out, ui.style.magenta("+- OW CUSTOMMATCH BOT --------------------------------------+"))
	fmt.Fprintf(ui.out, "  %s\n", ui.style.bold("OW CUSTOMMATCH BOT"))
	fmt.Fprintf(ui.out, "  %s %s\n", ui.style.cyan("Version:"), ui.style.dim(ver))
	fmt.Fprintf(ui.out, "  %s %s\n", ui.style.cyan("使い方:"), ui.externalLink("ow-custommatch-bot ガイド", guideURL))
	fmt.Fprintln(ui.out, ui.style.magenta("+-----------------------------------------------------------+"))
	fmt.Fprintln(ui.out)
}

func (ui startupUI) printPaths(_ string, logPath, dbPath string) {
	label := func(name string) string {
		return ui.style.cyan(fmt.Sprintf("%-7s", name))
	}
	fmt.Fprintf(ui.out, "  %s %s\n", label("ログファイル"), logPath)
	fmt.Fprintf(ui.out, "  %s %s\n", label("データベース"), dbPath)
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
	fmt.Fprintf(ui.out, "%s %s\n", ui.style.green("🎉"), ui.style.bold("Discord との接続に成功しました。"))
	fmt.Fprintf(ui.out, "   %s コマンドで募集を開始できます。\n", ui.style.cyan("/match"))
	fmt.Fprintf(ui.out, "   %s コマンドでランクを登録・更新できます。\n", ui.style.cyan("/register_rank"))
	fmt.Fprintf(ui.out, "   %s コマンドで登録済みランクを確認できます。\n", ui.style.cyan("/my_rank"))
	fmt.Fprintf(ui.out, "\n   終了するには %s を押してください。\n\n",
		ui.style.bold("Ctrl+C"),
	)
}

func (ui startupUI) printErrorLine(msg string) {
	fmt.Fprintf(ui.err, "%s %s\n", ui.style.red("ERROR"), formatErrorMessageText(ui.linkifyMessage(msg)))
}

func (ui startupUI) linkifyMessage(msg string) string {
	replacer := strings.NewReplacer(
		portalURL, ui.externalLink("Discord Developer Portal", portalURL),
		guideURL, ui.externalLink("使い方ページ", guideURL),
	)
	return replacer.Replace(msg)
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

func promptBotToken(ui startupUI, stdin io.Reader) (string, error) {
	fmt.Fprint(ui.out, "  BOT_TOKEN を入力してください: ")
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read token: %w", err)
	}
	token := strings.TrimSpace(line)
	if token == "" {
		return "", errTokenInputEmpty
	}
	return token, nil
}

func (ui startupUI) formatStartupActionLine(action startupAction, label string) string {
	switch action {
	case startupActionStartProd:
		return ui.style.green(label)
	case startupActionStartTest:
		return ui.style.yellow(label)
	case startupActionOverwriteToken:
		return ui.style.cyan(label)
	default:
		return label
	}
}

func (ui startupUI) printStartupActionMenu() {
	fmt.Fprintln(ui.out)
	fmt.Fprintln(ui.out, ui.style.bold("  起動方法を選択してください"))
	fmt.Fprintln(ui.out, "  普段そのままお使いになる場合は [1] を選んでください。")
	fmt.Fprintln(ui.out, "  表示確認や試運転をしたい場合は [2] を選んでください。")
	fmt.Fprintln(ui.out, "  保存済みトークンを更新したい場合は [3] を選んでください。")
	fmt.Fprintln(ui.out)
	fmt.Fprintln(ui.out, "    "+ui.formatStartupActionLine(startupActionStartProd, "[1] 通常運用"))
	fmt.Fprintln(ui.out, "        実際の運用として起動します。")
	fmt.Fprintln(ui.out, "    "+ui.formatStartupActionLine(startupActionStartTest, "[2] 動作確認用"))
	fmt.Fprintln(ui.out, "        テスト用ダミーデータで画面や流れを確認できます。")
	fmt.Fprintln(ui.out, "    "+ui.formatStartupActionLine(startupActionOverwriteToken, "[3] トークンを上書きする"))
	fmt.Fprintf(ui.out, "        保存先: %s\n", tokenStorageLocationLabel())
	fmt.Fprintln(ui.out)
	fmt.Fprint(ui.out, "  Enterキーで通常運用を開始できます。> ")
}

func promptStartupAction(ui startupUI, stdin io.Reader, _ time.Duration) startupAction {
	reader := bufio.NewReader(stdin)
	for {
		ui.printStartupActionMenu()
		line, err := reader.ReadString('\n')
		fmt.Fprintln(ui.out)
		if err != nil && err != io.EOF {
			return startupActionStartProd
		}
		input := strings.TrimSpace(line)
		switch input {
		case "", "1":
			return startupActionStartProd
		case "2":
			return startupActionStartTest
		case "3":
			return startupActionOverwriteToken
		}
		if err == io.EOF {
			return startupActionStartProd
		}
		ui.printErrorLine("1 / 2 / 3 / Enter のいずれかを入力してください")
		fmt.Fprintln(ui.out)
	}
}

func promptStartupMode(ui startupUI, stdin io.Reader, timeout time.Duration) bool {
	return promptStartupAction(ui, stdin, timeout) == startupActionStartTest
}

func startupModeConfirmationMessage(testMode bool) string {
	if testMode {
		return "動作確認用で起動します。"
	}
	return "通常運用で起動します。"
}

func setupLogger(dataDir string) (*os.File, string, error) {
	logDir := filepath.Join(dataDir, ".logs")
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
	fs.BoolVar(&opts.testMode, "test", false, "動作確認用で起動")
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
  1. 初回起動時に BOT_TOKEN を入力すると保存されます
  2. Windows では Credential Manager に安全に保存されます
  3. 保存済みトークンを変更したい場合は起動メニューの「トークンを上書きする」を使います
  4. 詳しい手順: %s

オプション:
  --help, -h    このヘルプを表示
  --version     バージョンを表示
  --test        動作確認用で起動（テスト用ダミーデータを使用）
`, appName, exeName, guideURL)
}

func tokenStorageLocationLabel() string {
	if runtimeGOOS == "windows" {
		return "Windows Credential Manager"
	}
	return "この OS では資格情報ストアを利用できません"
}

func overwriteStoredToken(ui startupUI, stdin io.Reader) error {
	fmt.Fprintln(ui.out)
	fmt.Fprintf(ui.out, "  保存先: %s\n", tokenStorageLocationLabel())
	token, err := promptBotToken(ui, stdin)
	if err != nil {
		return err
	}
	if err := saveTokenToStoreFn(token); err != nil {
		return fmt.Errorf("トークンの保存に失敗しました: %w", err)
	}
	fmt.Fprintln(ui.out, "  保存済みトークンを更新しました。続けて起動方法を選択してください。")
	return nil
}

func resolveToken(stdin io.Reader, ui startupUI) (string, error) {
	token, err := readTokenFromStoreFn()
	if err == nil {
		token = strings.TrimSpace(token)
		if token == "" {
			return "", errTokenNotFound
		}
		return token, nil
	}
	if !errors.Is(err, errTokenNotFound) && !errors.Is(err, errTokenStoreUnsupported) {
		return "", fmt.Errorf("トークンの読み込みに失敗しました: %w", err)
	}
	if !hasInteractiveConsole(stdin, ui.out) {
		return "", err
	}

	fmt.Fprintln(ui.out)
	token, err = promptBotToken(ui, stdin)
	if err != nil {
		return "", err
	}
	if err := saveTokenToStoreFn(token); err != nil {
		return "", fmt.Errorf("トークンの保存に失敗しました: %w", err)
	}
	return token, nil
}

func describeTokenError(err error) string {
	switch {
	case errors.Is(err, errTokenNotFound):
		return fmt.Sprintf("BOT_TOKEN が保存されていません。初回起動時は対話入力で保存してください。詳しい手順は %s をご確認ください。", guideURL)
	case errors.Is(err, errTokenStoreUnsupported):
		return fmt.Sprintf("この OS では資格情報ストアを利用できません。対応手順は %s をご確認ください。", guideURL)
	case errors.Is(err, errTokenInputEmpty):
		return fmt.Sprintf("トークンが入力されませんでした。詳しい手順は %s をご確認ください。", guideURL)
	default:
		return fmt.Sprintf("BOT_TOKEN の設定に失敗しました（%v）。詳しい手順は %s をご確認ください。", err, guideURL)
	}
}

func describeAuthRecoveryError(err error) string {
	switch {
	case errors.Is(err, errTokenInputEmpty):
		return fmt.Sprintf("トークンが入力されませんでした。詳しい手順は %s をご確認ください。", guideURL)
	case err != nil && strings.Contains(err.Error(), "トークンの保存に失敗しました"):
		return fmt.Sprintf("%v。詳しい手順は %s をご確認ください。", err, guideURL)
	case err != nil && strings.Contains(err.Error(), "再試行"):
		return fmt.Sprintf("%v。Discord Developer Portal で BOT TOKEN を再発行または確認してください。%s", err, portalURL)
	default:
		return describeTokenError(err)
	}
}

func isAuthenticationFailureError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "4004") && strings.Contains(msg, "authentication failed")
}

func recoverFromAuthenticationFailure(ui startupUI, stdin io.Reader, b botRunner, err error) error {
	ui.printErrorLine(fmt.Sprintf("実行中にエラーが発生しました: %v", err))
	fmt.Fprintln(ui.out)
	fmt.Fprintf(ui.out, "  %s で BOT TOKEN を再発行または確認してください。\n", ui.externalLink("Discord Developer Portal", portalURL))
	fmt.Fprintln(ui.out, "  新しい BOT_TOKEN を入力すると、その場で上書きして再試行します。")
	fmt.Fprintln(ui.out)

	token, promptErr := promptBotToken(ui, stdin)
	if promptErr != nil {
		return promptErr
	}
	if err := saveTokenToStoreFn(token); err != nil {
		return fmt.Errorf("トークンの保存に失敗しました: %w", err)
	}

	fmt.Fprintln(ui.out, "  保存済みトークンを更新しました。接続を再試行します。")
	if err := b.Run(token); err != nil {
		if isAuthenticationFailureError(err) {
			return fmt.Errorf("認証失敗のため再試行しましたが、再度失敗しました: %w", err)
		}
		return fmt.Errorf("再試行後も実行中にエラーが発生しました: %w", err)
	}
	return nil
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
		fmt.Fprintf(stdout, "%s %s\n", appName, displayVersion(version))
		return 0
	}

	dataDir, err := appDataDir(appName)
	if err != nil {
		ui.printErrorLine(fmt.Sprintf("起動に失敗しました: データ保存先を取得できませんでした: %v", err))
		return 1
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		ui.printErrorLine(fmt.Sprintf("起動に失敗しました: データ保存先を作成できませんでした: %v", err))
		return 1
	}

	logFile, logPath, err := setupLogger(dataDir)
	if err != nil {
		ui.printErrorLine(fmt.Sprintf("起動に失敗しました: ログを初期化できませんでした: %v", err))
		return 1
	}
	defer logFile.Close()

	dbPath := filepath.Join(dataDir, dbFileName)
	ui.printBanner(version)
	ui.printPaths(dataDir, logPath, dbPath)

	testMode := opts.testMode
	if !testMode && hasInteractiveConsole(stdin, stdout) {
		for {
			action := promptStartupAction(ui, stdin, 5*time.Second)
			if action == startupActionOverwriteToken {
				if err := overwriteStoredToken(ui, stdin); err != nil {
					ui.printErrorLine("起動に失敗しました: " + describeTokenError(err))
					return 1
				}
				continue
			}
			testMode = action == startupActionStartTest
			break
		}
	}
	if testMode {
		if err := os.Setenv("OW_CUSTOMMATCH_BOT_TEST_MODE", "true"); err != nil {
			log.Printf("[WARN] テストモード環境変数の設定に失敗: %v", err)
		}
	} else {
	}
	modeLabel := startupModeConfirmationMessage(testMode)
	if testMode {
		fmt.Fprintf(stdout, "  %s\n\n", ui.style.yellow(modeLabel))
	} else {
		fmt.Fprintf(stdout, "  %s\n\n", ui.style.green(modeLabel))
	}

	ui.stepOK(1, 3, "ログ初期化")

	log.Printf("[INFO] %s %s", appName, version)
	log.Printf("[INFO] データ保存先: %s", dataDir)
	log.Printf("[INFO] [1/3] ログ初期化 ... OK")
	log.Printf("[INFO] ログファイル: %s", logPath)

	log.Printf("[INFO] [2/3] トークン読込 ... 開始")
	botToken, err := resolveToken(stdin, ui)
	if err != nil {
		ui.stepFail(2, 3, "トークン読込")
		log.Printf("[ERROR] トークン読込に失敗: %v", err)
		ui.printErrorLine("起動に失敗しました: " + describeTokenError(err))
		return 1
	}
	ui.stepOK(2, 3, "トークン読込")
	log.Printf("[INFO] [2/3] トークン読込 ... OK")

	log.Printf("[INFO] [3/3] Bot初期化 (DB: %s) ... 開始", dbPath)

	b, err := newBotFn(dbPath)
	if err != nil {
		ui.stepFail(3, 3, "Bot初期化")
		log.Printf("[ERROR] Bot初期化に失敗: %v", err)
		ui.printErrorLine(fmt.Sprintf("起動に失敗しました: Bot初期化に失敗しました（DB: %s）: %v", dbPath, err))
		return 1
	}
	ui.stepOK(3, 3, "Bot初期化")
	log.Printf("[INFO] [3/3] Bot初期化 ... OK")
	enableConsoleLogging(logFile, stdout, ui.style.enabled)
	b.SetReadyNotifier(func() {
		ui.ready()
		log.Printf("[INFO] Discord との接続に成功しました。")
	})

	if err := b.Run(botToken); err != nil {
		if isAuthenticationFailureError(err) {
			log.Printf("[WARN] Bot認証に失敗: %v", err)
			if recoverErr := recoverFromAuthenticationFailure(ui, stdin, b, err); recoverErr != nil {
				log.Printf("[ERROR] 認証失敗からの復旧に失敗: %v", recoverErr)
				ui.printErrorLine("認証失敗から復旧できませんでした: " + describeAuthRecoveryError(recoverErr))
				return 1
			}
			log.Printf("[INFO] 認証失敗からの再試行で接続に成功しました。")
			log.Printf("[INFO] Botを終了しました。")
			return 0
		}
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
