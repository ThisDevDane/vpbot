package main

import (
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
)

var (
	insertPoliceChannel           *sql.Stmt
	queryPoliceChannel            *sql.Stmt
	deletePoliceChannel           *sql.Stmt
	queryAllPoliceChannelForGuild *sql.Stmt
	queryAllPoliceChannel         *sql.Stmt
)

func initPoliceChannel(db *sql.DB) {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS police_channels (id INTEGER PRIMARY KEY, guild_id TEXT, channel_id TEXT)")
	if err != nil {
		log.Panic(err)
	}

	insertPoliceChannel = dbPrepare(db, "INSERT INTO police_channels (guild_id, channel_id) VALUES (?, ?)")
	deletePoliceChannel = dbPrepare(db, "DELETE FROM police_channels WHERE channel_id = ?")
	queryPoliceChannel = dbPrepare(db, "SELECT guild_id, channel_id FROM police_channels WHERE channel_id = ?")
	queryAllPoliceChannelForGuild = dbPrepare(db, "SELECT channel_id FROM police_channels WHERE guild_id = ?")
	queryAllPoliceChannel = dbPrepare(db, "SELECT guild_id, channel_id FROM police_channels")
}

func msgStreamPoliceHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if isChannelPoliced(msg.ChannelID) {
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

func addPoliceChannelHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if policeChannel(session, msg.ChannelID, msg.Author) {
		session.ChannelMessageSend(msg.ChannelID, "Policing channel. o7")
	} else {
		session.ChannelMessageSend(msg.ChannelID, "Channel already policed. o7")
	}
}

func removePoliceChannelHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if unpoliceChannel(session, msg.ChannelID, msg.Author) {
		session.ChannelMessageSend(msg.ChannelID, "Stopping policing channel. o7")
	} else {
		session.ChannelMessageSend(msg.ChannelID, "Channel not policed!")
	}
}

func infoPoliceChannelHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	rows, _ := queryAllPoliceChannelForGuild.Query(msg.GuildID)
	defer rows.Close()

	session.ChannelMessageSend(msg.ChannelID, "Policing following channels:")
	for rows.Next() {
		var channelID string
		if err := rows.Scan(&channelID); err != nil {
			session.ChannelMessageSend(msg.ChannelID, "Error querying data...")
			return
		}

		channel, _ := session.State.Channel(channelID)
		session.ChannelMessageSend(msg.ChannelID, channel.Name)
	}
}

func isChannelPoliced(channelID string) bool {
	row := queryPoliceChannel.QueryRow(channelID)
	err := row.Scan()
	if err == sql.ErrNoRows {
		return false
	}

	return true
}

func sendPoliceDM(s *discordgo.Session, user *discordgo.User, guild *discordgo.Guild, channel *discordgo.Channel, event string, reason string) {
	dm, err := s.UserChannelCreate(user.ID)
	if err == nil {
		s.ChannelMessageSend(dm.ID, fmt.Sprintf("%s in '%s' channel '%s', reason:\n%s", event, guild.Name, channel.Name, reason))
	}
}

func policeChannel(s *discordgo.Session, channelID string, user *discordgo.User) bool {
	if isChannelPoliced(channelID) {
		return false
	}

	channel, _ := s.State.Channel(channelID)
	guild, _ := s.State.Guild(channel.GuildID)
	insertPoliceChannel.Exec(guild.ID, channel.ID)
	log.Printf("Observing '%s'(%s) in '%s', requested by %s#%s\n", channel.Name, channel.ID, guild.Name, user.Username, user.Discriminator)

	return true
}

func unpoliceChannel(s *discordgo.Session, channelID string, user *discordgo.User) bool {
	if isChannelPoliced(channelID) {
		channel, _ := s.State.Channel(channelID)
		guild, _ := s.State.Guild(channel.GuildID)
		deletePoliceChannel.Exec(channel.ID)
		log.Printf("Stopped observing '%s'(%s) in '%s', requested by %s#%s\n", channel.Name, channel.ID, guild.Name, user.Username, user.Discriminator)
		return true
	}

	return false
}
