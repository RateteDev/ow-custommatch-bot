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
	rankPath := filepath.Join(dir, "rank.json")

	if err := os.WriteFile(rankPath, []byte(`{"ranks":{"GOLD":{"1":2500}}}`), 0o644); err != nil {
		t.Fatalf("failed to write rank data: %v", err)
	}

	b, err := New(playersPath, rankPath)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if b.players == nil || b.recruitment == nil {
		t.Fatalf("New should initialize dependencies")
	}
}

func TestNewErrors(t *testing.T) {
	dir := t.TempDir()
	rankPath := filepath.Join(dir, "rank.json")
	if err := os.WriteFile(rankPath, []byte(`{"ranks":{}}`), 0o644); err != nil {
		t.Fatalf("failed to write rank data: %v", err)
	}

	if _, err := New(filepath.Join(dir, "missing", "players.json"), rankPath); err == nil {
		t.Fatalf("expected error when player file directory does not exist")
	}

	playersPath := filepath.Join(dir, "players.json")
	if err := os.WriteFile(playersPath, []byte(`{"players":[]}`), 0o644); err != nil {
		t.Fatalf("failed to write players file: %v", err)
	}
	if _, err := New(playersPath, filepath.Join(dir, "missing-rank.json")); err == nil {
		t.Fatalf("expected error when rank file is missing")
	}
}

func TestOnInteractionCreateIgnoresNonMenu(t *testing.T) {
	b := &Bot{}
	// should return immediately without panic for non-command interactions
	b.onInteractionCreate(nil, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{Type: discordgo.InteractionMessageComponent}})
}
