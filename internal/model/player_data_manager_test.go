package model

import (
	"path/filepath"
	"testing"
)

func TestNewPlayerDataManagerCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "players.json")

	mgr, err := NewPlayerDataManager(path)
	if err != nil {
		t.Fatalf("NewPlayerDataManager failed: %v", err)
	}

	if len(mgr.Data.Players) != 0 {
		t.Fatalf("expected empty players list, got %d", len(mgr.Data.Players))
	}
}

func TestPlayerDataManagerAddAndGetByID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "players.json")
	mgr, err := NewPlayerDataManager(path)
	if err != nil {
		t.Fatalf("NewPlayerDataManager failed: %v", err)
	}

	player := PlayerInfo{ID: "u1", Name: "alice"}
	if err := mgr.Add(player); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	got := mgr.GetByID("u1")
	if got == nil || got.Name != "alice" {
		t.Fatalf("unexpected player: %#v", got)
	}

	reloaded, err := NewPlayerDataManager(path)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if reloaded.GetByID("u1") == nil {
		t.Fatalf("expected player to persist after reload")
	}
}
