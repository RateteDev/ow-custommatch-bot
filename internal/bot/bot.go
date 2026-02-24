package bot

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/RateteDev/MatchyBot/internal/model"
)

type Bot struct {
	session              *discordgo.Session
	players              *model.PlayerDataManager
	recruitment          *model.Recruitment
	pendingRegistrations map[string]string // userID -> 選択中のランク（一時状態）
}

func New(playersPath, rankPath string) (*Bot, error) {
	players, err := model.NewPlayerDataManager(playersPath)
	if err != nil {
		return nil, fmt.Errorf("load players: %w", err)
	}
	ranks, err := model.LoadRankData(rankPath)
	if err != nil {
		return nil, fmt.Errorf("load ranks: %w", err)
	}

	return &Bot{
		players:              players,
		recruitment:          model.NewRecruitment(ranks),
		pendingRegistrations: make(map[string]string),
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

	log.Printf("MatchyBot (Go) is running with %d registered players", len(b.players.Data.Players))
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	return nil
}

func (b *Bot) registerCommands() error {
	_, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, "", &discordgo.ApplicationCommand{
		Name:        "match",
		Description: "マッチングの募集を開始します",
	})
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
		}
	case discordgo.InteractionMessageComponent:
		switch i.MessageComponentData().CustomID {
		case "entry":
			b.handleEntry(s, i)
		case "cancel_entry":
			b.handleCancelEntry(s, i)
		case "close":
			b.handleClose(s, i)
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
	if b.recruitment.IsOpen {
		if err := b.respondEphemeralText(s, i, "募集は既に開始されています"); err != nil {
			log.Printf("failed to respond match start conflict: %v", err)
		}
		return
	}

	userID, _ := interactionUser(i)
	b.recruitment.Entries = []model.Entry{}
	b.recruitment.OrganizerID = userID
	b.recruitment.ChannelID = i.ChannelID
	b.recruitment.MessageID = ""
	b.recruitment.IsOpen = true

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{b.buildRecruitEmbed("🎮 マッチング募集")},
			Components: b.buildRecruitComponents(false),
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
	b.recruitment.MessageID = msg.ID
}

func (b *Bot) handleEntry(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.recruitment.IsOpen {
		if err := b.respondEphemeralText(s, i, "募集は終了しています"); err != nil {
			log.Printf("failed to respond closed recruitment on entry: %v", err)
		}
		return
	}

	userID, name := interactionUser(i)
	if !b.recruitment.AddEntry(userID, name) {
		if err := b.respondEphemeralText(s, i, "既にエントリー済みです"); err != nil {
			log.Printf("failed to respond duplicate entry: %v", err)
		}
		return
	}

	player := b.players.GetByID(userID)
	if player == nil || player.HighestRank.Rank == "" {
		// ランク未登録の間は募集一覧に入れず、登録完了後に自動エントリーする。
		b.recruitment.RemoveEntry(userID)
		if err := b.respondRankRegistrationPrompt(s, i); err != nil {
			log.Printf("failed to start rank registration flow: %v", err)
		}
		return
	}

	if err := b.updateRecruitEmbed(s, false); err != nil {
		log.Printf("failed to update recruit embed on entry: %v", err)
		if err := b.respondEphemeralText(s, i, "エントリー処理中にエラーが発生しました"); err != nil {
			log.Printf("failed to respond entry error: %v", err)
		}
		return
	}

	if err := b.respondEphemeralText(s, i, "✅ エントリーしました！"); err != nil {
		log.Printf("failed to respond entry success: %v", err)
	}
}

func (b *Bot) handleRankSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.recruitment.IsOpen {
		if err := b.updateComponentWithText(s, i, "募集は終了しています"); err != nil {
			log.Printf("failed to respond closed recruitment on rank select: %v", err)
		}
		return
	}

	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		if err := b.updateComponentWithText(s, i, "ランクが選択されていません"); err != nil {
			log.Printf("failed to respond empty rank select: %v", err)
		}
		return
	}

	userID, name := interactionUser(i)
	selectedRank := data.Values[0]
	b.pendingRegistrations[userID] = selectedRank

	if selectedRank == "top500" {
		if err := b.savePlayerRank(userID, name, "top500", ""); err != nil {
			log.Printf("failed to save top500 rank: %v", err)
			_ = b.updateComponentWithText(s, i, "ランク登録に失敗しました")
			return
		}
		delete(b.pendingRegistrations, userID)
		b.recruitment.AddEntry(userID, name)
		if err := b.updateRecruitEmbed(s, false); err != nil {
			log.Printf("failed to update recruit embed after top500 entry: %v", err)
			_ = b.updateComponentWithText(s, i, "ランク登録後の更新に失敗しました")
			return
		}
		if err := b.updateComponentWithText(s, i, "✅ ランクを登録し、エントリーしました！"); err != nil {
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
	if !b.recruitment.IsOpen {
		if err := b.updateComponentWithText(s, i, "募集は終了しています"); err != nil {
			log.Printf("failed to respond closed recruitment on division select: %v", err)
		}
		return
	}

	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		if err := b.updateComponentWithText(s, i, "ディビジョンが選択されていません"); err != nil {
			log.Printf("failed to respond empty division select: %v", err)
		}
		return
	}

	userID, name := interactionUser(i)
	rank, ok := b.pendingRegistrations[userID]
	if !ok || rank == "" {
		if err := b.updateComponentWithText(s, i, "ランク選択からやり直してください"); err != nil {
			log.Printf("failed to respond missing pending rank: %v", err)
		}
		return
	}

	div := data.Values[0]
	if err := b.savePlayerRank(userID, name, rank, div); err != nil {
		log.Printf("failed to save player rank: %v", err)
		_ = b.updateComponentWithText(s, i, "ランク登録に失敗しました")
		return
	}
	delete(b.pendingRegistrations, userID)

	b.recruitment.AddEntry(userID, name)
	if err := b.updateRecruitEmbed(s, false); err != nil {
		log.Printf("failed to update recruit embed after division select: %v", err)
		_ = b.updateComponentWithText(s, i, "ランク登録後の更新に失敗しました")
		return
	}

	if err := b.updateComponentWithText(s, i, "✅ ランクを登録し、エントリーしました！"); err != nil {
		log.Printf("failed to update division select message: %v", err)
	}
}

func (b *Bot) handleCancelEntry(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !b.recruitment.IsOpen {
		if err := b.respondEphemeralText(s, i, "募集は終了しています"); err != nil {
			log.Printf("failed to respond closed recruitment on cancel entry: %v", err)
		}
		return
	}

	userID, _ := interactionUser(i)
	if !b.recruitment.RemoveEntry(userID) {
		if err := b.respondEphemeralText(s, i, "エントリーしていません"); err != nil {
			log.Printf("failed to respond missing entry on cancel: %v", err)
		}
		return
	}

	if err := b.updateRecruitEmbed(s, false); err != nil {
		log.Printf("failed to update recruit embed on cancel entry: %v", err)
		if err := b.respondEphemeralText(s, i, "エントリー取り消し処理中にエラーが発生しました"); err != nil {
			log.Printf("failed to respond cancel entry error: %v", err)
		}
		return
	}

	if err := b.respondEphemeralText(s, i, "エントリーを取り消しました"); err != nil {
		log.Printf("failed to respond cancel entry success: %v", err)
	}
}

func (b *Bot) handleClose(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID, _ := interactionUser(i)
	if userID != b.recruitment.OrganizerID {
		if err := b.respondEphemeralText(s, i, "募集を締め切れるのは発案者のみです"); err != nil {
			log.Printf("failed to respond unauthorized close: %v", err)
		}
		return
	}

	b.recruitment.IsOpen = false

	scoredPlayers := make([]model.ScoredPlayer, 0, len(b.recruitment.Entries))
	for _, e := range b.recruitment.Entries {
		player := b.players.GetByID(e.UserID)
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
			Score: b.recruitment.CalculatePlayerScore(highestRank),
		})
	}

	teams := b.recruitment.MakeTeams(scoredPlayers)
	if len(teams) == 0 {
		if _, err := s.ChannelMessageSend(i.ChannelID, "チーム分け可能な人数が不足しています（5人単位で編成します）。"); err != nil {
			log.Printf("failed to post insufficient players message: %v", err)
		}
	} else {
		lines := make([]string, 0, len(teams))
		for idx, team := range teams {
			members := make([]string, 0, len(team))
			for _, p := range team {
				members = append(members, "<@"+p.ID+">")
			}
			lines = append(lines, fmt.Sprintf("**チーム%s**: %s", teamLabel(idx), strings.Join(members, ", ")))
		}
		if _, err := s.ChannelMessageSend(i.ChannelID, strings.Join(lines, "\n")); err != nil {
			log.Printf("failed to post team assignments: %v", err)
		}
	}

	if err := b.updateRecruitEmbed(s, true); err != nil {
		log.Printf("failed to disable recruit buttons on close: %v", err)
	}

	if err := b.respondEphemeralText(s, i, "募集を締め切りました"); err != nil {
		log.Printf("failed to respond close success: %v", err)
	}
}

func (b *Bot) handleCancel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID, _ := interactionUser(i)
	if userID != b.recruitment.OrganizerID {
		if err := b.respondEphemeralText(s, i, "募集を中止できるのは発案者のみです"); err != nil {
			log.Printf("failed to respond unauthorized cancel: %v", err)
		}
		return
	}

	b.recruitment.IsOpen = false

	if err := b.updateRecruitEmbed(s, true); err != nil {
		log.Printf("failed to disable recruit buttons on cancel: %v", err)
	}
	if err := b.updateRecruitTitle(s, "🚫 募集は中止されました"); err != nil {
		log.Printf("failed to update canceled title: %v", err)
	}

	if err := b.respondEphemeralText(s, i, "募集を中止しました"); err != nil {
		log.Printf("failed to respond cancel success: %v", err)
	}
}

func (b *Bot) updateRecruitEmbed(s *discordgo.Session, disabled bool) error {
	if b.recruitment.ChannelID == "" || b.recruitment.MessageID == "" {
		return nil
	}

	embed := b.buildRecruitEmbed("🎮 マッチング募集")
	components := b.buildRecruitComponents(disabled)
	edit := discordgo.NewMessageEdit(b.recruitment.ChannelID, b.recruitment.MessageID)
	edit.Embeds = &[]*discordgo.MessageEmbed{embed}
	edit.Components = &components
	_, err := s.ChannelMessageEditComplex(edit)
	return err
}

func (b *Bot) updateRecruitTitle(s *discordgo.Session, title string) error {
	if b.recruitment.ChannelID == "" || b.recruitment.MessageID == "" {
		return nil
	}

	embed := b.buildRecruitEmbed(title)
	components := b.buildRecruitComponents(true)
	edit := discordgo.NewMessageEdit(b.recruitment.ChannelID, b.recruitment.MessageID)
	edit.Embeds = &[]*discordgo.MessageEmbed{embed}
	edit.Components = &components
	_, err := s.ChannelMessageEditComplex(edit)
	return err
}

func (b *Bot) buildRecruitEmbed(title string) *discordgo.MessageEmbed {
	description := "募集は開始されていません"
	if b.recruitment.OrganizerID != "" {
		description = fmt.Sprintf("<@%s> が募集を開始しました", b.recruitment.OrganizerID)
	}

	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       0x2ECC71,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  fmt.Sprintf("参加者（%d人）", len(b.recruitment.Entries)),
				Value: b.recruitParticipantList(),
			},
		},
	}
}

func (b *Bot) recruitParticipantList() string {
	if len(b.recruitment.Entries) == 0 {
		return "（なし）"
	}

	users := make([]string, 0, len(b.recruitment.Entries))
	for _, e := range b.recruitment.Entries {
		users = append(users, "<@"+e.UserID+">")
	}
	return strings.Join(users, "\n")
}

func (b *Bot) buildRecruitComponents(disabled bool) []discordgo.MessageComponent {
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
					Label:    "🔒 締め切り",
					CustomID: "close",
					Style:    discordgo.DangerButton,
					Disabled: disabled,
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

func (b *Bot) respondRankRegistrationPrompt(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       "📝 ランク登録",
					Description: "チーム分けのためランクを登録してください。登録後、自動的にエントリーされます。",
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
	if existing := b.players.GetByID(userID); existing != nil {
		existing.Name = name
		existing.HighestRank = model.Rank{Rank: rank, Division: div}
		return b.players.Save()
	}

	return b.players.Add(model.PlayerInfo{
		ID:   userID,
		Name: name,
		HighestRank: model.Rank{
			Rank:     rank,
			Division: div,
		},
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

func teamLabel(idx int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if idx >= 0 && idx < len(alphabet) {
		return string(alphabet[idx])
	}
	return fmt.Sprintf("%d", idx+1)
}
