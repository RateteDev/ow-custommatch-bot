package main

import (
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
