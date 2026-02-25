package bot

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/RateteDev/MatchyBot/internal/model"
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

func TestIsRankRegistrationExpired(t *testing.T) {
	now := time.Date(2026, 2, 25, 12, 0, 0, 0, time.UTC)
	b := &Bot{
		now: func() time.Time { return now },
	}

	cases := []struct {
		name   string
		player *model.PlayerInfo
		want   bool
	}{
		{
			name: "29日なら有効",
			player: &model.PlayerInfo{
				HighestRank:   model.Rank{Rank: "gold", Division: "1"},
				RankUpdatedAt: now.Add(-29 * 24 * time.Hour).Format(time.RFC3339),
			},
			want: false,
		},
		{
			name: "30日ちょうどで期限切れ",
			player: &model.PlayerInfo{
				HighestRank:   model.Rank{Rank: "gold", Division: "1"},
				RankUpdatedAt: now.Add(-30 * 24 * time.Hour).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "31日で期限切れ",
			player: &model.PlayerInfo{
				HighestRank:   model.Rank{Rank: "gold", Division: "1"},
				RankUpdatedAt: now.Add(-31 * 24 * time.Hour).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "日時未保存は期限切れ",
			player: &model.PlayerInfo{
				HighestRank: model.Rank{Rank: "gold", Division: "1"},
			},
			want: true,
		},
		{
			name: "未登録は期限切れ扱いではない",
			player: &model.PlayerInfo{
				HighestRank: model.Rank{},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := b.isRankRegistrationExpired(tc.player)
			if got != tc.want {
				t.Fatalf("isRankRegistrationExpired() = %v, want %v", got, tc.want)
			}
		})
	}
}
