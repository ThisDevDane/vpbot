package cmd

import (
	"encoding/json"
	"math/rand"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/thisdevdane/vpbot/cmd/shared"
	"github.com/thisdevdane/vpbot/internal/gateway"
)

var (
	channelId    string
	shurrupRegex *regexp.Regexp

	snarkyComeback = []string{
		"Well if you wouldn't keep breaking it, I wouldn't have to yell at you!",
		"NO! YOU SHURRUP! I HATE U!",
		"When pigs fly",
		"Can you you stop breaking things then? hmm? HMMM? >:|",
		"Oh I'm sorry mister, I'm only pointing out __**your**__ stupid mistakes :)",
		"Stop yelling, that is __**MY**__ job!",
	}
)

const shurrupRegexString = "(?i)shurrup"

func init() {
	shurrupRegex, _ = regexp.Compile(shurrupRegexString)
}

var SnarkCmd = &cobra.Command{
	Use: "snark",
	Run: func(cmd *cobra.Command, _ []string) {
		gatewayOpts := gateway.GatewayOpts{
			Host:     shared.RedisAddr,
			Password: shared.RedisPassword,
		}
		outgoingGateway := gateway.CreateClient(cmd.Context(), gatewayOpts)
		incomingGateway := gateway.CreateClient(cmd.Context(), gatewayOpts)

		go snark(incomingGateway, outgoingGateway)

		log.Info().Msgf("Github Snark is now running on ID %s. Press CTRL-C to exit.", channelId)
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
		<-sc

		outgoingGateway.Close()
		incomingGateway.Close()
	},
}

func snark(incoming *gateway.Client, outgoing *gateway.Client) {
	ch := incoming.ObtainMessageChannel(gateway.GetMessageChannelKey(channelId))
	for pubMsg := range ch {
		msg := gateway.IncomingMsg{}
		if err := json.Unmarshal([]byte(pubMsg.Payload), &msg); err != nil {
			log.Error().Err(err).Send()
		} else {
			if msg.IsThread {
				continue
			}

			if shurrupRegex.MatchString(msg.Content) {
				outgoing.PublishMessage(gateway.GatewayOutChannel, gateway.OutgoingMsg{
					ChannelID: msg.ChannelID,
					ReplyID:   &msg.MsgId,
					Content:   snarkyComeback[rand.Intn(len(snarkyComeback))],
				})
			}
		}
	}
}

func init() {
	SnarkCmd.Flags().StringVar(&channelId, "channel-id", "", "The ID of the channel to moderate as a showcase channel")
	SnarkCmd.MarkFlagRequired("channel-id")
}
