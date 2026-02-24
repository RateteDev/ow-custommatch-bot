package model

import "testing"

func testRankData() RankDataFile {
	return RankDataFile{Ranks: RankTable{
		"GOLD":   {"1": 2500},
		"SILVER": {"1": 2000},
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

	if got := r.CalculatePlayerScore(Rank{Rank: "TOP500"}); got != 4500 {
		t.Fatalf("TOP500 score mismatch: %v", got)
	}
	if got := r.CalculatePlayerScore(Rank{Rank: "GOLD", Division: "1"}); got != 2500 {
		t.Fatalf("rank score mismatch: %v", got)
	}
	if got := r.CalculatePlayerScore(Rank{Rank: "UNKNOWN", Division: "1"}); got != 0 {
		t.Fatalf("unknown rank should be 0, got %v", got)
	}
}

func TestMakeTeams(t *testing.T) {
	r := NewRecruitment(testRankData())

	if teams := r.MakeTeams([]ScoredPlayer{{ID: "1", Score: 1000}}); teams != nil {
		t.Fatalf("expected nil when fewer than 5 players")
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
