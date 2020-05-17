package main

import (
	"database/sql"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jasonlvhit/gocron"
	"log"
	"time"
)

var (
	insertUserTrackChannel   *sql.Stmt
	deleteUserTrackChannel   *sql.Stmt
	queryAllUserTrackChannel *sql.Stmt
	queryUserTrackChannel    *sql.Stmt

	insertUserTrackData              *sql.Stmt
	queryUserTrackDataByGuild        *sql.Stmt
	queryUserTrackDataByGuildAndDate *sql.Stmt
)

func initUserTracking(db *sql.DB) {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS user_track_channel (id INTEGER PRIMARY KEY, guild_id TEXT, post_channel_id TEXT)")
	if err != nil {
		log.Panic(err)
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS user_track_data (id INTEGER PRIMARY KEY, guild_id TEXT, week_number INT, year INT, user_count INT)")
	if err != nil {
		log.Panic(err)
	}

	insertUserTrackChannel = dbPrepare(db, "INSERT INTO user_track_channel (guild_id, post_channel_id) VALUES (?, ?)")
	queryAllUserTrackChannel = dbPrepare(db, "SELECT guild_id, post_channel_id FROM user_track_channel")
	queryUserTrackChannel = dbPrepare(db, "SELECT guild_id, post_channel_id FROM user_track_channel WHERE guild_id = ?")
	deleteUserTrackChannel = dbPrepare(db, "DELETE FROM user_track_channel WHERE guild_id = ?")

	insertUserTrackData = dbPrepare(db, "INSERT INTO user_track_data (guild_id, week_number, year, user_count) VALUES (?, ?, ?, ?)")
	queryUserTrackDataByGuild = dbPrepare(db, "SELECT guild_id, week_number, year, user_count FROM user_track_data WHERE guild_id = ?")
	queryUserTrackDataByGuildAndDate = dbPrepare(db, "SELECT user_count FROM user_track_data WHERE guild_id = ? AND week_number = ? AND year = ?")

	gocron.Every(1).Sunday().At("15:00").DoSafely(postUserTrackingInfo)
}

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
