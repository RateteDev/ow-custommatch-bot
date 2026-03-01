package main

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPromptBotToken(t *testing.T) {
	t.Run("正常入力", func(t *testing.T) {
		var out bytes.Buffer
		ui := startupUI{out: &out}

		got, err := promptBotToken(ui, strings.NewReader("mytoken\n"))
		if err != nil {
			t.Fatalf("promptBotToken returned error: %v", err)
		}
		if got != "mytoken" {
			t.Fatalf("promptBotToken = %q, want %q", got, "mytoken")
		}
		if !strings.Contains(out.String(), "BOT_TOKEN を入力してください") {
			t.Fatalf("prompt was not written: %q", out.String())
		}
	})

	t.Run("空入力", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}
		if _, err := promptBotToken(ui, strings.NewReader("\n")); err == nil {
			t.Fatalf("expected error for empty input")
		}
	})

	t.Run("EOF", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}
		if _, err := promptBotToken(ui, strings.NewReader("")); err == nil {
			t.Fatalf("expected error for EOF input")
		}
	})
}

func TestAppDataDir(t *testing.T) {
	origGOOS := runtimeGOOS
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
	})

	t.Run("windows with LOCALAPPDATA", func(t *testing.T) {
		runtimeGOOS = "windows"
		t.Setenv("LOCALAPPDATA", `C:\Users\tester\AppData\Local`)

		got, err := appDataDir(appName)
		if err != nil {
			t.Fatalf("appDataDir returned error: %v", err)
		}
		want := filepath.Join(`C:\Users\tester\AppData\Local`, appName)
		if got != want {
			t.Fatalf("appDataDir = %q, want %q", got, want)
		}
	})

	t.Run("windows without LOCALAPPDATA", func(t *testing.T) {
		runtimeGOOS = "windows"
		t.Setenv("LOCALAPPDATA", "")

		if _, err := appDataDir(appName); err == nil {
			t.Fatalf("expected error when LOCALAPPDATA is empty")
		}
	})

	t.Run("non-windows", func(t *testing.T) {
		runtimeGOOS = "linux"
		home := t.TempDir()
		t.Setenv("HOME", home)

		got, err := appDataDir(appName)
		if err != nil {
			t.Fatalf("appDataDir returned error: %v", err)
		}
		want := filepath.Join(home, ".local", "share", appName)
		if got != want {
			t.Fatalf("appDataDir = %q, want %q", got, want)
		}
	})
}

func TestParseCLIArgs(t *testing.T) {
	t.Run("help", func(t *testing.T) {
		opts, err := parseCLIArgs([]string{"--help"})
		if err != nil {
			t.Fatalf("parseCLIArgs returned error: %v", err)
		}
		if !opts.showHelp {
			t.Fatalf("expected showHelp to be true")
		}
	})

	t.Run("version", func(t *testing.T) {
		opts, err := parseCLIArgs([]string{"--version"})
		if err != nil {
			t.Fatalf("parseCLIArgs returned error: %v", err)
		}
		if !opts.showVersion {
			t.Fatalf("expected showVersion to be true")
		}
	})

	t.Run("test flag", func(t *testing.T) {
		opts, err := parseCLIArgs([]string{"--test"})
		if err != nil {
			t.Fatalf("parseCLIArgs returned error: %v", err)
		}
		if !opts.testMode {
			t.Fatalf("expected testMode to be true")
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		if _, err := parseCLIArgs([]string{"--unknown"}); err == nil {
			t.Fatalf("expected error for unknown flag")
		}
	})
}

func TestCLIUsageText(t *testing.T) {
	text := cliUsageText("ow-custommatch-bot")
	for _, want := range []string{"使い方", "--help", "--version", "--test", "BOT_TOKEN", "Credential Manager", guideURL} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage text missing %q: %s", want, text)
		}
	}
}

func TestTokenResolution(t *testing.T) {
	origRead := readTokenFromStoreFn
	origSave := saveTokenToStoreFn
	origConsole := hasInteractiveConsole
	t.Cleanup(func() {
		readTokenFromStoreFn = origRead
		saveTokenToStoreFn = origSave
		hasInteractiveConsole = origConsole
	})

	t.Run("store hit", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "stored-token", nil
		}
		saveTokenToStoreFn = func(token string) error {
			t.Fatalf("save should not be called, got %q", token)
			return nil
		}
		hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
			return false
		}

		got, err := resolveToken(strings.NewReader(""), startupUI{out: &bytes.Buffer{}})
		if err != nil {
			t.Fatalf("resolveToken returned error: %v", err)
		}
		if got != "stored-token" {
			t.Fatalf("resolveToken = %q, want %q", got, "stored-token")
		}
	})

	t.Run("store miss prompts and saves", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "", errTokenNotFound
		}
		saved := ""
		saveTokenToStoreFn = func(token string) error {
			saved = token
			return nil
		}
		hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
			return true
		}

		got, err := resolveToken(strings.NewReader("prompted-token\n"), startupUI{out: &bytes.Buffer{}})
		if err != nil {
			t.Fatalf("resolveToken returned error: %v", err)
		}
		if got != "prompted-token" {
			t.Fatalf("resolveToken = %q, want %q", got, "prompted-token")
		}
		if saved != "prompted-token" {
			t.Fatalf("saved token = %q, want %q", saved, "prompted-token")
		}
	})

	t.Run("non interactive store miss", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "", errTokenNotFound
		}
		saveTokenToStoreFn = func(token string) error {
			t.Fatalf("save should not be called, got %q", token)
			return nil
		}
		hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
			return false
		}

		if _, err := resolveToken(strings.NewReader(""), startupUI{out: &bytes.Buffer{}}); !errors.Is(err, errTokenNotFound) {
			t.Fatalf("resolveToken error = %v, want errTokenNotFound", err)
		}
	})

	t.Run("save failure", func(t *testing.T) {
		readTokenFromStoreFn = func() (string, error) {
			return "", errTokenNotFound
		}
		saveTokenToStoreFn = func(token string) error {
			return errTokenStoreUnsupported
		}
		hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
			return true
		}

		if _, err := resolveToken(strings.NewReader("prompted-token\n"), startupUI{out: &bytes.Buffer{}}); err == nil || !strings.Contains(err.Error(), "保存") {
			t.Fatalf("expected save error, got %v", err)
		}
	})
}

func TestPromptStartupMode(t *testing.T) {
	t.Run("test mode selected", func(t *testing.T) {
		var out bytes.Buffer
		ui := startupUI{out: &out}

		got := promptStartupMode(ui, strings.NewReader("2\n"), 50*time.Millisecond)
		if !got {
			t.Fatalf("expected test mode to be selected")
		}
		output := out.String()
		for _, want := range []string{"通常運用", "動作確認用", "Enterキーですぐに通常運用", "自動で通常運用を開始します"} {
			if !strings.Contains(output, want) {
				t.Fatalf("prompt output missing %q: %q", want, output)
			}
		}
		for _, unwanted := range []string{"本番モード", "テストモード"} {
			if strings.Contains(output, unwanted) {
				t.Fatalf("prompt output should not contain %q: %q", unwanted, output)
			}
		}
	})

	t.Run("production mode selected", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}

		got := promptStartupMode(ui, strings.NewReader("1\n"), 50*time.Millisecond)
		if got {
			t.Fatalf("expected production mode to be selected")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}

		got := promptStartupMode(ui, strings.NewReader("\n"), 50*time.Millisecond)
		if got {
			t.Fatalf("expected empty input to select production mode")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}
		reader, writer := io.Pipe()
		defer reader.Close()
		defer writer.Close()

		got := promptStartupMode(ui, reader, time.Millisecond)
		if got {
			t.Fatalf("expected timeout to select production mode")
		}
	})
}

func TestStartupUIPrintBanner(t *testing.T) {
	var out bytes.Buffer
	ui := startupUI{out: &out}

	ui.printBanner("1.2.3")

	got := out.String()
	for _, want := range []string{"Overwatch Custom Match Assistant", "v1.2.3", guideURL} {
		if !strings.Contains(got, want) {
			t.Fatalf("banner output missing %q: %q", want, got)
		}
	}
}

func TestStartupUIPrintPaths(t *testing.T) {
	var out bytes.Buffer
	ui := startupUI{out: &out}

	ui.printPaths("/var/lib/owcmb", "/var/log/owcmb.log", "/var/lib/owcmb/app.sqlite")

	got := out.String()
	for _, want := range []string{"データ保存先", "ログファイル", "データベース"} {
		if !strings.Contains(got, want) {
			t.Fatalf("paths output missing %q: %q", want, got)
		}
	}
	for _, unwanted := range []string{"\ndata ", "\nlog ", "\ndb "} {
		if strings.Contains("\n"+got, unwanted) {
			t.Fatalf("paths output should not contain %q label: %q", unwanted, got)
		}
	}
}

func TestStartupUIReady(t *testing.T) {
	var out bytes.Buffer
	ui := startupUI{out: &out}

	ui.ready()

	got := out.String()
	for _, want := range []string{"準備完了", "Discordへの接続を開始します", "Ctrl+C"} {
		if !strings.Contains(got, want) {
			t.Fatalf("ready output missing %q: %q", want, got)
		}
	}
}

func TestStartupModeConfirmationMessage(t *testing.T) {
	tests := []struct {
		name string
		mode bool
		want string
	}{
		{name: "prod", mode: false, want: "PROD 通常運用で起動します。"},
		{name: "test", mode: true, want: "TEST 動作確認用で起動します。"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := startupModeConfirmationMessage(tt.mode); got != tt.want {
				t.Fatalf("startupModeConfirmationMessage(%v) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestDescribeTokenError(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		msg := describeTokenError(errTokenNotFound)
		if !strings.Contains(msg, "BOT_TOKEN") {
			t.Fatalf("expected BOT_TOKEN hint: %s", msg)
		}
		if !strings.Contains(msg, guideURL) {
			t.Fatalf("expected guide URL hint: %s", msg)
		}
	})

	t.Run("unsupported store", func(t *testing.T) {
		msg := describeTokenError(errTokenStoreUnsupported)
		if !strings.Contains(msg, "OS") {
			t.Fatalf("expected OS hint: %s", msg)
		}
		if !strings.Contains(msg, guideURL) {
			t.Fatalf("expected guide URL hint: %s", msg)
		}
	})
}

func TestDetectColorEnabled(t *testing.T) {
	if detectColorEnabled(&bytes.Buffer{}) {
		t.Fatalf("expected color to be disabled for non-file writer")
	}
}

func TestStyleConsoleLogLine(t *testing.T) {
	line := "2026/02/25 20:00:00 [INFO] [2/3] トークン読込 ... OK\n"
	styled := styleConsoleLogLine(line, ansiStyle{enabled: true})
	if !strings.Contains(styled, "\x1b[") {
		t.Fatalf("expected ANSI escape sequence in styled line: %q", styled)
	}

	plain := styleConsoleLogLine(line, ansiStyle{enabled: false})
	if plain != line {
		t.Fatalf("expected plain line unchanged")
	}
}

func TestFormatErrorMessageText(t *testing.T) {
	msg := "起動に失敗しました: BOT_TOKEN が保存されていません。初回起動時は対話入力で保存してください。詳しい手順は https://ratetedev.github.io/ow-custommatch-bot/ をご確認ください。"
	got := formatErrorMessageText(msg)

	wants := []string{
		"保存されていません。\n",
		"保存してください。\n",
		"ご確認ください。",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted message missing %q: %q", want, got)
		}
	}
	if strings.Contains(got, "\n\n") {
		t.Fatalf("formatted message should not contain double newlines: %q", got)
	}
}

func TestStartupUIPrintErrorLineFormatsMessage(t *testing.T) {
	var errOut bytes.Buffer
	ui := newStartupUI(&bytes.Buffer{}, &errOut)

	ui.printErrorLine("Aです。Bです。")

	got := errOut.String()
	if !strings.Contains(got, "Aです。\nBです。") {
		t.Fatalf("unexpected error output: %q", got)
	}
}

func TestIsProgressToken(t *testing.T) {
	for _, tc := range []struct {
		token string
		want  bool
	}{
		{token: "[2/4]", want: true},
		{token: "[INFO]", want: false},
		{token: "[2m", want: false},
		{token: "[abc/4]", want: false},
	} {
		if got := isProgressToken(tc.token); got != tc.want {
			t.Fatalf("isProgressToken(%q) = %v, want %v", tc.token, got, tc.want)
		}
	}
}

func TestShouldPauseOnErrorExit(t *testing.T) {
	origGOOS := runtimeGOOS
	origConsole := hasInteractiveConsole
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		hasInteractiveConsole = origConsole
	})

	runtimeGOOS = "windows"
	hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
		return true
	}

	if !shouldPauseOnErrorExit(1, bytes.NewBuffer(nil), &bytes.Buffer{}) {
		t.Fatalf("expected pause on windows error exit with interactive console")
	}
	if shouldPauseOnErrorExit(0, bytes.NewBuffer(nil), &bytes.Buffer{}) {
		t.Fatalf("did not expect pause on success exit")
	}

	runtimeGOOS = "linux"
	if shouldPauseOnErrorExit(1, bytes.NewBuffer(nil), &bytes.Buffer{}) {
		t.Fatalf("did not expect pause on non-windows")
	}
}

func TestPauseOnErrorExit(t *testing.T) {
	origGOOS := runtimeGOOS
	origConsole := hasInteractiveConsole
	t.Cleanup(func() {
		runtimeGOOS = origGOOS
		hasInteractiveConsole = origConsole
	})

	runtimeGOOS = "windows"
	hasInteractiveConsole = func(stdin io.Reader, stdout io.Writer) bool {
		return true
	}

	var out bytes.Buffer
	pauseOnErrorExit(1, bytes.NewBufferString("\n"), &out)
	if !strings.Contains(out.String(), "Enterキー") {
		t.Fatalf("expected pause message, got: %q", out.String())
	}

	out.Reset()
	pauseOnErrorExit(0, bytes.NewBufferString("\n"), &out)
	if out.Len() != 0 {
		t.Fatalf("did not expect output on success exit: %q", out.String())
	}
}
