package bot

import (
	"path/filepath"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestNewSuccess(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "matchybot.db")

	b, err := New(dbPath)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if b.players == nil || b.recruitments == nil || b.testDummies == nil {
		t.Fatalf("New should initialize dependencies")
	}
}

func TestNewErrors(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nested", "matchybot.db")

	if _, err := New(dbPath); err == nil {
		t.Fatalf("expected error when db directory does not exist")
	}
}

func TestOnInteractionCreateIgnoresNonMenu(t *testing.T) {
	b := &Bot{}
	// should return immediately without panic for non-command interactions
	b.onInteractionCreate(nil, &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{Type: discordgo.InteractionMessageComponent}})
}
