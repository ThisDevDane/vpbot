package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"strings"
)

var (
	insertIdeasChannel        *sql.Stmt
	deleteIdeasChannel        *sql.Stmt
	queryIdeasChannelForGuild *sql.Stmt
	queryAllIdeasChannel      *sql.Stmt
)

type modQueueItem struct {
	AuthorID         string `json:authorID`
	AuthorName       string `json:authorName`
	GuildID          string `json:guildID`
	GuildName        string `json:guildName`
	PostingChannelID string `json:postingChannelID`
	Content          string `json:content`
}

func initIdeasChannel(db *sql.DB) {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS ideas_channel (id INTEGER PRIMARY KEY, guild_id TEXT, channel_id TEXT)")
	if err != nil {
		log.Panic(err)
	}

	insertIdeasChannel = dbPrepare(db, "INSERT INTO ideas_channel (guild_id, channel_id) VALUES (?, ?)")
	deleteIdeasChannel = dbPrepare(db, "DELETE FROM ideas_channel WHERE channel_id = ?")
	queryIdeasChannelForGuild = dbPrepare(db, "SELECT channel_id FROM ideas_channel WHERE guild_id = ?")
	queryAllIdeasChannel = dbPrepare(db, "SELECT guild_id, channel_id FROM ideas_channel")
}

func setupIdeasHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if setupIdeasChannel(session, msg.ChannelID, msg.Author) {
		session.ChannelMessageSend(msg.ChannelID, "Is now Ideas channel. o7")
	} else {
		session.ChannelMessageSend(msg.ChannelID, "Channel already ideas for guild. o7")
	}
}
func addIdeasHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	ok, postingChannelID := hasGuildIdeasChannel(msg.GuildID)

	if ok == false {
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
		postingChannelID,
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

	if verbose {
		log.Printf("[%s|%s|%s#%s] (%s) Reaction added: %+v\n", guild.Name, channel.Name, user.Username, user.Discriminator, r.MessageID, r.Emoji)
	}

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

func hasGuildIdeasChannel(guildID string) (bool, string) {
	row := queryIdeasChannelForGuild.QueryRow(guildID)
	var channelID string
	err := row.Scan(&channelID)
	if err == sql.ErrNoRows {
		return false, ""
	}

	return true, channelID
}

func setupIdeasChannel(s *discordgo.Session, channelID string, user *discordgo.User) bool {
	channel, _ := s.State.Channel(channelID)
	guild, _ := s.State.Guild(channel.GuildID)

	if ok, _ := hasGuildIdeasChannel(guild.ID); ok {
		return false
	}

	data := discordgo.ChannelEdit{
		Topic: "Use the command !addidea in any channel to post ideas, these will be added once a mod has reviewed them",
	}

	_, err := s.ChannelEditComplex(channelID, &data)
	if err != nil {
		fmt.Println(err)
	}

	insertIdeasChannel.Exec(guild.ID, channel.ID)
	log.Printf("Setup ideas '%s'(%s) in '%s', requested by %s#%s\n", channel.Name, channel.ID, guild.Name, user.Username, user.Discriminator)

	return true
}
