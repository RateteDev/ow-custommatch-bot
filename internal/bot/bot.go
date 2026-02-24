package bot

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"matchybot/internal/model"
)

type Bot struct {
	session     *discordgo.Session
	players     *model.PlayerDataManager
	recruitment *model.Recruitment
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

	return &Bot{players: players, recruitment: model.NewRecruitment(ranks)}, nil
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
		Name:        "menu",
		Description: "コマンドメニューを表示します",
	})
	return err
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	log.Printf("Logged in as %s", r.User.String())
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if i.ApplicationCommandData().Name != "menu" {
		return
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{{
				Title:       "コマンドメニュー",
				Description: "Go版MatchyBotのメニューです。今後ここに募集・チーム分け機能を追加します。",
				Color:       0x3498DB,
			}},
		},
	})
	if err != nil {
		log.Printf("failed to respond to interaction: %v", err)
	}
}
