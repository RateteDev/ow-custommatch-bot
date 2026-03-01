package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTempFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func TestLoadEnvFileSuccess(t *testing.T) {
	t.Setenv("BOT_TOKEN", "")
	dir := t.TempDir()
	path := writeTempFile(t, dir, ".env", "# comment\nBOT_TOKEN=test-token\n")

	if err := loadEnvFile(path); err != nil {
		t.Fatalf("loadEnvFile returned error: %v", err)
	}

	if got := os.Getenv("BOT_TOKEN"); got != "test-token" {
		t.Fatalf("unexpected BOT_TOKEN: %s", got)
	}
}

func TestLoadEnvFileErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		if err := loadEnvFile("does-not-exist.env"); err == nil {
			t.Fatalf("expected error for missing file")
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		dir := t.TempDir()
		path := writeTempFile(t, dir, ".env", "INVALID_LINE")
		if err := loadEnvFile(path); err == nil {
			t.Fatalf("expected error for invalid format")
		}
	})
}

func TestRequiredEnv(t *testing.T) {
	t.Setenv("BOT_TOKEN", "abc")
	v, err := requiredEnv("BOT_TOKEN")
	if err != nil {
		t.Fatalf("requiredEnv returned error: %v", err)
	}
	if v != "abc" {
		t.Fatalf("unexpected value: %s", v)
	}

	t.Setenv("BOT_TOKEN", "")
	if _, err := requiredEnv("BOT_TOKEN"); err == nil {
		t.Fatalf("expected error when env is empty")
	}

	t.Setenv("BOT_TOKEN", "YOUR_DISCORD_BOT_TOKEN")
	if _, err := requiredEnv("BOT_TOKEN"); err == nil {
		t.Fatalf("expected error when BOT_TOKEN is placeholder")
	}
}

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

	t.Run("プレースホルダー入力", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}
		if _, err := promptBotToken(ui, strings.NewReader("YOUR_DISCORD_BOT_TOKEN\n")); err == nil {
			t.Fatalf("expected error for placeholder input")
		}
	})

	t.Run("EOF", func(t *testing.T) {
		ui := startupUI{out: &bytes.Buffer{}}
		if _, err := promptBotToken(ui, strings.NewReader("")); err == nil {
			t.Fatalf("expected error for EOF input")
		}
	})
}

func TestSaveTokenToEnv(t *testing.T) {
	t.Run(".env なし", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".env")

		if err := saveTokenToEnv(path, "new-token"); err != nil {
			t.Fatalf("saveTokenToEnv returned error: %v", err)
		}

		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read env file: %v", err)
		}
		if string(body) != "BOT_TOKEN=new-token\n" {
			t.Fatalf("unexpected env content: %q", string(body))
		}
	})

	t.Run(".env あり BOT_TOKEN 行なし", func(t *testing.T) {
		dir := t.TempDir()
		path := writeTempFile(t, dir, ".env", "FOO=bar\n")

		if err := saveTokenToEnv(path, "appended-token"); err != nil {
			t.Fatalf("saveTokenToEnv returned error: %v", err)
		}

		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read env file: %v", err)
		}
		if string(body) != "FOO=bar\nBOT_TOKEN=appended-token\n" {
			t.Fatalf("unexpected env content: %q", string(body))
		}
	})

	t.Run(".env あり BOT_TOKEN 行あり", func(t *testing.T) {
		dir := t.TempDir()
		path := writeTempFile(t, dir, ".env", "BOT_TOKEN=YOUR_DISCORD_BOT_TOKEN\nFOO=bar\n")

		if err := saveTokenToEnv(path, "updated-token"); err != nil {
			t.Fatalf("saveTokenToEnv returned error: %v", err)
		}

		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read env file: %v", err)
		}
		if string(body) != "BOT_TOKEN=updated-token\nFOO=bar\n" {
			t.Fatalf("unexpected env content: %q", string(body))
		}
	})
}

func TestExecutableDir(t *testing.T) {
	dir, err := executableDir()
	if err != nil {
		t.Fatalf("executableDir returned error: %v", err)
	}
	if strings.TrimSpace(dir) == "" {
		t.Fatalf("executableDir returned empty dir")
	}
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
	for _, want := range []string{"使い方", "--help", "--version", "--test", ".env", "BOT_TOKEN"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage text missing %q: %s", want, text)
		}
	}
}

func TestPromptStartupMode(t *testing.T) {
	t.Run("test mode selected", func(t *testing.T) {
		var out bytes.Buffer
		ui := startupUI{out: &out}

		got := promptStartupMode(ui, strings.NewReader("2\n"), 50*time.Millisecond)
		if !got {
			t.Fatalf("expected test mode to be selected")
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

func TestDescribeStartupError(t *testing.T) {
	t.Run("missing env file", func(t *testing.T) {
		msg := describeStartupError(filepath.Join("C:\\bot", ".env"), "BOT_TOKEN", dbFileName, fmt.Errorf("open env file: %w", os.ErrNotExist))
		if !strings.Contains(msg, ".env") {
			t.Fatalf("expected .env hint: %s", msg)
		}
		if !strings.Contains(msg, "BOT_TOKEN=") {
			t.Fatalf("expected BOT_TOKEN example: %s", msg)
		}
		if !strings.Contains(msg, "使い方.html") {
			t.Fatalf("expected guide hint: %s", msg)
		}
	})

	t.Run("missing bot token", func(t *testing.T) {
		msg := describeStartupError("dummy.env", "BOT_TOKEN", dbFileName, requiredEnvErr("BOT_TOKEN"))
		if !strings.Contains(msg, "BOT_TOKEN") {
			t.Fatalf("expected BOT_TOKEN hint: %s", msg)
		}
		if !strings.Contains(msg, "YOUR_DISCORD_BOT_TOKEN") {
			t.Fatalf("expected placeholder hint: %s", msg)
		}
		if !strings.Contains(msg, "使い方.html") {
			t.Fatalf("expected guide hint: %s", msg)
		}
	})

	t.Run("invalid env format", func(t *testing.T) {
		msg := describeStartupError("dummy.env", "BOT_TOKEN", dbFileName, fmt.Errorf("invalid env format at line 1"))
		if !strings.Contains(msg, "書式") {
			t.Fatalf("expected format hint: %s", msg)
		}
		if !strings.Contains(msg, "KEY=VALUE") {
			t.Fatalf("expected KEY=VALUE hint: %s", msg)
		}
		if !strings.Contains(msg, "使い方.html") {
			t.Fatalf("expected guide hint: %s", msg)
		}
	})
}

func TestDetectColorEnabled(t *testing.T) {
	if detectColorEnabled(&bytes.Buffer{}) {
		t.Fatalf("expected color to be disabled for non-file writer")
	}
}

func TestStyleConsoleLogLine(t *testing.T) {
	line := "2026/02/25 20:00:00 [INFO] [2/4] 設定ファイル読込 ... OK\n"
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
	msg := "起動に失敗しました: 必須設定 BOT_TOKEN が未設定です。dummy.env に設定してください。`BOT_TOKEN=YOUR_DISCORD_BOT_TOKEN` のままでも未設定扱いです。\n詳しい手順は同じフォルダの 使い方.html をご確認ください。"
	got := formatErrorMessageText(msg)

	wants := []string{
		"未設定です。\n",
		"設定してください。\n",
		"未設定扱いです。\n",
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
