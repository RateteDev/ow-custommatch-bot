package bot

import (
	"path/filepath"
	"strings"
	"sync"
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

func TestStartRecruitmentKeepsStatePerChannelWithinSameGuild(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	b := &Bot{
		rankData:     rankData,
		recruitments: make(map[string]*model.Recruitment),
		testDummies:  make(map[string]map[string]model.PlayerInfo),
	}

	first, started := b.startRecruitment("guild-1", "channel-1", "user-1")
	if !started {
		t.Fatalf("first recruitment should start")
	}
	second, started := b.startRecruitment("guild-1", "channel-2", "user-2")
	if !started {
		t.Fatalf("second recruitment in another channel should start")
	}

	if first == second {
		t.Fatalf("recruitments in different channels should be distinct")
	}

	gotFirst, ok := b.getRecruitment("channel-1")
	if !ok || gotFirst != first {
		t.Fatalf("channel-1 recruitment = %v, %v; want first recruitment", gotFirst, ok)
	}
	gotSecond, ok := b.getRecruitment("channel-2")
	if !ok || gotSecond != second {
		t.Fatalf("channel-2 recruitment = %v, %v; want second recruitment", gotSecond, ok)
	}

	if gotFirst.GuildID != "guild-1" || gotSecond.GuildID != "guild-1" {
		t.Fatalf("guild ids should be preserved: first=%q second=%q", gotFirst.GuildID, gotSecond.GuildID)
	}
}

func TestStartRecruitmentPreventsDuplicateOpenRecruitmentInSameChannel(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	b := &Bot{
		rankData:     rankData,
		recruitments: make(map[string]*model.Recruitment),
		testDummies:  make(map[string]map[string]model.PlayerInfo),
	}

	first, started := b.startRecruitment("guild-1", "channel-1", "user-1")
	if !started {
		t.Fatalf("initial recruitment should start")
	}

	second, started := b.startRecruitment("guild-1", "channel-1", "user-2")
	if started {
		t.Fatalf("duplicate recruitment in same channel should not start")
	}
	if second != first {
		t.Fatalf("duplicate start should return existing recruitment")
	}

	got, ok := b.getRecruitment("channel-1")
	if !ok {
		t.Fatalf("channel-1 recruitment should still exist")
	}
	if got.OrganizerID != "user-1" {
		t.Fatalf("organizer should remain unchanged, got %q", got.OrganizerID)
	}
}

func TestStartRecruitmentIsAtomicPerChannel(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	b := &Bot{
		rankData:     rankData,
		recruitments: make(map[string]*model.Recruitment),
		testDummies:  make(map[string]map[string]model.PlayerInfo),
	}

	const workers = 8
	var wg sync.WaitGroup
	startedCount := 0
	var startedMu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, started := b.startRecruitment("guild-1", "channel-1", teamLabel(i)); started {
				startedMu.Lock()
				startedCount++
				startedMu.Unlock()
			}
		}(i)
	}
	wg.Wait()

	if startedCount != 1 {
		t.Fatalf("startedCount = %d, want 1", startedCount)
	}

	got, ok := b.getRecruitment("channel-1")
	if !ok || got == nil || !got.IsOpen {
		t.Fatalf("channel-1 recruitment should exist and stay open")
	}
}

func TestRecruitmentAccessorsSetGetDelete(t *testing.T) {
	b := &Bot{
		recruitments: make(map[string]*model.Recruitment),
	}
	r := &model.Recruitment{ChannelID: "channel-1", IsOpen: true}

	b.setRecruitment("channel-1", r)

	got, ok := b.getRecruitment("channel-1")
	if !ok || got != r {
		t.Fatalf("getRecruitment() = %v, %v; want %v, true", got, ok, r)
	}

	b.deleteRecruitment("channel-1")

	if got, ok := b.getRecruitment("channel-1"); ok || got != nil {
		t.Fatalf("after delete getRecruitment() = %v, %v; want nil, false", got, ok)
	}
}

func TestPrepareMatchRestartRequest(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	b := &Bot{
		rankData:             rankData,
		recruitments:         make(map[string]*model.Recruitment),
		pendingMatchRestarts: make(map[string]pendingMatchRestart),
		testDummies:          make(map[string]map[string]model.PlayerInfo),
		now:                  func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) },
	}
	current, started := b.startRecruitment("guild-1", "channel-1", "organizer-1")
	if !started {
		t.Fatalf("initial recruitment should start")
	}
	current.MessageID = "message-1"

	req, ok := b.prepareMatchRestartRequest("guild-1", "channel-1", "organizer-2", true)
	if !ok {
		t.Fatalf("prepareMatchRestartRequest should succeed")
	}
	if req.token == "" {
		t.Fatalf("restart request token should not be empty")
	}
	if !req.fillWithDummies {
		t.Fatalf("fillWithDummies should be preserved")
	}
	stored, ok := b.getPendingMatchRestart(req.token)
	if !ok {
		t.Fatalf("pending restart should be stored")
	}
	if stored.recruitmentMessageID != "message-1" {
		t.Fatalf("stored recruitment message id = %q, want %q", stored.recruitmentMessageID, "message-1")
	}
	if stored.requesterID != "organizer-2" {
		t.Fatalf("stored requester id = %q, want %q", stored.requesterID, "organizer-2")
	}
}

func TestConfirmMatchRestart(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	newBot := func() *Bot {
		return &Bot{
			rankData:             rankData,
			recruitments:         make(map[string]*model.Recruitment),
			pendingMatchRestarts: make(map[string]pendingMatchRestart),
			testDummies:          make(map[string]map[string]model.PlayerInfo),
			now:                  func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) },
		}
	}

	t.Run("確認した本人だけが既存募集を閉じて開始できる", func(t *testing.T) {
		b := newBot()
		current, started := b.startRecruitment("guild-1", "channel-1", "organizer-1")
		if !started {
			t.Fatalf("initial recruitment should start")
		}
		current.MessageID = "message-1"
		current.AddEntry("player-1", "Player 1")

		req, ok := b.prepareMatchRestartRequest("guild-1", "channel-1", "organizer-1", false)
		if !ok {
			t.Fatalf("prepareMatchRestartRequest should succeed")
		}

		_, oldRecruitment, newRecruitment, err := b.confirmMatchRestart(req.token, "organizer-1")
		if err != nil {
			t.Fatalf("confirmMatchRestart returned error: %v", err)
		}
		if oldRecruitment != current {
			t.Fatalf("old recruitment = %v, want %v", oldRecruitment, current)
		}
		if oldRecruitment.IsOpen {
			t.Fatalf("old recruitment should be closed")
		}
		if newRecruitment == nil || !newRecruitment.IsOpen {
			t.Fatalf("new recruitment should be open")
		}
		if len(newRecruitment.Entries) != 0 {
			t.Fatalf("new recruitment should not inherit entries, got %d", len(newRecruitment.Entries))
		}
		if newRecruitment == oldRecruitment {
			t.Fatalf("new recruitment should be distinct from old recruitment")
		}
		got, ok := b.getRecruitment("channel-1")
		if !ok || got != newRecruitment {
			t.Fatalf("current recruitment = %v, %v; want new recruitment", got, ok)
		}
		if _, ok := b.getPendingMatchRestart(req.token); ok {
			t.Fatalf("pending restart should be removed after confirm")
		}
	})

	t.Run("本人以外の操作は拒否する", func(t *testing.T) {
		b := newBot()
		current, started := b.startRecruitment("guild-1", "channel-1", "organizer-1")
		if !started {
			t.Fatalf("initial recruitment should start")
		}
		current.MessageID = "message-1"
		req, ok := b.prepareMatchRestartRequest("guild-1", "channel-1", "organizer-1", false)
		if !ok {
			t.Fatalf("prepareMatchRestartRequest should succeed")
		}

		_, _, _, err := b.confirmMatchRestart(req.token, "other-user")
		if err == nil || err != errMatchRestartNotRequester {
			t.Fatalf("confirmMatchRestart error = %v, want %v", err, errMatchRestartNotRequester)
		}
		got, ok := b.getRecruitment("channel-1")
		if !ok || got != current || !got.IsOpen {
			t.Fatalf("recruitment should stay unchanged: %v, %v", got, ok)
		}
	})

	t.Run("確認後に募集状態が変わっていたら安全側で拒否する", func(t *testing.T) {
		b := newBot()
		current, started := b.startRecruitment("guild-1", "channel-1", "organizer-1")
		if !started {
			t.Fatalf("initial recruitment should start")
		}
		current.MessageID = "message-1"
		req, ok := b.prepareMatchRestartRequest("guild-1", "channel-1", "organizer-1", false)
		if !ok {
			t.Fatalf("prepareMatchRestartRequest should succeed")
		}

		current.IsOpen = false
		_, _, _, err := b.confirmMatchRestart(req.token, "organizer-1")
		if err == nil || err != errMatchRestartStateChanged {
			t.Fatalf("confirmMatchRestart error = %v, want %v", err, errMatchRestartStateChanged)
		}
		if _, ok := b.getPendingMatchRestart(req.token); ok {
			t.Fatalf("pending restart should be removed after state change")
		}
	})
}

func TestCancelMatchRestart(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	b := &Bot{
		rankData:             rankData,
		recruitments:         make(map[string]*model.Recruitment),
		pendingMatchRestarts: make(map[string]pendingMatchRestart),
		testDummies:          make(map[string]map[string]model.PlayerInfo),
		now:                  func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) },
	}
	current, started := b.startRecruitment("guild-1", "channel-1", "organizer-1")
	if !started {
		t.Fatalf("initial recruitment should start")
	}
	current.MessageID = "message-1"
	req, ok := b.prepareMatchRestartRequest("guild-1", "channel-1", "organizer-1", false)
	if !ok {
		t.Fatalf("prepareMatchRestartRequest should succeed")
	}

	if err := b.cancelMatchRestart(req.token, "organizer-1"); err != nil {
		t.Fatalf("cancelMatchRestart returned error: %v", err)
	}
	got, ok := b.getRecruitment("channel-1")
	if !ok || got != current || !got.IsOpen {
		t.Fatalf("recruitment should stay open: %v, %v", got, ok)
	}
	if _, ok := b.getPendingMatchRestart(req.token); ok {
		t.Fatalf("pending restart should be removed after cancel")
	}
}

func TestRollbackMatchRestart(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	b := &Bot{
		rankData:     rankData,
		recruitments: make(map[string]*model.Recruitment),
		testDummies:  make(map[string]map[string]model.PlayerInfo),
	}
	current := model.NewRecruitment(rankData)
	current.ChannelID = "channel-1"
	current.IsOpen = false
	next := model.NewRecruitment(rankData)
	next.ChannelID = "channel-1"
	next.IsOpen = true
	b.recruitments["channel-1"] = next
	b.testDummies["channel-1"] = map[string]model.PlayerInfo{
		"dummy-1": {ID: "dummy-1", Name: "dummy"},
	}

	b.rollbackMatchRestart(current, next)

	got, ok := b.getRecruitment("channel-1")
	if !ok || got != current || !got.IsOpen {
		t.Fatalf("recruitment should be restored to current: %v, %v", got, ok)
	}
	if _, ok := b.testDummies["channel-1"]; ok {
		t.Fatalf("test dummies should be cleared on rollback")
	}
}

func TestBuildMatchRestartComponents(t *testing.T) {
	components := buildMatchRestartComponents("restart-token")
	if len(components) != 1 {
		t.Fatalf("len(components) = %d, want 1", len(components))
	}

	row, ok := components[0].(discordgo.ActionsRow)
	if !ok {
		t.Fatalf("component should be actions row")
	}
	if len(row.Components) != 2 {
		t.Fatalf("len(row.Components) = %d, want 2", len(row.Components))
	}

	confirm, ok := row.Components[0].(discordgo.Button)
	if !ok {
		t.Fatalf("first component should be button")
	}
	cancel, ok := row.Components[1].(discordgo.Button)
	if !ok {
		t.Fatalf("second component should be button")
	}

	if confirm.CustomID != "match_restart_confirm:restart-token" {
		t.Fatalf("confirm.CustomID = %q", confirm.CustomID)
	}
	if cancel.CustomID != "match_restart_cancel:restart-token" {
		t.Fatalf("cancel.CustomID = %q", cancel.CustomID)
	}
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

func TestBuildAssignEmbedTeamFieldLayout(t *testing.T) {
	teams := [][]model.ScoredPlayer{
		{
			{ID: "user-1", Score: 3000},
			{ID: "user-2", Score: 2880},
		},
	}
	embed := buildAssignEmbed(teams, []string{"https://discord.gg/team-a"}, nil, false)

	if len(embed.Fields) != 1 {
		t.Fatalf("len(embed.Fields) = %d, want 1", len(embed.Fields))
	}

	field := embed.Fields[0]
	if field.Name != "チームA" {
		t.Fatalf("field.Name = %q, want %q", field.Name, "チームA")
	}
	if strings.Contains(field.Name, "平均スコア") {
		t.Fatalf("field.Name = %q, want not to contain %q", field.Name, "平均スコア")
	}
	if !strings.HasPrefix(field.Value, "平均スコア: 2940\n") {
		t.Fatalf("field.Value = %q, want prefix %q", field.Value, "平均スコア: 2940\n")
	}
	if !strings.Contains(field.Value, "<@user-1>\n<@user-2>") {
		t.Fatalf("field.Value = %q, want member list", field.Value)
	}
	if !strings.Contains(field.Value, "[📢 VCに参加](https://discord.gg/team-a)") {
		t.Fatalf("field.Value = %q, want vc link", field.Value)
	}
	if !field.Inline {
		t.Fatalf("field.Inline = %v, want true", field.Inline)
	}
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
