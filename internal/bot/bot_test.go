package bot

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RateteDev/ow-custommatch-bot/internal/model"
	"github.com/bwmarrin/discordgo"
)

func TestNewSuccess(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ow-custommatch-bot.db")

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
	dbPath := filepath.Join(dir, "nested", "ow-custommatch-bot.db")

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

func TestBuildRecruitEmbedEntryThresholdLabel(t *testing.T) {
	t.Run("9人なら10人以上必要を表示", func(t *testing.T) {
		r := &model.Recruitment{
			Entries: make([]model.Entry, 9),
		}

		embed := buildRecruitEmbed(r, "title", 0)
		if len(embed.Fields) == 0 {
			t.Fatalf("embed.Fields should not be empty")
		}
		if !strings.Contains(embed.Fields[0].Name, "10人以上必要") {
			t.Fatalf("field name = %q, want to contain %q", embed.Fields[0].Name, "10人以上必要")
		}
	})

	t.Run("10人なら振り分け可能を表示", func(t *testing.T) {
		r := &model.Recruitment{
			Entries: make([]model.Entry, 10),
		}

		embed := buildRecruitEmbed(r, "title", 0)
		if len(embed.Fields) == 0 {
			t.Fatalf("embed.Fields should not be empty")
		}
		if !strings.Contains(embed.Fields[0].Name, "振り分け可能") {
			t.Fatalf("field name = %q, want to contain %q", embed.Fields[0].Name, "振り分け可能")
		}
	})
}

func TestBuildRecruitComponentsAssignButtonDisabledByEntryCount(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	b := &Bot{rankData: rankData}

	t.Run("9人ならassignボタンは無効", func(t *testing.T) {
		r := model.NewRecruitment(rankData)
		r.Entries = make([]model.Entry, 9)

		components := b.buildRecruitComponents(r, false)
		assignButton := findAssignButton(t, components)
		if !assignButton.Disabled {
			t.Fatalf("assign button should be disabled when entries are less than 10")
		}
	})

	t.Run("10人ならassignボタンは有効", func(t *testing.T) {
		r := model.NewRecruitment(rankData)
		r.Entries = make([]model.Entry, 10)

		components := b.buildRecruitComponents(r, false)
		assignButton := findAssignButton(t, components)
		if assignButton.Disabled {
			t.Fatalf("assign button should be enabled when entries are 10 or more")
		}
	})
}

func TestTeamAverageScore(t *testing.T) {
	t.Run("空チームは0", func(t *testing.T) {
		if got := teamAverageScore(nil); got != 0 {
			t.Fatalf("teamAverageScore(nil) = %v, want 0", got)
		}
	})

	t.Run("平均値を返す", func(t *testing.T) {
		team := []model.ScoredPlayer{
			{Score: 3000},
			{Score: 2000},
		}
		if got := teamAverageScore(team); got != 2500 {
			t.Fatalf("teamAverageScore() = %v, want 2500", got)
		}
	})
}

func findAssignButton(t *testing.T, components []discordgo.MessageComponent) discordgo.Button {
	t.Helper()

	for _, component := range components {
		row, ok := component.(discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, child := range row.Components {
			button, ok := child.(discordgo.Button)
			if ok && button.CustomID == "assign" {
				return button
			}
		}
	}

	t.Fatalf("assign button not found")
	return discordgo.Button{}
}
