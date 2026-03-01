package bot

import (
	"encoding/json"
	"io"
	"net/http"
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

func TestBuildRecruitComponentsAssignButtonLabels(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	b := &Bot{rankData: rankData}
	r := model.NewRecruitment(rankData)
	r.Entries = make([]model.Entry, 10)

	t.Run("初回は通常ラベル", func(t *testing.T) {
		assignButton := findAssignButton(t, b.buildRecruitComponents(r, false))
		if assignButton.Label != "🎲 振り分け" {
			t.Fatalf("assign button label = %q, want %q", assignButton.Label, "🎲 振り分け")
		}
		if assignButton.Disabled {
			t.Fatalf("assign button should be enabled")
		}
	})

	t.Run("振り分け中は計算中ラベルで無効", func(t *testing.T) {
		r.AssignInProgress = true
		assignButton := findAssignButton(t, b.buildRecruitComponents(r, false))
		if assignButton.Label != "🤖 計算中" {
			t.Fatalf("assign button label = %q, want %q", assignButton.Label, "🤖 計算中")
		}
		if !assignButton.Disabled {
			t.Fatalf("assign button should be disabled while assign is in progress")
		}
		r.AssignInProgress = false
	})

	t.Run("振り分け済みは再振り分けラベル", func(t *testing.T) {
		r.HasAssigned = true
		assignButton := findAssignButton(t, b.buildRecruitComponents(r, false))
		if assignButton.Label != "🔁 再振り分け" {
			t.Fatalf("assign button label = %q, want %q", assignButton.Label, "🔁 再振り分け")
		}
		if assignButton.Disabled {
			t.Fatalf("assign button should be enabled after assign completion")
		}
	})
}

func TestTryStartAssignGuardsDuplicateExecution(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	r := model.NewRecruitment(rankData)
	r.ChannelID = "channel-1"
	r.IsOpen = true
	b := &Bot{
		recruitments: map[string]*model.Recruitment{
			"channel-1": r,
		},
	}

	if !b.tryStartAssign("channel-1") {
		t.Fatalf("first tryStartAssign should succeed")
	}
	if !r.AssignInProgress {
		t.Fatalf("AssignInProgress should be true after first tryStartAssign")
	}
	if b.tryStartAssign("channel-1") {
		t.Fatalf("second tryStartAssign should be rejected")
	}

	b.finishAssign("channel-1")

	if r.AssignInProgress {
		t.Fatalf("AssignInProgress should be false after finishAssign")
	}
	if !b.tryStartAssign("channel-1") {
		t.Fatalf("tryStartAssign should succeed again after finishAssign")
	}
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

func TestHandleAssignRejectsWhenAssignAlreadyInProgress(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	r := model.NewRecruitment(rankData)
	r.ChannelID = "channel-1"
	r.IsOpen = true
	r.OrganizerID = "organizer-1"
	r.AssignInProgress = true

	var requests []recordedRequest
	session := newTestDiscordSession(t, func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		requests = append(requests, recordedRequest{
			Method: req.Method,
			Path:   req.URL.Path,
			Body:   body,
		})
		return jsonHTTPResponse(`{}`), nil
	})

	b := &Bot{
		rankData: rankData,
		recruitments: map[string]*model.Recruitment{
			"channel-1": r,
		},
	}

	b.handleAssign(session, newAssignInteraction("channel-1", "organizer-1"))

	if len(requests) != 1 {
		t.Fatalf("len(requests) = %d, want 1", len(requests))
	}

	var payload map[string]any
	if err := json.Unmarshal(requests[0].Body, &payload); err != nil {
		t.Fatalf("failed to decode response payload: %v", err)
	}
	if got := int(payload["type"].(float64)); got != int(discordgo.InteractionResponseChannelMessageWithSource) {
		t.Fatalf("response type = %d, want %d", got, discordgo.InteractionResponseChannelMessageWithSource)
	}
	data := payload["data"].(map[string]any)
	if got := data["content"].(string); got != "現在振り分け中です" {
		t.Fatalf("content = %q, want %q", got, "現在振り分け中です")
	}
	if got := int(data["flags"].(float64)); got != int(discordgo.MessageFlagsEphemeral) {
		t.Fatalf("flags = %d, want %d", got, discordgo.MessageFlagsEphemeral)
	}
}

func TestHandleAssignRestoresAssignButtonAfterFailure(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	tests := []struct {
		name             string
		initialAssigned  bool
		wantRestoreLabel string
	}{
		{
			name:             "初回失敗なら通常ラベルへ戻す",
			initialAssigned:  false,
			wantRestoreLabel: "🎲 振り分け",
		},
		{
			name:             "再振り分け失敗なら再振り分けラベルへ戻す",
			initialAssigned:  true,
			wantRestoreLabel: "🔁 再振り分け",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := model.NewRecruitment(rankData)
			r.ChannelID = "channel-1"
			r.MessageID = "message-1"
			r.GuildID = "guild-1"
			r.IsOpen = true
			r.OrganizerID = "organizer-1"
			r.HasAssigned = tc.initialAssigned

			testDummies := make(map[string]model.PlayerInfo, 10)
			for i := 0; i < 10; i++ {
				userID := "dummy-" + teamLabel(i)
				r.AddEntry(userID, "Dummy "+teamLabel(i))
				testDummies[userID] = model.PlayerInfo{
					ID:   userID,
					Name: "Dummy " + teamLabel(i),
				}
			}

			var requests []recordedRequest
			session := newTestDiscordSession(t, func(req *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
				requests = append(requests, recordedRequest{
					Method: req.Method,
					Path:   req.URL.Path,
					Body:   body,
				})

				switch {
				case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/interactions/"):
					return jsonHTTPResponse(`{}`), nil
				case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/webhooks/"):
					return jsonHTTPResponse(`{"id":"followup-1"}`), nil
				case req.Method == http.MethodPatch && req.URL.Path == "/api/v9/channels/channel-1/messages/message-1":
					return jsonHTTPResponse(`{"id":"message-1"}`), nil
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
					return nil, nil
				}
			})

			b := &Bot{
				rankData: rankData,
				recruitments: map[string]*model.Recruitment{
					"channel-1": r,
				},
				testDummies: map[string]map[string]model.PlayerInfo{
					"channel-1": testDummies,
				},
			}

			b.handleAssign(session, newAssignInteraction("channel-1", "organizer-1"))

			if len(requests) != 3 {
				t.Fatalf("len(requests) = %d, want 3", len(requests))
			}

			initialPayload := decodeJSONMap(t, requests[0].Body)
			if got := int(initialPayload["type"].(float64)); got != int(discordgo.InteractionResponseUpdateMessage) {
				t.Fatalf("initial response type = %d, want %d", got, discordgo.InteractionResponseUpdateMessage)
			}
			initialData := initialPayload["data"].(map[string]any)
			label, disabled := extractAssignButtonState(t, initialData)
			if label != "🤖 計算中" {
				t.Fatalf("initial assign button label = %q, want %q", label, "🤖 計算中")
			}
			if !disabled {
				t.Fatalf("initial assign button should be disabled")
			}

			followupPayload := decodeJSONMap(t, requests[1].Body)
			if got := followupPayload["content"].(string); got != "VCチャンネルの準備に失敗しました" {
				t.Fatalf("followup content = %q, want %q", got, "VCチャンネルの準備に失敗しました")
			}

			restorePayload := decodeJSONMap(t, requests[2].Body)
			label, disabled = extractAssignButtonState(t, restorePayload)
			if label != tc.wantRestoreLabel {
				t.Fatalf("restored assign button label = %q, want %q", label, tc.wantRestoreLabel)
			}
			if disabled {
				t.Fatalf("restored assign button should be enabled")
			}
			if r.AssignInProgress {
				t.Fatalf("AssignInProgress should be false after handleAssign returns")
			}
		})
	}
}

func TestHandleAssignRestoresReassignButtonAfterSuccess(t *testing.T) {
	rankData, err := model.LoadEmbeddedRankData()
	if err != nil {
		t.Fatalf("LoadEmbeddedRankData failed: %v", err)
	}

	vcConfig := model.NewVCConfigManager(filepath.Join(t.TempDir(), "vc-config.json"))
	if err := vcConfig.Load(); err != nil {
		t.Fatalf("vcConfig.Load failed: %v", err)
	}

	r := model.NewRecruitment(rankData)
	r.ChannelID = "channel-1"
	r.MessageID = "message-1"
	r.GuildID = "guild-1"
	r.IsOpen = true
	r.OrganizerID = "organizer-1"

	testDummies := make(map[string]model.PlayerInfo, 10)
	for i := 0; i < 10; i++ {
		userID := "dummy-" + teamLabel(i)
		r.AddEntry(userID, "Dummy "+teamLabel(i))
		testDummies[userID] = model.PlayerInfo{
			ID:   userID,
			Name: "Dummy " + teamLabel(i),
		}
	}

	var (
		mu       sync.Mutex
		requests []recordedRequest
	)
	session := newTestDiscordSession(t, func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}

		mu.Lock()
		requests = append(requests, recordedRequest{
			Method: req.Method,
			Path:   req.URL.Path,
			Body:   body,
		})
		mu.Unlock()

		switch {
		case req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/interactions/"):
			return jsonHTTPResponse(`{}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/api/v9/guilds/guild-1/channels":
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("failed to decode guild channel payload: %v", err)
			}
			name, _ := payload["name"].(string)
			switch name {
			case "ow-custommatch-bot":
				return jsonHTTPResponse(`{"id":"category-1","type":4}`), nil
			case "チームA":
				return jsonHTTPResponse(`{"id":"vc-1","type":2}`), nil
			case "チームB":
				return jsonHTTPResponse(`{"id":"vc-2","type":2}`), nil
			default:
				t.Fatalf("unexpected guild channel name: %q", name)
				return nil, nil
			}
		case req.Method == http.MethodPost && req.URL.Path == "/api/v9/channels/vc-1/invites":
			return jsonHTTPResponse(`{"code":"invite-1"}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/api/v9/channels/vc-2/invites":
			return jsonHTTPResponse(`{"code":"invite-2"}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/api/v9/channels/channel-1/messages":
			return jsonHTTPResponse(`{"id":"result-1"}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/api/v9/channels/channel-1/messages/message-1":
			return jsonHTTPResponse(`{"id":"message-1"}`), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	b := &Bot{
		rankData: rankData,
		vcConfig: vcConfig,
		recruitments: map[string]*model.Recruitment{
			"channel-1": r,
		},
		testDummies: map[string]map[string]model.PlayerInfo{
			"channel-1": testDummies,
		},
	}

	b.handleAssign(session, newAssignInteraction("channel-1", "organizer-1"))

	mu.Lock()
	defer mu.Unlock()

	if len(requests) != 8 {
		t.Fatalf("len(requests) = %d, want 8", len(requests))
	}

	initialPayload := decodeJSONMap(t, requests[0].Body)
	initialData := initialPayload["data"].(map[string]any)
	label, disabled := extractAssignButtonState(t, initialData)
	if label != "🤖 計算中" {
		t.Fatalf("initial assign button label = %q, want %q", label, "🤖 計算中")
	}
	if !disabled {
		t.Fatalf("initial assign button should be disabled")
	}

	restorePayload := decodeJSONMap(t, requests[len(requests)-1].Body)
	label, disabled = extractAssignButtonState(t, restorePayload)
	if label != "🔁 再振り分け" {
		t.Fatalf("restored assign button label = %q, want %q", label, "🔁 再振り分け")
	}
	if disabled {
		t.Fatalf("restored assign button should be enabled")
	}
	if r.AssignInProgress {
		t.Fatalf("AssignInProgress should be false after handleAssign returns")
	}
	if !r.HasAssigned {
		t.Fatalf("HasAssigned should be true after successful assign")
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

type recordedRequest struct {
	Method string
	Path   string
	Body   []byte
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newTestDiscordSession(t *testing.T, transport roundTripFunc) *discordgo.Session {
	t.Helper()

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("discordgo.New failed: %v", err)
	}
	session.Client = &http.Client{Transport: transport}
	return session
}

func newAssignInteraction(channelID, userID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:        "interaction-1",
			AppID:     "app-1",
			Token:     "token-1",
			ChannelID: channelID,
			GuildID:   "guild-1",
			Type:      discordgo.InteractionMessageComponent,
			Member: &discordgo.Member{
				User: &discordgo.User{
					ID:       userID,
					Username: "Organizer",
				},
			},
		},
	}
}

func jsonHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func decodeJSONMap(t *testing.T, body []byte) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("failed to decode JSON payload: %v", err)
	}
	return payload
}

func extractAssignButtonState(t *testing.T, payload map[string]any) (string, bool) {
	t.Helper()

	components, ok := payload["components"].([]any)
	if !ok {
		t.Fatalf("components not found in payload: %v", payload)
	}
	for _, rowValue := range components {
		row, ok := rowValue.(map[string]any)
		if !ok {
			continue
		}
		children, ok := row["components"].([]any)
		if !ok {
			continue
		}
		for _, childValue := range children {
			child, ok := childValue.(map[string]any)
			if !ok {
				continue
			}
			if customID, _ := child["custom_id"].(string); customID == "assign" {
				label, _ := child["label"].(string)
				disabled, _ := child["disabled"].(bool)
				return label, disabled
			}
		}
	}

	t.Fatalf("assign button not found in payload: %v", payload)
	return "", false
}
