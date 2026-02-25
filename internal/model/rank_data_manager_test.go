package model

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRankData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rank.json")
	body := `{"ranks":{"BRONZE":{"1":1000},"SILVER":{"1":1500}}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("failed to write rank file: %v", err)
	}

	r, err := LoadRankData(path)
	if err != nil {
		t.Fatalf("LoadRankData failed: %v", err)
	}

	if r.Ranks["BRONZE"]["1"] != 1000 {
		t.Fatalf("unexpected parsed value: %#v", r.Ranks)
	}
}

func TestLoadEmbeddedRankData(t *testing.T) {
	r, err := LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}
	if len(r.Ranks) == 0 {
		t.Fatalf("expected embedded rank data to be non-empty")
	}
	if r.Ranks["bronze"]["5"] == 0 {
		t.Fatalf("expected embedded rank data to contain bronze rank values")
	}
}

func TestLoadRankDataErrors(t *testing.T) {
	if _, err := LoadRankData("does-not-exist.json"); err == nil {
		t.Fatalf("expected error for missing rank file")
	}

	path := filepath.Join(t.TempDir(), "rank.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("failed to write invalid rank file: %v", err)
	}
	if _, err := LoadRankData(path); err == nil {
		t.Fatalf("expected error for invalid rank json")
	}
}
