package main

import (
	"database/sql"
	"fmt"
	"github.com/jasonlvhit/gocron"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"strings"
	"time"
)

func postUserTrackingInfo() {
	tracked := make([]userTrackChannel, 0)

	rows, err := queryAllUserTrackChannel.Query()
	if err != nil {
		fmt.Println("ERR TRYING TO DO USER GRAPH!", err)
		return
	}

	for rows.Next() {
		var (
			guildID       string
			postChannelID string
		)

		if err := rows.Scan(&guildID, &postChannelID); err != nil {
			log.Println("ERR TRYING TO DO USER GRAPH!", err)
			return
		}

		tracked = append(tracked, userTrackChannel{guildID, postChannelID})
	}
	rows.Close()

	for _, t := range tracked {
		guild, err := discord.Guild(t.guildID)
		if err != nil {
			log.Println("ERR TRYING TO GET GUILD!", t.guildID, err)
			return
		}

		now := time.Now().UTC()
		year, week := now.ISOWeek()

		insertUserTrackData.Exec(guild.ID, week, year, guild.MemberCount)

		lastYear := year
		lastWeek := week
		if lastWeek-1 <= 0 {
			lastYear--
		} else {
			lastWeek--
		}

		var lastWeekUserCount int

		row := queryUserTrackDataByGuildAndDate.QueryRow(guild.ID, lastWeek, lastYear)
		err = row.Scan(&lastWeekUserCount)
		if err == sql.ErrNoRows {
			discord.ChannelMessageSend(t.postChannelID, fmt.Sprintf("User count in week %v: %v", week, guild.MemberCount))
			return
		}

		diff := guild.MemberCount - lastWeekUserCount

		percent := float32(diff) / float32(lastWeekUserCount) * 100

		symbol := "up"
		if percent < 0 {
			symbol = "down"
		}

		discord.ChannelMessageSend(t.postChannelID, fmt.Sprintf("User count in week %v %v: %v (%s %v%%) (last week: %v)", week, year, guild.MemberCount, symbol, percent, lastWeekUserCount))
	}
}

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

func cronSetup() {
	gocron.Every(1).Sunday().At("15:00").DoSafely(postUserTrackingInfo)
	gocron.Every(2).Minutes().From(gocron.NextTick()).DoSafely(updateInfoChannel)
	<-gocron.Start()
}
