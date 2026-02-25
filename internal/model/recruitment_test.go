package model

import "testing"

func testRankData() RankDataFile {
	return RankDataFile{Ranks: RankTable{
		"gold":   {"1": 2500},
		"silver": {"1": 2000},
	}}
}

func TestAddEntry(t *testing.T) {
	r := NewRecruitment(testRankData())
	if !r.AddEntry("u1", "alice") {
		t.Fatalf("first add should succeed")
	}
	if r.AddEntry("u1", "alice") {
		t.Fatalf("duplicate add should fail")
	}
}

func TestCalculatePlayerScore(t *testing.T) {
	r := NewRecruitment(testRankData())

	if got := r.CalculatePlayerScore(Rank{Rank: "top500"}); got != 4500 {
		t.Fatalf("TOP500 score mismatch: %v", got)
	}
	if got := r.CalculatePlayerScore(Rank{Rank: "gold", Division: "1"}); got != 2500 {
		t.Fatalf("rank score mismatch: %v", got)
	}
	if got := r.CalculatePlayerScore(Rank{Rank: "unknown", Division: "1"}); got != 0 {
		t.Fatalf("unknown rank should be 0, got %v", got)
	}
}

func TestMakeTeams(t *testing.T) {
	r := NewRecruitment(testRankData())

	ninePlayerSlice := []ScoredPlayer{
		{ID: "1", Score: 1000}, {ID: "2", Score: 1100}, {ID: "3", Score: 1200},
		{ID: "4", Score: 1300}, {ID: "5", Score: 1400}, {ID: "6", Score: 1500},
		{ID: "7", Score: 1600}, {ID: "8", Score: 1700}, {ID: "9", Score: 1800},
	}
	if teams := r.MakeTeams(ninePlayerSlice); teams != nil {
		t.Fatalf("expected nil for 9 players (< 10)")
	}

	players := []ScoredPlayer{
		{ID: "1", Score: 1000}, {ID: "2", Score: 1100}, {ID: "3", Score: 1200}, {ID: "4", Score: 1300}, {ID: "5", Score: 1400},
		{ID: "6", Score: 1500}, {ID: "7", Score: 1600}, {ID: "8", Score: 1700}, {ID: "9", Score: 1800}, {ID: "10", Score: 1900},
	}
	teams := r.MakeTeams(players)
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(teams))
	}

	seen := map[string]bool{}
	for i, team := range teams {
		if len(team) != 5 {
			t.Fatalf("team %d should contain 5 players, got %d", i, len(team))
		}
		for _, p := range team {
			if seen[p.ID] {
				t.Fatalf("duplicate player assigned: %s", p.ID)
			}
			seen[p.ID] = true
		}
	}
	if len(seen) != 10 {
		t.Fatalf("expected all players to be assigned once, got %d", len(seen))
	}
}

func TestMakeTeamsWithRemainder(t *testing.T) {
	r := NewRecruitment(testRankData())

	players := make([]ScoredPlayer, 0, 11)
	for i := 1; i <= 11; i++ {
		players = append(players, ScoredPlayer{
			ID:    string(rune('a' + i - 1)),
			Score: float64(1000 + i),
		})
	}

	teams, remainder := r.MakeTeamsWithRemainder(players)
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(teams))
	}
	if len(remainder) != 1 {
		t.Fatalf("expected 1 remainder player, got %d", len(remainder))
	}

	seen := map[string]bool{}
	for _, team := range teams {
		for _, p := range team {
			if seen[p.ID] {
				t.Fatalf("duplicate player assigned in teams: %s", p.ID)
			}
			seen[p.ID] = true
		}
	}
	for _, p := range remainder {
		if seen[p.ID] {
			t.Fatalf("remainder player also assigned to team: %s", p.ID)
		}
		seen[p.ID] = true
	}
	if len(seen) != 11 {
		t.Fatalf("expected all 11 players to be accounted for, got %d", len(seen))
	}
}

func TestRemoveEntry_existing(t *testing.T) {
	r := NewRecruitment(RankDataFile{})
	r.AddEntry("u1", "UserOne")
	r.AddEntry("u2", "UserTwo")

	got := r.RemoveEntry("u1")

	if !got {
		t.Errorf("RemoveEntry(existing) = false, want true")
	}
	if len(r.Entries) != 1 {
		t.Errorf("len(Entries) = %d, want 1", len(r.Entries))
	}
	if r.Entries[0].UserID != "u2" {
		t.Errorf("remaining entry = %s, want u2", r.Entries[0].UserID)
	}
}

func TestRemoveEntry_notExisting(t *testing.T) {
	r := NewRecruitment(RankDataFile{})
	r.AddEntry("u1", "UserOne")

	got := r.RemoveEntry("unknown")

	if got {
		t.Errorf("RemoveEntry(not existing) = true, want false")
	}
	if len(r.Entries) != 1 {
		t.Errorf("len(Entries) = %d, want 1", len(r.Entries))
	}
}
