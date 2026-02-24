package main

import (
	"os"
	"path/filepath"
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

func TestLoadConfigSuccess(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "config.json", `{
		"bot_token":"token",
		"player_data_path":"player_data.json",
		"rank_data_path":"rank.json"
	}`)

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if cfg.BotToken != "token" || cfg.PlayerDataPath != "player_data.json" || cfg.RankDataPath != "rank.json" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadConfigValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing bot token", body: `{"player_data_path":"a","rank_data_path":"b"}`},
		{name: "missing player data", body: `{"bot_token":"t","rank_data_path":"b"}`},
		{name: "missing rank data", body: `{"bot_token":"t","player_data_path":"a"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeTempFile(t, dir, "config.json", tc.body)
			if _, err := loadConfig(path); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func TestLoadConfigFileErrors(t *testing.T) {
	if _, err := loadConfig("does-not-exist.json"); err == nil {
		t.Fatalf("expected error for missing file")
	}

	dir := t.TempDir()
	path := writeTempFile(t, dir, "config.json", `{invalid`)
	if _, err := loadConfig(path); err == nil {
		t.Fatalf("expected error for invalid json")
	}
}

func TestResolvePath(t *testing.T) {
	base := "/opt/matchybot"
	if got := resolvePath(base, "player_data.json"); got != "/opt/matchybot/player_data.json" {
		t.Fatalf("unexpected relative resolution: %s", got)
	}
	if got := resolvePath(base, "/var/lib/rank.json"); got != "/var/lib/rank.json" {
		t.Fatalf("absolute path should stay unchanged: %s", got)
	}
}
