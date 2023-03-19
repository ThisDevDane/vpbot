package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
)

const (
	messageChannelTmpl = "discord.msg.%s"
	outChannel         = "discord.out"
	commandChannel     = "discord.cmd"
)

type DiscordMsg struct {
	InternalID *string
	UserDM     bool
	ChannelID  string
	UserID     string
	Content    string
}

func (msg DiscordMsg) MarshalBinary() ([]byte, error) {
	return json.Marshal(msg)
}

var (
	botToken  string
	redisAddr string
	redisPass string
	rdb       *redis.Client
	ctx       context.Context
)
var gatewayCmd = &cobra.Command{
	Use: "gateway",
	Run: func(cmd *cobra.Command, _ []string) {
		ctx = cmd.Context()
		rdb = redis.NewClient(&redis.Options{
			Addr:     redisAddr,
			Password: redisPass,
			DB:       0,
		})
		defer rdb.Close()

		session, err := discordgo.New("Bot " + botToken)
		if err != nil {
			panic(err)
		}
		session.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent

		session.AddHandler(messagePump)

		err = session.Open()
		defer session.Close()
		if err != nil {
			log.Fatalln("error opening connection,", err)
		}

		go pumpOutgoingMessage(session, rdb)
		log.Println("gateway is now running.  Press CTRL-C to exit.")
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
		<-sc
	},
}

func pumpOutgoingMessage(s *discordgo.Session, rdb *redis.Client) {
	pubsub := rdb.Subscribe(ctx, outChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for pubMsg := range ch {
		msg := DiscordMsg{}
		if err := json.Unmarshal([]byte(pubMsg.Payload), &msg); err != nil {
			log.Println(err)
		} else {
			if msg.InternalID != nil {
				storedID, err := rdb.Get(ctx, *msg.InternalID).Result()
				if err != nil {
					s.ChannelMessageEdit(msg.ChannelID, storedID, msg.Content)
					if err != nil {
						log.Println(err)
					}
				}
			} else {
				m, err := s.ChannelMessageSend(msg.ChannelID, msg.Content)
				if err != nil {
					log.Println(err)
					continue
				}

				if msg.InternalID != nil {
					rdb.Set(ctx, *msg.InternalID, m.ID, 24*time.Hour)
				}
			}
		}
	}
}

func messagePump(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	pubsubChannel := fmt.Sprintf(messageChannelTmpl, m.ChannelID)
	if strings.HasPrefix(m.Content, "!") {
		pubsubChannel = commandChannel
	}

	status := rdb.Publish(ctx, pubsubChannel, DiscordMsg{
		ChannelID: m.ChannelID,
		UserID:    m.Author.ID,
		Content:   m.Content,
	})

	if status != nil && status.Err() != nil {
		log.Println(status.Err())
	}
}

func init() {
	gatewayCmd.Flags().StringVar(&botToken, "token", "", "Bot token for connecting to Discord Gateway")
	gatewayCmd.MarkFlagRequired("token")
	gatewayCmd.Flags().StringVar(&redisAddr, "redis-addr", "localhost:6379", "")
	gatewayCmd.Flags().StringVar(&redisPass, "redis-pass", "", "")

	rootCmd.AddCommand(gatewayCmd)
}
