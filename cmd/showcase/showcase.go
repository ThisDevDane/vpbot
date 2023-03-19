package cmd

import (
	"encoding/json"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/thisdevdane/vpbot/cmd/shared"
	"github.com/thisdevdane/vpbot/internal/gateway"
)

const urlRegexString string = `(?:(?:https?|ftp):\/\/|\b(?:[a-z\d]+\.))(?:(?:[^\s()<>]+|\((?:[^\s()<>]+|(?:\([^\s()<>]+\)))?\))+(?:\((?:[^\s()<>]+|(?:\(?:[^\s()<>]+\)))?\)|[^\s!()\[\]{};:'".,<>?«»“”‘’]))?`

var (
	channelId string
	urlRegex  *regexp.Regexp
)

var ShowcaseCmd = &cobra.Command{
	Use: "showcase",
	Run: func(cmd *cobra.Command, _ []string) {
		gatewayOpts := gateway.GatewayOpts{
			Host:     shared.RedisAddr,
			Password: shared.RedisPassword,
		}
		outgoingGateway := gateway.CreateClient(cmd.Context(), gatewayOpts)
		incomingGateway := gateway.CreateClient(cmd.Context(), gatewayOpts)

		go moderateChannel(incomingGateway, outgoingGateway)

		log.Info().Msgf("showcase is now running on ID %s. Press CTRL-C to exit.", channelId)
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
		<-sc
	},
}

func moderateChannel(incoming *gateway.Client, outgoing *gateway.Client) {
	ch := incoming.ObtainMessageChannel(gateway.GetMessageChannelKey(channelId))
	for pubMsg := range ch {
		msg := shared.DiscordMsg{}
		if err := json.Unmarshal([]byte(pubMsg.Payload), &msg); err != nil {
			log.Error().Err(err).Send()
		} else {
			if msg.IsThread {
				continue
			}

			urlInMsg := urlRegex.MatchString(msg.Content)

			if !urlInMsg && !msg.HasAttachOrEmbeds {
				outgoing.PublishMessage(gateway.GatewayOutChannel, shared.DiscordMsg{
					UserDM: true,
					UserID: msg.UserID,
					Content: "I've deleted your recent message in our showcase channel.\n" +
						"Showcase messages require that either you include a link or a picture/file in your message,\n" +
						"if you believe your message has been wrongfully deleted, please contact a mod.",
				})
				outgoing.PublishMessage(gateway.GatewayCommandChannel, shared.GatewayCommand{
					Type:      shared.GatewayCmdDeleteMsg,
					ChannelID: msg.ChannelID,
					MsgId:     msg.MsgId,
				})
			}
		}
	}
}

func init() {
	ShowcaseCmd.Flags().StringVar(&channelId, "channel-id", "", "The ID of the channel to moderate as a showcase channel")
	ShowcaseCmd.MarkFlagRequired("channel-id")

	urlRegex = regexp.MustCompile(urlRegexString)
}
