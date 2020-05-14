package main

import (
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
)

func userCountCommandHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	guild, _ := session.State.Guild(msg.GuildID)
	session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Current user count: %d", guild.MemberCount))
}

func addUserTrackingHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if isGuildUserTracked(msg.GuildID) {
		session.ChannelMessageSend(msg.ChannelID, "Guild already tracked. o7")
	} else {
		insertUserTrackChannel.Exec(msg.GuildID, msg.ChannelID)
		session.ChannelMessageSend(msg.ChannelID, "Tracking guild user count, will post to this channel weekly. o7")
	}
}

func removeUserTrackingHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if isGuildUserTracked(msg.GuildID) {
		deleteUserTrackChannel.Exec(msg.GuildID)
		session.ChannelMessageSend(msg.ChannelID, "Will stop tracking user count on this guild. o7")
	} else {
		session.ChannelMessageSend(msg.ChannelID, "Guild already not tracked. o7")
	}
}

func isGuildUserTracked(guildID string) bool {
	row := queryUserTrackChannel.QueryRow(guildID)
	err := row.Scan()
	if err == sql.ErrNoRows {
		return false
	}

	return true
}
