package bot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestNewSuccess(t *testing.T) {
	dir := t.TempDir()
	playersPath := filepath.Join(dir, "players.json")
	vcConfigPath := filepath.Join(dir, "vc_config.json")

	b, err := New(playersPath, vcConfigPath)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if b.players == nil || b.recruitments == nil || b.testDummies == nil {
		t.Fatalf("New should initialize dependencies")
	}
}

func TestNewErrors(t *testing.T) {
	dir := t.TempDir()
	vcConfigPath := filepath.Join(dir, "vc_config.json")

	if _, err := New(filepath.Join(dir, "missing", "players.json"), vcConfigPath); err == nil {
		t.Fatalf("expected error when player file directory does not exist")
	}

	playersPath := filepath.Join(dir, "players.json")
	if err := os.WriteFile(playersPath, []byte(`{`), 0o644); err != nil {
		t.Fatalf("failed to write players file: %v", err)
	}
	if _, err := New(playersPath, vcConfigPath); err == nil {
		t.Fatalf("expected error when players file is invalid json")
	}
}

func TestOnInteractionCreateIgnoresNonMenu(t *testing.T) {
	b := &Bot{}
	// should return immediately without panic for non-command interactions
	b.onInteractionCreate(nil, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{Type: discordgo.InteractionMessageComponent}})
}
