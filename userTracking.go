package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/go-co-op/gocron"
)

var (
	userTrackChannel *discordgo.Channel

	insertUserTrackData              *sql.Stmt
	queryUserTrackDataByGuildAndDate *sql.Stmt
)

func initUserTracking(s *discordgo.Session, db *sql.DB, scheduler *gocron.Scheduler) {
	channelId := os.Getenv("VPBOT_USERTRACK_CHANNEL")

	if len(channelId) > 0 {
		var err error
		userTrackChannel, err = s.Channel(channelId)

		if err != nil {
			log.Printf("Couldn't find the usertrack channel with ID: %s", channelId)
		}
	}

	_, err := db.Exec("CREATE TABLE IF NOT EXISTS user_track_data (id INTEGER PRIMARY KEY, guild_id TEXT, week_number INT, year INT, user_count INT)")
	if err != nil {
		log.Panic(err)
	}

	insertUserTrackData = dbPrepare(db,
		"INSERT INTO user_track_data (guild_id, week_number, year, user_count) VALUES ($1, $2, $3, $4)")
	queryUserTrackDataByGuildAndDate = dbPrepare(db,
		"SELECT user_count FROM user_track_data WHERE guild_id = $1 AND week_number = $2 AND year = $3")

	_, err = scheduler.Every(1).Sunday().At("15:00").Do(postUserTrackingInfo)
	if err != nil {
		log.Panic(err)
	}
}

func userCountCommandHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	guild, _ := session.State.Guild(msg.GuildID)
	session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("Current user count: %d", guild.MemberCount))
}

func postUserTrackingInfo() {
	guild, err := discord.State.Guild(guildID)
	if err != nil {
		log.Println("ERR TRYING TO GET GUILD!", guildID, err)
		return
	}

	now := time.Now().UTC()
	year, week := now.ISOWeek()

	_, err = insertUserTrackData.Exec(guild.ID, week, year, guild.MemberCount)
	if err != nil {
		log.Printf("Error trying to insert user count data: %s", err)
	}

	if userTrackChannel == nil {
		return
	}

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
		discord.ChannelMessageSend(userTrackChannel.ID, fmt.Sprintf("User count in week %v: %v", week, guild.MemberCount))
		return
	}

	diff := guild.MemberCount - lastWeekUserCount
	percent := float32(diff) / float32(lastWeekUserCount) * 100

	symbol := "up"
	if percent < 0 {
		symbol = "down"
	}

	discord.ChannelMessageSend(userTrackChannel.ID,
		fmt.Sprintf("User count in week %v %v: %v (%s %v%%) (last week: %v)",
			week,
			year,
			guild.MemberCount,
			symbol,
			percent,
			lastWeekUserCount))
}
