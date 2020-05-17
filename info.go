package main

import (
	"fmt"
	"github.com/jasonlvhit/gocron"
	"log"
	"strings"
	"time"
)

func updateInfoChannel() {
	if infoChannel == nil {
		log.Println("No info channel found")
		return
	}

	var sb strings.Builder

	sb.WriteString("Currently part of these guilds: \n")
	for _, g := range discord.State.Guilds {
		sb.WriteString(fmt.Sprintf(" - %s | %s\n", g.Name, g.ID))
	}

	return

	sb.WriteString("\nPolicing these channels: \n")
	rows, _ := queryAllIdeasChannel.Query()
	for rows.Next() {
		var (
			guildID   string
			channelID string
		)

		if err := rows.Scan(&guildID, &channelID); err != nil {
			continue
		}

		guild, err := discord.State.Guild(guildID)
		if err != nil {
			log.Println("Err: Couldn't find guild with ID", guildID)
			continue
		}
		channel, _ := discord.State.Channel(channelID)

		sb.WriteString(fmt.Sprintf("- #%s in '%s'\n", channel.Name, guild.Name))
	}
	rows.Close()

	sb.WriteString("\nTracking users for these guilds: \n")
	rows, _ = queryAllUserTrackChannel.Query()
	for rows.Next() {
		var (
			guildID       string
			postChannelID string
		)

		if err := rows.Scan(&guildID, &postChannelID); err != nil {
			continue
		}

		guild, err := discord.State.Guild(guildID)
		if err != nil {
			log.Println("Err: Couldn't find guild with ID", guildID)
			continue
		}
		channel, _ := discord.State.Channel(postChannelID)

		sb.WriteString(fmt.Sprintf("- %s (posting in #%s)\n", guild.Name, channel.Name))
	}
	rows.Close()

	sb.WriteString("\nTracking ideas for these guilds: \n")
	rows, _ = queryAllIdeasChannel.Query()
	for rows.Next() {
		var (
			guildID   string
			channelID string
		)

		if err := rows.Scan(&guildID, &channelID); err != nil {
			continue
		}

		guild, err := discord.State.Guild(guildID)
		if err != nil {
			log.Println("Err: Couldn't find guild with ID", guildID)
			continue
		}
		channel, _ := discord.State.Channel(channelID)

		sb.WriteString(fmt.Sprintf("- %s (posting in #%s)\n", guild.Name, channel.Name))
	}
	rows.Close()

	// Post message
	messages, _ := discord.ChannelMessages(infoChannel.ID, 1, "", "", "")
	if len(messages) < 1 {
		discord.ChannelMessageSend(infoChannel.ID, sb.String())
	} else {
		m := messages[0]
		now := time.Now()
		sb.WriteString(fmt.Sprintf("\n\nLast edit: %s", now.String()))
		discord.ChannelMessageEdit(m.ChannelID, m.ID, sb.String())
	}
}

func initInfo() {
	gocron.Every(2).Minutes().From(gocron.NextTick()).DoSafely(updateInfoChannel)
}
