package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bwmarrin/discordgo"
)

const urlRegexString string = `(?:(?:https?|ftp):\/\/|\b(?:[a-z\d]+\.))(?:(?:[^\s()<>]+|\((?:[^\s()<>]+|(?:\([^\s()<>]+\)))?\))+(?:\((?:[^\s()<>]+|(?:\(?:[^\s()<>]+\)))?\)|[^\s!()\[\]{};:'".,<>?«»“”‘’]))?`

var (
	policeChannel *discordgo.Channel
)

func initPoliceChannel(s *discordgo.Session) {
	channelId := os.Getenv("VPBOT_POLICE_CHANNEL")

	if(len(channelId) <= 0) {
		return
	}

	var err error
	policeChannel, err = s.Channel(channelId)

	if err != nil {
		log.Printf("Couldn't find the police with ID: %s", channelId)
		return
	}
}

func msgStreamPoliceHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if policeChannel != nil && msg.ChannelID == policeChannel.ID {
		urlInMessage := urlRegex.MatchString(msg.Content)

		if len(msg.Attachments) <= 0 && len(msg.Embeds) <= 0 && urlInMessage == false {
			guild, _ := session.State.Guild(msg.GuildID)
			channel, _ := session.State.Channel(msg.ChannelID)
			log.Printf("[%s|%s] Message did not furfill requirements! deleting message (%s) from %s#%s\n%s", guild.Name, channel.Name, msg.ID, msg.Author.Username, msg.Author.Discriminator, msg.Content)
			session.ChannelMessageDelete(channel.ID, msg.ID)
			sendPoliceDM(session, msg.Author, guild, channel, "Message was deleted", "Showcase messages require that either you include a link or a picture/file in your message, if you believe your message has been wrongfully deleted, please contact a mod.\n If you wish to chat about showcase, please look for a #showcase-banter channel")
		}
	}
}

func sendPoliceDM(s *discordgo.Session, user *discordgo.User, guild *discordgo.Guild, channel *discordgo.Channel, event string, reason string) {
	dm, err := s.UserChannelCreate(user.ID)
	if err == nil {
		s.ChannelMessageSend(dm.ID, fmt.Sprintf("%s in '%s' channel '%s', reason:\n%s", event, guild.Name, channel.Name, reason))
	}
}