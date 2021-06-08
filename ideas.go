package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var (
	modQueueChannel *discordgo.Channel
	ideasChannel *discordgo.Channel
)

type modQueueItem struct {
	AuthorID         string `json:"authorID"`
	AuthorName       string `json:"authorName"`
	GuildID          string `json:"guildID"`
	GuildName        string `json:"guildName"`
	PostingChannelID string `json:"postingChannelID"`
	Content          string `json:"content"`
}

func initIdeasChannel(s *discordgo.Session) {
	channelId := os.Getenv("VPBOT_MOD_QUEUE_CHANNEL")
	ideasChannelId := os.Getenv("VPBOT_IDEAS_CHANNEL")

	if(len(channelId) <= 0) {
		return
	}

	var err error
	modQueueChannel, err = s.Channel(channelId)
	if err != nil {
		log.Printf("Couldn't find the ideas channel with ID: %s or %s", channelId, ideasChannelId)
		return
	}

	ideasChannel, err = s.Channel(ideasChannelId)
	if err != nil {
		log.Printf("Couldn't find the ideas channel with ID: %s or %s", channelId, ideasChannelId)
		return
	}
}

func addIdeasHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if modQueueChannel == nil {
		session.ChannelMessageSend(msg.ChannelID, "Guild does not have an ideas channel, ask a mod to add one")
		return
	}

	guild, _ := session.State.Guild(msg.GuildID)

	idea := strings.TrimPrefix(msg.Content, "!addidea")
	idea = strings.TrimSpace(idea)

	item := modQueueItem{
		msg.Author.ID,
		fmt.Sprintf("%s#%s", msg.Author.Username, msg.Author.Discriminator),
		guild.ID,
		guild.Name,
		ideasChannel.ID,
		idea,
	}

	data, _ := json.MarshalIndent(item, "", "    ")
	session.ChannelMessageSend(modQueueChannel.ID, string(data))
}

func ideasQueueReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.UserID == s.State.User.ID {
		return
	}

	guild, _ := s.State.Guild(r.GuildID)
	channel, _ := s.State.Channel(r.ChannelID)
	user, _ := s.User(r.UserID)

	log.Printf("[%s|%s|%s#%s] (%s) Reaction added: %+v\n", guild.Name, channel.Name, user.Username, user.Discriminator, r.MessageID, r.Emoji)

	if r.ChannelID == modQueueChannel.ID {
		if r.Emoji.Name == "yes" {
			m, _ := s.ChannelMessage(r.ChannelID, r.MessageID)

			// Already moderated?
			for _, e := range m.Reactions {
				if (e.Emoji.Name == "yes" || e.Emoji.Name == "no") && e.Emoji.Name != r.Emoji.Name {
					return
				}
			}

			var item modQueueItem
			if err := json.Unmarshal([]byte(m.Content), &item); err != nil {
				s.ChannelMessageSend(r.ChannelID, err.Error())
			} else {
				member, _ := s.GuildMember(item.GuildID, item.AuthorID)
				message := fmt.Sprintf("%s's idea: %s", member.User.Mention(), item.Content)

				s.ChannelMessageSend(item.PostingChannelID, message)
			}
		}
	}
}
