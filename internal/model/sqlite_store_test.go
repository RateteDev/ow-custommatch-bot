package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewSQLiteStoreCreatesDBAndSchema(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "matchybot.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected db file to be created: %v", err)
	}

	count, err := store.PlayerCount()
	if err != nil {
		t.Fatalf("PlayerCount failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected empty players table, got %d", count)
	}
}

func TestSQLiteStoreUpsertPlayerAndReload(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "matchybot.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}

	if err := store.UpsertPlayer(PlayerInfo{
		ID:       "u1",
		Name:     "alice",
		MainRole: "tank",
		HighestRank: Rank{
			Rank:     "gold",
			Division: "2",
		},
	}); err != nil {
		t.Fatalf("UpsertPlayer (insert) failed: %v", err)
	}

	if err := store.UpsertPlayer(PlayerInfo{
		ID:       "u1",
		Name:     "alice2",
		MainRole: "support",
		HighestRank: Rank{
			Rank:     "platinum",
			Division: "4",
		},
	}); err != nil {
		t.Fatalf("UpsertPlayer (update) failed: %v", err)
	}

	got, err := store.GetPlayerByID("u1")
	if err != nil {
		t.Fatalf("GetPlayerByID failed: %v", err)
	}
	if got == nil {
		t.Fatalf("expected player to exist")
	}
	if got.Name != "alice2" || got.MainRole != "support" {
		t.Fatalf("unexpected player after update: %#v", got)
	}
	if got.HighestRank.Rank != "platinum" || got.HighestRank.Division != "4" {
		t.Fatalf("unexpected highest rank after update: %#v", got.HighestRank)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	reloaded, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("reload NewSQLiteStore failed: %v", err)
	}
	defer reloaded.Close()

	got, err = reloaded.GetPlayerByID("u1")
	if err != nil {
		t.Fatalf("GetPlayerByID after reload failed: %v", err)
	}
	if got == nil || got.Name != "alice2" {
		t.Fatalf("expected player to persist after reload, got %#v", got)
	}
}

func TestSQLiteStoreVCConfigRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "matchybot.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	defer store.Close()

	empty, err := store.LoadVCConfig()
	if err != nil {
		t.Fatalf("LoadVCConfig (empty) failed: %v", err)
	}
	if empty.VCChannelIDs == nil {
		t.Fatalf("expected empty VCChannelIDs slice, got nil")
	}
	if len(empty.VCChannelIDs) != 0 {
		t.Fatalf("expected empty VCChannelIDs, got %d", len(empty.VCChannelIDs))
	}

	want := VCConfig{
		CategoryID:   "cat-1",
		VCChannelIDs: []string{"vc-1", "vc-2"},
	}
	if err := store.SaveVCConfig(want); err != nil {
		t.Fatalf("SaveVCConfig failed: %v", err)
	}

	got, err := store.LoadVCConfig()
	if err != nil {
		t.Fatalf("LoadVCConfig failed: %v", err)
	}
	if got.CategoryID != want.CategoryID {
		t.Fatalf("unexpected category id: %s", got.CategoryID)
	}
	if len(got.VCChannelIDs) != 2 || got.VCChannelIDs[0] != "vc-1" || got.VCChannelIDs[1] != "vc-2" {
		t.Fatalf("unexpected VCChannelIDs: %#v", got.VCChannelIDs)
	}
}
