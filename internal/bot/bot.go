package bot

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/RateteDev/ow-custommatch-bot/internal/model"
	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	session              *discordgo.Session
	players              *model.PlayerDataManager
	rankData             model.RankDataFile
	vcConfig             *model.VCConfigManager
	recruitments         map[string]*model.Recruitment
	pendingRegistrations map[string]pendingRegEntry
	testDummies          map[string]map[string]model.PlayerInfo
	now                  func() time.Time
}

type pendingRegEntry struct {
	rank      string
	channelID string
	autoEntry bool
}

const rankRegistrationValidDuration = 30 * 24 * time.Hour

type rankRegistrationPromptType int

const (
	rankRegistrationPromptNew rankRegistrationPromptType = iota
	rankRegistrationPromptRefresh
	rankRegistrationPromptManual
)

func New(dbPath string) (*Bot, error) {
	store, err := model.NewSQLiteStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}

	players, err := model.NewSQLitePlayerDataManager(store)
	if err != nil {
		return nil, fmt.Errorf("load sqlite players: %w", err)
	}
	ranks, err := model.LoadEmbeddedRankData()
	if err != nil {
		return nil, fmt.Errorf("load embedded ranks: %w", err)
	}
	vcConfig := model.NewSQLiteVCConfigManager(store)
	if err := vcConfig.Load(); err != nil {
		return nil, fmt.Errorf("load sqlite vc config: %w", err)
	}

	return &Bot{
		players:              players,
		rankData:             ranks,
		vcConfig:             vcConfig,
		recruitments:         make(map[string]*model.Recruitment),
		pendingRegistrations: make(map[string]pendingRegEntry),
		testDummies:          make(map[string]map[string]model.PlayerInfo),
		now:                  time.Now,
	}, nil
}

func (b *Bot) Run(token string) error {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return err
	}

	dg.AddHandler(b.onReady)
	dg.AddHandler(b.onInteractionCreate)

	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages

	if err := dg.Open(); err != nil {
		return err
	}
	defer dg.Close()
	b.session = dg

	if err := b.registerCommands(); err != nil {
		return err
	}

	log.Printf("ow-custommatch-bot (Go) is running with %d registered players", len(b.players.Data.Players))
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	return nil
}

func (b *Bot) registerCommands() error {
	appID := b.session.State.User.ID

	matchCmd := &discordgo.ApplicationCommand{
		Name:        "match",
		Description: "マッチングの募集を開始します",
	}
	if os.Getenv("OW_CUSTOMMATCH_BOT_TEST_MODE") == "true" {
		matchCmd.Options = []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionBoolean,
				Name:        "fill",
				Description: "ダミープレイヤーをランダム追加してテスト振り分けを行います（20〜60人）",
				Required:    false,
			},
		}
	}

	registerRankCmd := &discordgo.ApplicationCommand{
		Name:        "register_rank",
		Description: "チーム分けに使用するランクを登録・更新します",
	}

	myRankCmd := &discordgo.ApplicationCommand{
		Name:        "my_rank",
		Description: "登録済みのランク情報を確認します",
	}

	_, err := b.session.ApplicationCommandBulkOverwrite(appID, "", []*discordgo.ApplicationCommand{matchCmd, registerRankCmd, myRankCmd})
	return err
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("Logged in as %s", r.User.String())
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i == nil || i.Interaction == nil || i.Data == nil {
		return
	}

	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		switch i.ApplicationCommandData().Name {
		case "match":
			b.handleMatchStart(s, i)
		case "register_rank":
			b.handleRegisterRank(s, i)
		case "my_rank":
			b.handleMyRank(s, i)
		}
	case discordgo.InteractionMessageComponent:
		switch i.MessageComponentData().CustomID {
		case "entry":
			b.handleEntry(s, i)
		case "cancel_entry":
			b.handleCancelEntry(s, i)
		case "assign":
			b.handleAssign(s, i)
		case "cancel":
			b.handleCancel(s, i)
		case "rank_select":
			b.handleRankSelect(s, i)
		case "division_select":
			b.handleDivisionSelect(s, i)
		}
	}
}

func (b *Bot) handleMatchStart(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID := i.ChannelID
	if r, ok := b.recruitments[channelID]; ok && r.IsOpen {
		if err := b.respondEphemeralText(s, i, "このチャンネルでは既に募集が開始されています"); err != nil {
			log.Printf("failed to respond match start conflict: %v", err)
		}
		return
	}

	userID, _ := interactionUser(i)
	r := model.NewRecruitment(b.rankData)
	r.OrganizerID = userID
	r.ChannelID = channelID
	r.GuildID = i.GuildID
	r.IsOpen = true
	b.recruitments[channelID] = r
	b.testDummies[channelID] = make(map[string]model.PlayerInfo)

	if os.Getenv("OW_CUSTOMMATCH_BOT_TEST_MODE") == "true" && matchStartFillMode(i) {
		b.injectTestDummies(channelID, r)
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{buildRecruitEmbed(r, "🎮 マッチング募集", 0x2ECC71)},
			Components: b.buildRecruitComponents(r, false),
		},
	})
	if err != nil {
		log.Printf("failed to send match start message: %v", err)
		return
	}

	msg, err := s.InteractionResponse(i.Interaction)
	if err != nil {
		log.Printf("failed to get interaction response message: %v", err)
		return
	}
	r.MessageID = msg.ID
}

func (b *Bot) handleEntry(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID := i.ChannelID
	r, ok := b.recruitments[channelID]
	if !ok || !r.IsOpen {
		if err := b.respondEphemeralText(s, i, "募集は終了しています"); err != nil {
			log.Printf("failed to respond closed recruitment on entry: %v", err)
		}
		return
	}

	userID, name := interactionUser(i)
	if !r.AddEntry(userID, name) {
		if err := b.respondEphemeralText(s, i, "既にエントリー済みです"); err != nil {
			log.Printf("failed to respond duplicate entry: %v", err)
		}
		return
	}

	player := b.players.GetByID(userID)
	if player == nil || player.HighestRank.Rank == "" {
		// ランク未登録の間は募集一覧に入れず、登録完了後に自動エントリーする。
		r.RemoveEntry(userID)
		if err := b.startRankRegistrationFlow(s, i, rankRegistrationPromptNew, true); err != nil {
			log.Printf("failed to start rank registration flow: %v", err)
		}
		return
	}
	if b.isRankRegistrationExpired(player) {
		r.RemoveEntry(userID)
		if err := b.startRankRegistrationFlow(s, i, rankRegistrationPromptRefresh, true); err != nil {
			log.Printf("failed to start rank refresh flow: %v", err)
		}
		return
	}

	if err := b.updateRecruitEmbed(s, r, false); err != nil {
		log.Printf("failed to update recruit embed on entry: %v", err)
		if err := b.respondEphemeralText(s, i, "エントリー処理中にエラーが発生しました"); err != nil {
			log.Printf("failed to respond entry error: %v", err)
		}
		return
	}

	if err := b.ackInteraction(s, i); err != nil {
		log.Printf("failed to respond entry success: %v", err)
	}
}

func (b *Bot) handleRankSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		if err := b.updateComponentWithText(s, i, "ランクが選択されていません"); err != nil {
			log.Printf("failed to respond empty rank select: %v", err)
		}
		return
	}

	userID, name := interactionUser(i)
	selectedRank := data.Values[0]
	entry, ok := b.pendingRegistrations[userID]
	if !ok {
		if err := b.updateComponentWithText(s, i, "ランク登録を最初からやり直してください"); err != nil {
			log.Printf("failed to respond missing pending registration on rank select: %v", err)
		}
		return
	}
	entry.rank = selectedRank
	b.pendingRegistrations[userID] = entry

	if selectedRank == "top500" {
		if err := b.savePlayerRank(userID, name, "top500", ""); err != nil {
			log.Printf("failed to save top500 rank: %v", err)
			_ = b.updateComponentWithText(s, i, "ランク登録に失敗しました")
			return
		}
		delete(b.pendingRegistrations, userID)
		if entry.autoEntry {
			r, ok := b.recruitments[entry.channelID]
			if !ok || !r.IsOpen {
				_ = b.updateComponentWithText(s, i, "募集は終了しています")
				return
			}
			r.AddEntry(userID, name)
			if err := b.updateRecruitEmbed(s, r, false); err != nil {
				log.Printf("failed to update recruit embed after top500 entry: %v", err)
				_ = b.updateComponentWithText(s, i, "ランク登録後の更新に失敗しました")
				return
			}
		}
		if err := b.updateComponentWithText(s, i, b.rankRegistrationCompleteMessage(entry.autoEntry)); err != nil {
			log.Printf("failed to update rank select message: %v", err)
		}
		return
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{b.buildDivisionSelectEmbed()},
			Components: b.buildDivisionSelectComponents(),
			Content:    "",
		},
	})
	if err != nil {
		log.Printf("failed to show division selector: %v", err)
	}
}

func (b *Bot) handleDivisionSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		if err := b.updateComponentWithText(s, i, "ディビジョンが選択されていません"); err != nil {
			log.Printf("failed to respond empty division select: %v", err)
		}
		return
	}

	userID, name := interactionUser(i)
	entry, ok := b.pendingRegistrations[userID]
	if !ok || entry.rank == "" {
		if err := b.updateComponentWithText(s, i, "ランク選択からやり直してください"); err != nil {
			log.Printf("failed to respond missing pending rank: %v", err)
		}
		return
	}

	div := data.Values[0]
	if err := b.savePlayerRank(userID, name, entry.rank, div); err != nil {
		log.Printf("failed to save player rank: %v", err)
		_ = b.updateComponentWithText(s, i, "ランク登録に失敗しました")
		return
	}
	delete(b.pendingRegistrations, userID)

	if entry.autoEntry {
		r, ok := b.recruitments[entry.channelID]
		if !ok || !r.IsOpen {
			if err := b.updateComponentWithText(s, i, "募集は終了しています"); err != nil {
				log.Printf("failed to respond closed recruitment on pending division select: %v", err)
			}
			return
		}
		r.AddEntry(userID, name)
		if err := b.updateRecruitEmbed(s, r, false); err != nil {
			log.Printf("failed to update recruit embed after division select: %v", err)
			_ = b.updateComponentWithText(s, i, "ランク登録後の更新に失敗しました")
			return
		}
	}

	if err := b.updateComponentWithText(s, i, b.rankRegistrationCompleteMessage(entry.autoEntry)); err != nil {
		log.Printf("failed to update division select message: %v", err)
	}
}

func (b *Bot) handleRegisterRank(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if err := b.startRankRegistrationFlow(s, i, rankRegistrationPromptManual, false); err != nil {
		log.Printf("failed to start manual rank registration flow: %v", err)
	}
}

func (b *Bot) handleMyRank(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID, _ := interactionUser(i)
	player := b.players.GetByID(userID)

	if player == nil || player.HighestRank.Rank == "" {
		_ = b.respondEphemeralText(s, i, "ランクが登録されていません。エントリー時または /register_rank で登録できます。")
		return
	}

	rankLabel := player.HighestRank.Rank
	if player.HighestRank.Division != "" {
		rankLabel += " " + player.HighestRank.Division
	}

	registeredAt, err := time.Parse(time.RFC3339, player.RankUpdatedAt)
	if err != nil {
		_ = b.respondEphemeralText(s, i, "ランク登録日時の読み込みに失敗しました。/register_rank で再登録してください。")
		return
	}
	expiryDate := registeredAt.Add(rankRegistrationValidDuration).Format("2006/01/02")

	expiredMsg := ""
	if b.isRankRegistrationExpired(player) {
		expiredMsg = "\n⚠️ 有効期限が切れています。次回エントリー時に再登録が必要です。"
	}

	description := fmt.Sprintf(
		"ランク: **%s**\n登録日: %s\n有効期限: %s%s",
		rankLabel,
		registeredAt.Format("2006/01/02"),
		expiryDate,
		expiredMsg,
	)

	embed := &discordgo.MessageEmbed{
		Title:       "📊 ランク情報",
		Description: description,
		Color:       0x3498DB,
	}

	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:  discordgo.MessageFlagsEphemeral,
		},
	})
}

func (b *Bot) handleCancelEntry(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID := i.ChannelID
	r, ok := b.recruitments[channelID]
	if !ok || !r.IsOpen {
		if err := b.respondEphemeralText(s, i, "募集は終了しています"); err != nil {
			log.Printf("failed to respond closed recruitment on cancel entry: %v", err)
		}
		return
	}

	userID, _ := interactionUser(i)
	if !r.RemoveEntry(userID) {
		if err := b.respondEphemeralText(s, i, "エントリーしていません"); err != nil {
			log.Printf("failed to respond missing entry on cancel: %v", err)
		}
		return
	}

	if err := b.updateRecruitEmbed(s, r, false); err != nil {
		log.Printf("failed to update recruit embed on cancel entry: %v", err)
		if err := b.respondEphemeralText(s, i, "エントリー取り消し処理中にエラーが発生しました"); err != nil {
			log.Printf("failed to respond cancel entry error: %v", err)
		}
		return
	}

	if err := b.ackInteraction(s, i); err != nil {
		log.Printf("failed to respond cancel entry success: %v", err)
	}
}

func (b *Bot) handleAssign(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID := i.ChannelID
	r, ok := b.recruitments[channelID]
	if !ok || !r.IsOpen {
		if err := b.respondEphemeralText(s, i, "募集は終了しています"); err != nil {
			log.Printf("failed to respond closed recruitment on assign: %v", err)
		}
		return
	}

	userID, _ := interactionUser(i)
	if userID != r.OrganizerID {
		if err := b.respondEphemeralText(s, i, "振り分けできるのは発案者のみです"); err != nil {
			log.Printf("failed to respond unauthorized assign: %v", err)
		}
		return
	}

	// VC準備・招待リンク生成で3秒制限を超えやすいため、先にインタラクションを受理する。
	if err := b.ackInteraction(s, i); err != nil {
		log.Printf("failed to defer assign interaction: %v", err)
		return
	}

	scoredPlayers := make([]model.ScoredPlayer, 0, len(r.Entries))
	for _, e := range r.Entries {
		var player *model.PlayerInfo
		if strings.HasPrefix(e.UserID, "dummy-") {
			if dummy, ok := b.testDummies[i.ChannelID][e.UserID]; ok {
				player = &dummy
			}
		} else {
			player = b.players.GetByID(e.UserID)
		}
		name := e.Name
		highestRank := model.Rank{}
		if player != nil {
			if player.Name != "" {
				name = player.Name
			}
			highestRank = player.HighestRank
		}
		scoredPlayers = append(scoredPlayers, model.ScoredPlayer{
			ID:    e.UserID,
			Name:  name,
			Score: r.CalculatePlayerScore(highestRank),
		})
	}

	teams, remainder := r.MakeTeamsWithRemainder(scoredPlayers)
	if teams == nil {
		if err := b.followupEphemeralText(s, i, fmt.Sprintf("チーム分けには10人以上必要です（現在 %d 人）", len(scoredPlayers))); err != nil {
			log.Printf("failed to respond insufficient players on assign: %v", err)
		}
		return
	}

	fields := make([]*discordgo.MessageEmbedField, 0, len(teams))
	testModeResult := false
	for idx, team := range teams {
		members := make([]string, 0, len(team))
		for _, p := range team {
			if strings.HasPrefix(p.ID, "dummy-") {
				testModeResult = true
				if p.Name != "" {
					members = append(members, p.Name)
					continue
				}
			}
			members = append(members, "<@"+p.ID+">")
		}
		value := strings.Join(members, "\n")
		if value == "" {
			value = "（なし）"
		}
		avgScore := teamAverageScore(team)
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("チーム%s（平均スコア: %.0f）", teamLabel(idx), avgScore),
			Value:  value,
			Inline: true,
		})
	}
	embed := &discordgo.MessageEmbed{
		Title:  "🎲 チーム振り分け結果",
		Color:  0x3498DB,
		Fields: fields,
	}
	if testModeResult {
		embed.Footer = &discordgo.MessageEmbedFooter{Text: "テストモード"}
	}

	vcChannelIDs, err := b.ensureVCChannels(s, r.GuildID, len(teams))
	if err != nil {
		log.Printf("failed to ensure vc channels: %v", err)
		if err := b.followupEphemeralText(s, i, "VCチャンネルの準備に失敗しました"); err != nil {
			log.Printf("failed to respond vc setup error: %v", err)
		}
		return
	}

	type inviteResult struct {
		idx int
		url string
		err error
	}
	results := make(chan inviteResult, len(vcChannelIDs))
	var wg sync.WaitGroup
	for idx, chID := range vcChannelIDs {
		wg.Add(1)
		go func(idx int, chID string) {
			defer wg.Done()
			inv, err := s.ChannelInviteCreate(chID, discordgo.Invite{
				MaxAge:  86400,
				MaxUses: 0,
				Unique:  true,
			})
			if err != nil {
				results <- inviteResult{idx: idx, err: err}
				return
			}
			results <- inviteResult{idx: idx, url: "https://discord.gg/" + inv.Code}
		}(idx, chID)
	}
	wg.Wait()
	close(results)

	for res := range results {
		if res.err != nil {
			log.Printf("failed to create vc invite for team %s: %v", teamLabel(res.idx), res.err)
			if err := b.followupEphemeralText(s, i, "VC招待リンクの作成に失敗しました"); err != nil {
				log.Printf("failed to respond vc invite error: %v", err)
			}
			return
		}
		fields[res.idx].Value += "\n[📢 VCに参加](" + res.url + ")"
	}

	if len(remainder) > 0 {
		members := make([]string, 0, len(remainder))
		for _, p := range remainder {
			if strings.HasPrefix(p.ID, "dummy-") {
				testModeResult = true
				if p.Name != "" {
					members = append(members, p.Name)
					continue
				}
			}
			members = append(members, "<@"+p.ID+">")
		}
		value := strings.Join(members, "\n")
		if value == "" {
			value = "（なし）"
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   fmt.Sprintf("余りメンバー（%d人）", len(remainder)),
			Value:  value,
			Inline: false,
		})
	}
	embed.Fields = fields

	if _, err := s.ChannelMessageSendEmbed(i.ChannelID, embed); err != nil {
		log.Printf("failed to post team assignments: %v", err)
		if err := b.followupEphemeralText(s, i, "チーム振り分け結果の送信に失敗しました"); err != nil {
			log.Printf("failed to respond assign post error: %v", err)
		}
		return
	}

	r.HasAssigned = true
	if err := b.updateRecruitEmbed(s, r, false); err != nil {
		log.Printf("failed to update recruit embed after assign result post: %v", err)
	}
}

func (b *Bot) ensureVCChannels(s *discordgo.Session, guildID string, teamCount int) ([]string, error) {
	if guildID == "" {
		return nil, fmt.Errorf("guild id is empty")
	}
	if teamCount <= 0 {
		return []string{}, nil
	}
	if b.vcConfig == nil {
		return nil, fmt.Errorf("vc config manager is nil")
	}
	if b.vcConfig.Data.VCChannelIDs == nil {
		b.vcConfig.Data.VCChannelIDs = []string{}
	}

	categoryID := b.vcConfig.Data.CategoryID
	categoryMissing := categoryID == ""
	if !categoryMissing {
		ch, err := s.Channel(categoryID)
		if err != nil {
			if isDiscord404(err) {
				categoryMissing = true
			} else {
				return nil, fmt.Errorf("get category channel: %w", err)
			}
		} else if ch == nil || ch.Type != discordgo.ChannelTypeGuildCategory {
			categoryMissing = true
		}
	}
	if categoryMissing {
		// カテゴリが消えている場合は保存済みVCを削除してから再作成する。
		for _, chID := range b.vcConfig.Data.VCChannelIDs {
			if chID == "" {
				continue
			}
			if _, err := s.ChannelDelete(chID); err != nil && !isDiscord404(err) {
				log.Printf("failed to delete orphan vc channel %s: %v", chID, err)
			}
		}

		// 配下VCを全再作成するためID一覧をリセットする。
		b.vcConfig.Data.CategoryID = ""
		b.vcConfig.Data.VCChannelIDs = []string{}

		ch, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
			Name: "ow-custommatch-bot",
			Type: discordgo.ChannelTypeGuildCategory,
		})
		if err != nil {
			return nil, fmt.Errorf("create category channel: %w", err)
		}
		b.vcConfig.Data.CategoryID = ch.ID
	}

	for idx := range b.vcConfig.Data.VCChannelIDs {
		chID := b.vcConfig.Data.VCChannelIDs[idx]
		if chID == "" {
			ch, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
				Name:     "チーム" + teamLabel(idx),
				Type:     discordgo.ChannelTypeGuildVoice,
				ParentID: b.vcConfig.Data.CategoryID,
			})
			if err != nil {
				return nil, fmt.Errorf("create vc channel %d: %w", idx, err)
			}
			b.vcConfig.Data.VCChannelIDs[idx] = ch.ID
			continue
		}

		ch, err := s.Channel(chID)
		if err != nil {
			if !isDiscord404(err) {
				return nil, fmt.Errorf("get vc channel %s: %w", chID, err)
			}
			ch, err = s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
				Name:     "チーム" + teamLabel(idx),
				Type:     discordgo.ChannelTypeGuildVoice,
				ParentID: b.vcConfig.Data.CategoryID,
			})
			if err != nil {
				return nil, fmt.Errorf("recreate vc channel %d: %w", idx, err)
			}
			b.vcConfig.Data.VCChannelIDs[idx] = ch.ID
			continue
		}

		if ch == nil || ch.Type != discordgo.ChannelTypeGuildVoice {
			created, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
				Name:     "チーム" + teamLabel(idx),
				Type:     discordgo.ChannelTypeGuildVoice,
				ParentID: b.vcConfig.Data.CategoryID,
			})
			if err != nil {
				return nil, fmt.Errorf("replace vc channel %d: %w", idx, err)
			}
			b.vcConfig.Data.VCChannelIDs[idx] = created.ID
			continue
		}
	}

	for len(b.vcConfig.Data.VCChannelIDs) < teamCount {
		idx := len(b.vcConfig.Data.VCChannelIDs)
		ch, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
			Name:     "チーム" + teamLabel(idx),
			Type:     discordgo.ChannelTypeGuildVoice,
			ParentID: b.vcConfig.Data.CategoryID,
		})
		if err != nil {
			return nil, fmt.Errorf("create vc channel %d: %w", idx, err)
		}
		b.vcConfig.Data.VCChannelIDs = append(b.vcConfig.Data.VCChannelIDs, ch.ID)
	}

	if err := b.vcConfig.Save(); err != nil {
		return nil, fmt.Errorf("save vc config: %w", err)
	}

	return append([]string(nil), b.vcConfig.Data.VCChannelIDs[:teamCount]...), nil
}

func (b *Bot) handleCancel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID := i.ChannelID
	r, ok := b.recruitments[channelID]
	if !ok || !r.IsOpen {
		if err := b.respondEphemeralText(s, i, "募集は終了しています"); err != nil {
			log.Printf("failed to respond closed recruitment on cancel: %v", err)
		}
		return
	}

	userID, _ := interactionUser(i)
	if userID != r.OrganizerID {
		if err := b.respondEphemeralText(s, i, "募集を中止できるのは発案者のみです"); err != nil {
			log.Printf("failed to respond unauthorized cancel: %v", err)
		}
		return
	}

	r.IsOpen = false

	if err := b.updateClosedEmbed(s, r, "🚫 募集は中止されました"); err != nil {
		log.Printf("failed to update canceled embed: %v", err)
	}

	if err := b.ackInteraction(s, i); err != nil {
		log.Printf("failed to respond cancel success: %v", err)
	}
}

func (b *Bot) updateRecruitEmbed(s *discordgo.Session, r *model.Recruitment, disabled bool) error {
	if r == nil || r.ChannelID == "" || r.MessageID == "" {
		return nil
	}

	embed := buildRecruitEmbed(r, "🎮 マッチング募集", 0x2ECC71)
	components := b.buildRecruitComponents(r, disabled)
	edit := discordgo.NewMessageEdit(r.ChannelID, r.MessageID)
	edit.Embeds = &[]*discordgo.MessageEmbed{embed}
	edit.Components = &components
	_, err := s.ChannelMessageEditComplex(edit)
	return err
}

func (b *Bot) updateClosedEmbed(s *discordgo.Session, r *model.Recruitment, title string) error {
	if r == nil || r.ChannelID == "" || r.MessageID == "" {
		return nil
	}

	embed := buildRecruitEmbed(r, title, 0xE74C3C)
	components := []discordgo.MessageComponent{}
	edit := discordgo.NewMessageEdit(r.ChannelID, r.MessageID)
	edit.Embeds = &[]*discordgo.MessageEmbed{embed}
	edit.Components = &components
	_, err := s.ChannelMessageEditComplex(edit)
	return err
}

func buildRecruitEmbed(r *model.Recruitment, title string, color int) *discordgo.MessageEmbed {
	description := "募集は開始されていません"
	if r != nil && r.OrganizerID != "" {
		description = fmt.Sprintf("<@%s> が募集を開始しました", r.OrganizerID)
	}
	entryCount := 0
	participants := "（なし）"
	if r != nil {
		entryCount = len(r.Entries)
		participants = recruitParticipantList(r)
	}
	entryLabel := fmt.Sprintf("参加者（%d人 / 振り分けには10人以上必要）", entryCount)
	if entryCount >= 10 {
		entryLabel = fmt.Sprintf("参加者（%d人 / 振り分け可能✅）", entryCount)
	}

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  entryLabel,
				Value: participants,
			},
		},
	}
}

func recruitParticipantList(r *model.Recruitment) string {
	if r == nil || len(r.Entries) == 0 {
		return "（なし）"
	}

	users := make([]string, 0, len(r.Entries))
	for _, e := range r.Entries {
		if strings.HasPrefix(e.UserID, "dummy-") {
			users = append(users, e.Name)
			continue
		}
		users = append(users, "<@"+e.UserID+">")
	}
	return strings.Join(users, "\n")
}

func (b *Bot) buildRecruitComponents(r *model.Recruitment, disabled bool) []discordgo.MessageComponent {
	assignLabel := "🎲 振り分け"
	if r != nil && r.HasAssigned {
		assignLabel = "🔁 再振り分け"
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    "✅ エントリー",
					CustomID: "entry",
					Style:    discordgo.PrimaryButton,
					Disabled: disabled,
				},
				discordgo.Button{
					Label:    "❌ 取り消し",
					CustomID: "cancel_entry",
					Style:    discordgo.SecondaryButton,
					Disabled: disabled,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.Button{
					Label:    assignLabel,
					CustomID: "assign",
					Style:    discordgo.DangerButton,
					Disabled: disabled || r == nil || len(r.Entries) < 10,
				},
				discordgo.Button{
					Label:    "🚫 中止",
					CustomID: "cancel",
					Style:    discordgo.SecondaryButton,
					Disabled: disabled,
				},
			},
		},
	}
}

func teamAverageScore(team []model.ScoredPlayer) float64 {
	if len(team) == 0 {
		return 0
	}
	total := 0.0
	for _, p := range team {
		total += p.Score
	}
	return total / float64(len(team))
}

func (b *Bot) startRankRegistrationFlow(s *discordgo.Session, i *discordgo.InteractionCreate, promptType rankRegistrationPromptType, autoEntry bool) error {
	userID, _ := interactionUser(i)
	if userID == "" {
		return fmt.Errorf("interaction user not found")
	}
	if err := b.respondRankRegistrationPrompt(s, i, promptType); err != nil {
		return err
	}
	b.pendingRegistrations[userID] = pendingRegEntry{
		channelID: i.ChannelID,
		autoEntry: autoEntry,
	}
	return nil
}

func (b *Bot) respondRankRegistrationPrompt(s *discordgo.Session, i *discordgo.InteractionCreate, promptType rankRegistrationPromptType) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       b.rankRegistrationPromptTitle(promptType),
					Description: b.rankRegistrationPromptDescription(promptType),
					Color:       0x3498DB,
				},
			},
			Components: b.buildRankSelectComponents(),
		},
	})
}

func (b *Bot) buildRankSelectComponents() []discordgo.MessageComponent {
	options := []discordgo.SelectMenuOption{
		{Label: "Bronze", Value: "bronze"},
		{Label: "Silver", Value: "silver"},
		{Label: "Gold", Value: "gold"},
		{Label: "Platinum", Value: "platinum"},
		{Label: "Diamond", Value: "diamond"},
		{Label: "Master", Value: "master"},
		{Label: "Grandmaster", Value: "grandmaster"},
		{Label: "Top 500", Value: "top500"},
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "rank_select",
					Placeholder: "ランクを選択してください",
					Options:     options,
					MaxValues:   1,
				},
			},
		},
	}
}

func (b *Bot) rankRegistrationPromptTitle(promptType rankRegistrationPromptType) string {
	if promptType == rankRegistrationPromptRefresh {
		return "🔄 ランク再登録"
	}
	return "📝 ランク登録"
}

func (b *Bot) rankRegistrationPromptDescription(promptType rankRegistrationPromptType) string {
	switch promptType {
	case rankRegistrationPromptRefresh:
		return "ランク登録から30日が経過しています。ロール内での最高ランクを選択してください。登録後、自動的にエントリーされます。"
	case rankRegistrationPromptManual:
		return "ロール内での最高ランクを選択してください。"
	default:
		return "チーム分けのため、ロール内での最高ランクを選択してください。登録後、自動的にエントリーされます。"
	}
}

func (b *Bot) buildDivisionSelectEmbed() *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "📝 ディビジョン選択",
		Description: "ディビジョンを選択してください。",
		Color:       0x3498DB,
	}
}

func (b *Bot) buildDivisionSelectComponents() []discordgo.MessageComponent {
	options := []discordgo.SelectMenuOption{
		{Label: "5（一番下）", Value: "5"},
		{Label: "4", Value: "4"},
		{Label: "3", Value: "3"},
		{Label: "2", Value: "2"},
		{Label: "1（一番上）", Value: "1"},
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "division_select",
					Placeholder: "ディビジョンを選択してください",
					Options:     options,
					MaxValues:   1,
				},
			},
		},
	}
}

func (b *Bot) savePlayerRank(userID, name, rank, div string) error {
	rankUpdatedAt := b.nowUTC().Format(time.RFC3339)
	if existing := b.players.GetByID(userID); existing != nil {
		existing.Name = name
		existing.HighestRank = model.Rank{Rank: rank, Division: div}
		existing.RankUpdatedAt = rankUpdatedAt
		return b.players.Save()
	}

	return b.players.Add(model.PlayerInfo{
		ID:            userID,
		Name:          name,
		RankUpdatedAt: rankUpdatedAt,
		HighestRank: model.Rank{
			Rank:     rank,
			Division: div,
		},
	})
}

func (b *Bot) nowUTC() time.Time {
	if b != nil && b.now != nil {
		return b.now().UTC()
	}
	return time.Now().UTC()
}

func (b *Bot) isRankRegistrationExpired(player *model.PlayerInfo) bool {
	if player == nil || player.HighestRank.Rank == "" {
		return false
	}
	if player.RankUpdatedAt == "" {
		return true
	}
	registeredAt, err := time.Parse(time.RFC3339, player.RankUpdatedAt)
	if err != nil {
		return true
	}
	return !registeredAt.Add(rankRegistrationValidDuration).After(b.nowUTC())
}

func (b *Bot) rankRegistrationCompleteMessage(autoEntry bool) string {
	if autoEntry {
		return "✅ ランクを登録し、エントリーしました！"
	}
	return "✅ ランクを登録しました！"
}

func (b *Bot) ackInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	})
}

func (b *Bot) respondEphemeralText(s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func (b *Bot) followupEphemeralText(s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	_, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
	return err
}

func (b *Bot) updateComponentWithText(s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    content,
			Embeds:     []*discordgo.MessageEmbed{},
			Components: []discordgo.MessageComponent{},
		},
	})
}

func interactionUser(i *discordgo.InteractionCreate) (id string, name string) {
	if i == nil || i.Interaction == nil {
		return "", "unknown"
	}
	if i.Member != nil && i.Member.User != nil {
		u := i.Member.User
		display := u.Username
		if u.GlobalName != "" {
			display = u.GlobalName
		}
		if i.Member.Nick != "" {
			display = i.Member.Nick
		}
		return u.ID, display
	}
	if i.User != nil {
		u := i.User
		display := u.Username
		if u.GlobalName != "" {
			display = u.GlobalName
		}
		return u.ID, display
	}
	return "", "unknown"
}

func isDiscord404(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "404")
}

func teamLabel(idx int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if idx >= 0 && idx < len(alphabet) {
		return string(alphabet[idx])
	}
	return fmt.Sprintf("%d", idx+1)
}

func matchStartFillMode(i *discordgo.InteractionCreate) bool {
	if i == nil || i.Interaction == nil {
		return false
	}
	for _, opt := range i.ApplicationCommandData().Options {
		if opt != nil && opt.Name == "fill" {
			return opt.BoolValue()
		}
	}
	return false
}

func (b *Bot) injectTestDummies(channelID string, r *model.Recruitment) {
	count := rand.Intn(41) + 20
	ranks := b.testRankPool(r)
	if len(ranks) == 0 {
		ranks = []model.Rank{{Rank: "top500"}}
	}

	if _, ok := b.testDummies[channelID]; !ok {
		b.testDummies[channelID] = make(map[string]model.PlayerInfo)
	}

	for i := 0; i < count; i++ {
		id := fmt.Sprintf("dummy-%d", i+1)
		name := fmt.Sprintf("ダミー%d", i+1)
		rank := ranks[rand.Intn(len(ranks))]
		player := model.PlayerInfo{
			ID:          id,
			Name:        name,
			HighestRank: rank,
		}
		b.testDummies[channelID][id] = player
		r.AddEntry(id, name)
	}
}

func (b *Bot) testRankPool(r *model.Recruitment) []model.Rank {
	pool := make([]model.Rank, 0)
	if r == nil {
		return pool
	}
	for rank, divisions := range r.RankData.Ranks {
		for div := range divisions {
			pool = append(pool, model.Rank{Rank: rank, Division: div})
		}
	}
	return pool
}
