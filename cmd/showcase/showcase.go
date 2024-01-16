package cmd

import (
	"encoding/json"
	"fmt"
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

var Cmd = &cobra.Command{
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

		outgoingGateway.Close()
		incomingGateway.Close()
	},
}

func moderateChannel(incoming *gateway.Client, outgoing *gateway.Client) {
	ch := incoming.ObtainMessageChannel(gateway.GetMessageChannelKey(channelId))
	for pubMsg := range ch {
		msg := gateway.IncomingMsg{}
		if err := json.Unmarshal([]byte(pubMsg.Payload), &msg); err != nil {
			log.Error().Err(err).Send()
		} else {
			if msg.IsThread || msg.IsBot {
				continue
			}

			urlInMsg := urlRegex.MatchString(msg.Content)

			if !urlInMsg && !msg.HasAttachOrEmbeds {
				outgoing.PublishMessage(gateway.GatewayOutChannel, gateway.OutgoingMsg{
					UserDM: true,
					UserID: msg.UserID,
					Content: "I've deleted your recent message in our showcase channel.\n" +
						"Showcase messages require that either you include a link or a picture/file in your message, " +
						"if you're trying to discuss a posted showcase, please either start a thread or visit the showcase-banter channel\n" +
						"If you believe your message has been wrongfully deleted, please contact a mod.",
				})
				outgoing.PublishMessage(gateway.GatewayCommandChannel, gateway.Command{
					Type:      gateway.CmdDeleteMsg,
					ChannelID: msg.ChannelID,
					MsgId:     msg.MsgId,
					Reason:    fmt.Sprintf("failed showcase checks; url: %v attachembeds: %v", urlInMsg, msg.HasAttachOrEmbeds),
				})
			}
		}
	}
}

func init() {
	Cmd.Flags().StringVar(&channelId, "channel-id", "", "The ID of the channel to moderate as a showcase channel")
	Cmd.MarkFlagRequired("channel-id")

	urlRegex = regexp.MustCompile(urlRegexString)
}
