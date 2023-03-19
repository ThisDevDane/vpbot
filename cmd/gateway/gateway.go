package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/thisdevdane/vpbot/cmd/shared"
	"github.com/thisdevdane/vpbot/internal/gateway"
)

var (
	botToken        string
	rdb             *redis.Client
	cmdGateway      *gateway.Client
	outGoingGateway *gateway.Client
	incomingGateway *gateway.Client

	ctx context.Context
)
var GatewayCmd = &cobra.Command{
	Use: "gateway",
	Run: func(cmd *cobra.Command, _ []string) {
		ctx = cmd.Context()
		gatewayOpts := gateway.GatewayOpts{
			Host:     shared.RedisAddr,
			Password: shared.RedisPassword,
		}
		redisOpts := redis.Options{
			Addr:     shared.RedisAddr,
			Password: shared.RedisPassword,
			DB:       0,
		}
		outGoingGateway = gateway.CreateClient(ctx, gatewayOpts)
		incomingGateway = gateway.CreateClient(ctx, gatewayOpts)
		cmdGateway = gateway.CreateClient(ctx, gatewayOpts)
		rdb = redis.NewClient(&redisOpts)

		session, err := discordgo.New("Bot " + botToken)
		if err != nil {
			panic(err)
		}
		session.Identify.Intents = discordgo.IntentsAllWithoutPrivileged | discordgo.IntentsMessageContent
		session.StateEnabled = true

		session.AddHandler(messagePump)

		err = session.Open()
		defer session.Close()
		if err != nil {
			log.Fatal().Msgf("error opening connection,", err)
		}

		go pumpOutgoingMessage(session, outGoingGateway)
		go performCommands(session, cmdGateway)
		log.Info().Msg("gateway is now running.  Press CTRL-C to exit.")
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
		<-sc
	},
}

func performCommands(s *discordgo.Session, gateClient *gateway.Client) {
	ch := gateClient.ObtainMessageChannel(gateway.GatewayCommandChannel)
	for pubMsg := range ch {
		cmd := gateway.Command{}
		if err := json.Unmarshal([]byte(pubMsg.Payload), &cmd); err != nil {
			log.Error().Err(err).Send()
		} else {
			switch cmd.Type {
			case gateway.CmdDeleteMsg:
				log.Info().
					Str("channel", cmd.ChannelID).
					Str("msg_id", cmd.MsgId).
					Str("reason", cmd.Reason).
					Msg("deleting msg")
				s.ChannelMessageDelete(cmd.ChannelID, cmd.MsgId)

			default:
				log.Error().Interface("cmd", cmd).Msg("invalid command arrived on gateway!")
			}

		}
	}
}

func pumpOutgoingMessage(s *discordgo.Session, gateClient *gateway.Client) {
	ch := gateClient.ObtainMessageChannel(gateway.GatewayOutChannel)
	for pubMsg := range ch {
		msg := gateway.OutgoingMsg{}
		if err := json.Unmarshal([]byte(pubMsg.Payload), &msg); err != nil {
			log.Error().Err(err).Send()
		} else {
			switch {
			case msg.UserDM:
				dm, _ := s.UserChannelCreate(msg.UserID)
				s.ChannelMessageSend(dm.ID, msg.Content)

			case msg.ReplyID != nil:
				s.ChannelMessageSendReply(msg.ChannelID, msg.Content, &discordgo.MessageReference{
					ChannelID: msg.ChannelID,
					MessageID: *msg.ReplyID,
				})
			case msg.InternalID != nil:
				storedID, err := rdb.Get(ctx, *msg.InternalID).Result()
				if err != nil {
					s.ChannelMessageEdit(msg.ChannelID, storedID, msg.Content)
					if err != nil {
						log.Error().Err(err).Send()
					}
					continue
				}

				fallthrough
			default:
				m, err := s.ChannelMessageSend(msg.ChannelID, msg.Content)
				if err != nil {
					log.Error().Err(err).Send()
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

	gatewayChannel := fmt.Sprintf(gateway.GatewayMessageChannelTmpl, m.ChannelID)
	if strings.HasPrefix(m.Content, "!") {
		gatewayChannel = gateway.GatewayCommandChannel
	}

	ch, _ := s.State.Channel(m.ChannelID)
	err := incomingGateway.PublishMessage(gatewayChannel, gateway.IncomingMsg{
		MsgId:             m.ID,
		ChannelID:         m.ChannelID,
		UserID:            m.Author.ID,
		Content:           m.Content,
		HasAttachOrEmbeds: len(m.Attachments) > 0 || len(m.Embeds) > 0,
		IsThread:          ch.IsThread(),
	})

	if err != nil {
		log.Error().Err(err).Send()
	}
}

func init() {
	GatewayCmd.Flags().StringVar(&botToken, "token", "", "Bot token for connecting to Discord Gateway")
	GatewayCmd.MarkFlagRequired("token")
}
