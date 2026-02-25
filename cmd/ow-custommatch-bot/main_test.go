package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	t.Run("unknown flag", func(t *testing.T) {
		if _, err := parseCLIArgs([]string{"--unknown"}); err == nil {
			t.Fatalf("expected error for unknown flag")
		}
	})
}

func TestCLIUsageText(t *testing.T) {
	text := cliUsageText("ow-custommatch-bot")
	for _, want := range []string{"使い方", "--help", "--version", ".env", "BOT_TOKEN"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usage text missing %q: %s", want, text)
		}
	}
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
