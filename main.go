package main

import (
	"database/sql"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jasonlvhit/gocron"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var (
	token    string
	urlRegex *regexp.Regexp
	db       *sql.DB

	discord *discordgo.Session

	insertPoliceChannel           *sql.Stmt
	queryPoliceChannel            *sql.Stmt
	deletePoliceChannel           *sql.Stmt
	queryAllPoliceChannelForGuild *sql.Stmt

	insertUserTrackChannel   *sql.Stmt
	deleteUserTrackChannel   *sql.Stmt
	queryAllUserTrackChannel *sql.Stmt
	queryUserTrackChannel    *sql.Stmt

	insertUserTrackData              *sql.Stmt
	queryUserTrackDataByGuild        *sql.Stmt
	queryUserTrackDataByGuildAndDate *sql.Stmt

	queryRandomMathSentence  *sql.Stmt
	insertRandomMathSentence *sql.Stmt
)

type userTrackChannel struct {
	guildID       string
	postChannelID string
}

func postUserGraph() {
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
			fmt.Println("ERR TRYING TO DO USER GRAPH!", err)
			return
		}

		tracked = append(tracked, userTrackChannel{guildID, postChannelID})
	}
	rows.Close()

	for _, t := range tracked {
		guild, err := discord.Guild(t.guildID)
		if err != nil {
			fmt.Println("ERR TRYING TO GET GUILD!", t.guildID, err)
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

		fmt.Println(lastWeekUserCount)
		fmt.Println(guild.MemberCount)
		fmt.Println(diff)
		fmt.Println(float32(diff) / float32(lastWeekUserCount))
		fmt.Println(float32(diff) / float32(lastWeekUserCount) * 100)

		percent := float32(diff) / float32(lastWeekUserCount) * 100

		symbol := "up"
		if percent < 0 {
			symbol = "down"
		}

		discord.ChannelMessageSend(t.postChannelID, fmt.Sprintf("User count in week %v %v: %v (%s %v%%)", week, year, guild.MemberCount, symbol, percent))
	}
}

func cronSetup() {
	gocron.Every(1).Sunday().At("15:00").DoSafely(postUserGraph)
	<-gocron.Start()
}

func init() {

	flag.StringVar(&token, "t", "", "Bot Token")
	flag.Parse()
}

func main() {
	if token == "" {
		fmt.Println("No token provided. Please run: vpbot -t <bot token>")
		os.Exit(1)
	}

	urlRegex, _ = regexp.Compile(urlRegexString)
	db, _ := sql.Open("sqlite3", "./vpbot.db")

	db.Exec("CREATE TABLE IF NOT EXISTS police_channels (id INTEGER PRIMARY KEY, guild_id TEXT, channel_id TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS user_track_channel (id INTEGER PRIMARY KEY, guild_id TEXT, post_channel_id TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS user_track_data (id INTEGER PRIMARY KEY, guild_id TEXT, week_number INT, year INT, user_count INT)")
	db.Exec("CREATE TABLE IF NOT EXISTS math_sentence (id INTEGER PRIMARY KEY, sentence TEXT)")

	insertPoliceChannel, _ = db.Prepare("INSERT INTO police_channels (guild_id, channel_id) VALUES (?, ?)")
	deletePoliceChannel, _ = db.Prepare("DELETE FROM police_channels WHERE channel_id = ?")
	queryPoliceChannel, _ = db.Prepare("SELECT guild_id, channel_id FROM police_channels WHERE channel_id = ?")
	queryAllPoliceChannelForGuild, _ = db.Prepare("SELECT channel_id FROM police_channels WHERE guild_id = ?")

	insertUserTrackChannel, _ = db.Prepare("INSERT INTO user_track_channel (guild_id, post_channel_id) VALUES (?, ?)")
	queryAllUserTrackChannel, _ = db.Prepare("SELECT guild_id, post_channel_id FROM user_track_channel")
	queryUserTrackChannel, _ = db.Prepare("SELECT guild_id, post_channel_id FROM user_track_channel WHERE guild_id = ?")
	deleteUserTrackChannel, _ = db.Prepare("DELETE FROM user_track_channel WHERE guild_id = ?")

	insertUserTrackData, _ = db.Prepare("INSERT INTO user_track_data (guild_id, week_number, year, user_count) VALUES (?, ?, ?, ?)")
	queryUserTrackDataByGuild, _ = db.Prepare("SELECT guild_id, week_number, year, user_count FROM user_track_data WHERE guild_id = ?")
	queryUserTrackDataByGuildAndDate, _ = db.Prepare("SELECT user_count FROM user_track_data WHERE guild_id = ? AND week_number = ? AND year = ?")

	queryRandomMathSentence, _ = db.Prepare("SELECT sentence FROM math_sentence ORDER BY random() LIMIT 1")
	insertRandomMathSentence, _ = db.Prepare("INSERT INTO math_sentence (sentence) VALUES (?)")

	var err error
	discord, err = discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		os.Exit(1)
	}

	discord.StateEnabled = true

	discord.AddHandler(ready)
	discord.AddHandler(messageCreate)
	discord.AddHandler(guildCreate)

	err = discord.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
		os.Exit(1)
	}

	go cronSetup()

	fmt.Println("VPBot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	fmt.Println("VPBot is terminating...")

	discord.Close()

}

func guildCreate(s *discordgo.Session, g *discordgo.GuildCreate) {
	fmt.Printf("Bot has joined a guild! '%s'(%s)\n", g.Name, g.ID)

	fmt.Println("Bot is now part of these guilds:")
	for _, guild := range s.State.Guilds {
		fmt.Printf("\t '%s'(%s)\n", guild.Name, guild.ID)
	}
}

func ready(s *discordgo.Session, r *discordgo.Ready) {
	if len(r.Guilds) <= 0 {
		return
	}

	fmt.Println("Bot is part of these guilds:")

	for _, guild := range r.Guilds {
		fmt.Printf("\t '%s'(%s)\n", guild.Name, guild.ID)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	if strings.HasPrefix(m.Content, "!") {
		if userAllowedBotCommands(s, m.GuildID, m.ChannelID, m.Author.ID) {
			if strings.HasPrefix(m.Content, "!test") {
				s.ChannelMessageSend(m.ChannelID, "ACK")
				return
			}

			if strings.HasPrefix(m.Content, "!usertrack") {
				if isGuildUserTracked(m.GuildID) {
					s.ChannelMessageSend(m.ChannelID, "Guild already tracked. o7")
				} else {
					insertUserTrackChannel.Exec(m.GuildID, m.ChannelID)
					s.ChannelMessageSend(m.ChannelID, "Tracking guild user count, will post to this channel weekly. o7")
				}

				return
			}

			if strings.HasPrefix(m.Content, "!addmathsentence") {
				sentence := strings.TrimPrefix(m.Content, "!addmathsentence")
				sentence = strings.TrimSpace(sentence)
				if len(sentence) <= 1 {
					s.ChannelMessageSend(m.ChannelID, "Remember to include sentence in command...")
					return
				}
				insertRandomMathSentence.Exec(sentence)
				s.ChannelMessageSend(m.ChannelID, "Added sentence to set! o7")

				return
			}

			if strings.HasPrefix(m.Content, "!useruntrack") {
				if isGuildUserTracked(m.GuildID) {
					deleteUserTrackChannel.Exec(m.GuildID)
					s.ChannelMessageSend(m.ChannelID, "Will stop tracking user count on this guild. o7")
				} else {
					s.ChannelMessageSend(m.ChannelID, "Guild already not tracked. o7")
				}

				return
			}

			if strings.HasPrefix(m.Content, "!police") {
				if policeChannel(s, m.ChannelID, m.Author) {
					s.ChannelMessageSend(m.ChannelID, "Policing channel. o7")
				} else {
					s.ChannelMessageSend(m.ChannelID, "Channel already policed. o7")
				}

				return
			}

			if strings.HasPrefix(m.Content, "!info") {
				rows, _ := queryAllPoliceChannelForGuild.Query(m.GuildID)
				defer rows.Close()

				s.ChannelMessageSend(m.ChannelID, "Policing following channels:")
				for rows.Next() {
					var channelID string
					if err := rows.Scan(&channelID); err != nil {
						s.ChannelMessageSend(m.ChannelID, "Error querying data...")
						return
					}

					channel, _ := s.State.Channel(channelID)
					s.ChannelMessageSend(m.ChannelID, channel.Name)
				}

				return
			}

			if strings.HasPrefix(m.Content, "!unpolice") {
				if unpoliceChannel(s, m.ChannelID, m.Author) {
					s.ChannelMessageSend(m.ChannelID, "Stopping policing channel. o7")
				} else {
					s.ChannelMessageSend(m.ChannelID, "Channel not policed!")
				}

				return
			}

			if strings.HasPrefix(m.Content, "!usercount") {
				guild, _ := s.State.Guild(m.GuildID)
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Current user count: %d", guild.MemberCount))
				return
			}

			return
		}
	}

	if isChannelPoliced(m.ChannelID) {
		urlInMessage := urlRegex.MatchString(m.Content)

		if len(m.Attachments) <= 0 && len(m.Embeds) <= 0 && urlInMessage == false {
			guild, _ := s.State.Guild(m.GuildID)
			channel, _ := s.State.Channel(m.ChannelID)
			fmt.Printf("[%s|%s] Message did not furfill requirements! deleting message (%s) from %s#%s\n", guild.Name, channel.Name, m.ID, m.Author.Username, m.Author.Discriminator)
			s.ChannelMessageDelete(channel.ID, m.ID)
			sendPoliceDM(s, m.Author, guild, channel, "Message was deleted", "Showcase messages require that either you include a link or a picture/file in your message, if you believe your message has been wrongfully deleted, please contact a mod.\n If you wish to chat about showcase, please look for a #showcase-banter channel")
		}

		return
	}

	if len(m.Mentions) > 0 {
		for _, mention := range m.Mentions {
			if mention.ID == s.State.User.ID {
				str := strings.ToLower(m.Content)
				if strings.Contains(str, "math") {

					var sentence string
					row := queryRandomMathSentence.QueryRow()
					err := row.Scan(&sentence)
					if err == sql.ErrNoRows {
						sentence = "MATH IS THE WORST THING ON EARH"
					}

					recepient := m.Author

					if len(m.Mentions) > 1 {
						recepient = m.Mentions[1]
					}

					msg := fmt.Sprintf("%s %s", recepient.Mention(), sentence)
					s.ChannelMessageSend(m.ChannelID, msg)
				}
				break
			}
		}
	}
}

func sendPoliceDM(s *discordgo.Session, user *discordgo.User, guild *discordgo.Guild, channel *discordgo.Channel, event string, reason string) {
	dm, err := s.UserChannelCreate(user.ID)
	if err == nil {
		s.ChannelMessageSend(dm.ID, fmt.Sprintf("%s in '%s' channel '%s', reason:\n%s", event, guild.Name, channel.Name, reason))
	}
}

func userAllowedBotCommands(s *discordgo.Session, guildID string, channelID string, userID string) bool {
	perm, _ := s.State.UserChannelPermissions(userID, channelID)
	hasPerm := perm&discordgo.PermissionAdministrator != 0
	hasRole := false

	member, _ := s.State.Member(guildID, userID)
	if member != nil {

		guild, _ := s.State.Guild(guildID)
		for _, x := range guild.Roles {
			for _, y := range member.Roles {
				if x.ID == y {
					if x.Name == "Mod" || x.Name == "mod" {
						hasRole = true
					}
				}
			}
		}
	}

	return hasPerm || hasRole
}

func isGuildUserTracked(guildID string) bool {
	row := queryUserTrackChannel.QueryRow(guildID)
	err := row.Scan()
	if err == sql.ErrNoRows {
		return false
	}

	return true
}

func isChannelPoliced(channelID string) bool {
	row := queryPoliceChannel.QueryRow(channelID)
	err := row.Scan()
	if err == sql.ErrNoRows {
		return false
	}

	return true
}

func policeChannel(s *discordgo.Session, channelID string, user *discordgo.User) bool {
	if isChannelPoliced(channelID) {
		return false
	}

	channel, _ := s.State.Channel(channelID)
	guild, _ := s.State.Guild(channel.GuildID)
	insertPoliceChannel.Exec(guild.ID, channel.ID)
	fmt.Printf("Observing '%s'(%s) in '%s', requested by %s#%s\n", channel.Name, channel.ID, guild.Name, user.Username, user.Discriminator)

	return true
}

func unpoliceChannel(s *discordgo.Session, channelID string, user *discordgo.User) bool {
	if isChannelPoliced(channelID) {
		channel, _ := s.State.Channel(channelID)
		guild, _ := s.State.Guild(channel.GuildID)
		deletePoliceChannel.Exec(channel.ID)
		fmt.Printf("Stopped observing '%s'(%s) in '%s', requested by %s#%s\n", channel.Name, channel.ID, guild.Name, user.Username, user.Discriminator)
		return true
	}

	return false
}

const urlRegexString string = `(?:(?:https?|ftp):\/\/|\b(?:[a-z\d]+\.))(?:(?:[^\s()<>]+|\((?:[^\s()<>]+|(?:\([^\s()<>]+\)))?\))+(?:\((?:[^\s()<>]+|(?:\(?:[^\s()<>]+\)))?\)|[^\s!()\[\]{};:'".,<>?«»“”‘’]))?`
