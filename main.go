package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jasonlvhit/gocron"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
)

var (
	token        string
	verbose      bool
	adminGuildID string

	urlRegex *regexp.Regexp
	db       *sql.DB

	discord *discordgo.Session

	insertPoliceChannel           *sql.Stmt
	queryPoliceChannel            *sql.Stmt
	deletePoliceChannel           *sql.Stmt
	queryAllPoliceChannelForGuild *sql.Stmt
	queryAllPoliceChannel         *sql.Stmt

	insertUserTrackChannel   *sql.Stmt
	deleteUserTrackChannel   *sql.Stmt
	queryAllUserTrackChannel *sql.Stmt
	queryUserTrackChannel    *sql.Stmt

	insertUserTrackData              *sql.Stmt
	queryUserTrackDataByGuild        *sql.Stmt
	queryUserTrackDataByGuildAndDate *sql.Stmt

	queryRandomMathSentence  *sql.Stmt
	insertRandomMathSentence *sql.Stmt

	insertIdeasChannel        *sql.Stmt
	deleteIdeasChannel        *sql.Stmt
	queryIdeasChannelForGuild *sql.Stmt
	queryAllIdeasChannel      *sql.Stmt
)

var (
	modQueueChannel *discordgo.Channel
	logsChannel     *discordgo.Channel
	infoChannel     *discordgo.Channel
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

		percent := float32(diff) / float32(lastWeekUserCount) * 100

		symbol := "up"
		if percent < 0 {
			symbol = "down"
		}

		discord.ChannelMessageSend(t.postChannelID, fmt.Sprintf("User count in week %v %v: %v (%s %v%%)", week, year, guild.MemberCount, symbol, percent))
	}
}

func updateInfoChannel() {
	if infoChannel == nil {
		return
	}

	var sb strings.Builder

	sb.WriteString("Currently part of these guilds: \n")
	for _, g := range discord.State.Guilds {
		sb.WriteString(fmt.Sprintf(" - %s | %s\n", g.Name, g.ID))
	}

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

		channel, _ := discord.State.Channel(channelID)
		guild, _ := discord.State.Guild(guildID)

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

		channel, _ := discord.State.Channel(postChannelID)
		guild, _ := discord.State.Guild(guildID)

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

		channel, _ := discord.State.Channel(channelID)
		guild, _ := discord.State.Guild(guildID)

		sb.WriteString(fmt.Sprintf("- %s (posting in #%s)\n", guild.Name, channel.Name))
	}
	rows.Close()

	// Post message
	messages, _ := discord.ChannelMessages(infoChannel.ID, 1, "", "", "")
	if len(messages) < 1 {
		discord.ChannelMessageSend(infoChannel.ID, sb.String())
	} else {
		m := messages[0]
		discord.ChannelMessageEdit(m.ChannelID, m.ID, sb.String())
	}
}

func cronSetup() {
	gocron.Every(1).Sunday().At("15:00").DoSafely(postUserGraph)
	gocron.Every(2).Minutes().From(gocron.NextTick()).DoSafely(updateInfoChannel)
	<-gocron.Start()
}

func init() {

	flag.StringVar(&token, "t", "", "Bot Token")
	flag.StringVar(&adminGuildID, "a", "", "Admin Guild")
	flag.BoolVar(&verbose, "v", false, "Verbose Output")
	flag.Parse()
}

type discordLogger struct {
	session       *discordgo.Session
	logsChannelID string
}

func (l discordLogger) Write(p []byte) (n int, err error) {
	_, e := l.session.ChannelMessageSend(l.logsChannelID, string(p))
	if e == nil {
		return len(p), nil
	}
	return 0, e
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
	db.Exec("CREATE TABLE IF NOT EXISTS ideas_channel (id INTEGER PRIMARY KEY, guild_id TEXT, channel_id TEXT)")

	insertPoliceChannel, _ = db.Prepare("INSERT INTO police_channels (guild_id, channel_id) VALUES (?, ?)")
	deletePoliceChannel, _ = db.Prepare("DELETE FROM police_channels WHERE channel_id = ?")
	queryPoliceChannel, _ = db.Prepare("SELECT guild_id, channel_id FROM police_channels WHERE channel_id = ?")
	queryAllPoliceChannelForGuild, _ = db.Prepare("SELECT channel_id FROM police_channels WHERE guild_id = ?")
	queryAllPoliceChannel, _ = db.Prepare("SELECT guild_id, channel_id FROM police_channels")

	insertUserTrackChannel, _ = db.Prepare("INSERT INTO user_track_channel (guild_id, post_channel_id) VALUES (?, ?)")
	queryAllUserTrackChannel, _ = db.Prepare("SELECT guild_id, post_channel_id FROM user_track_channel")
	queryUserTrackChannel, _ = db.Prepare("SELECT guild_id, post_channel_id FROM user_track_channel WHERE guild_id = ?")
	deleteUserTrackChannel, _ = db.Prepare("DELETE FROM user_track_channel WHERE guild_id = ?")

	insertUserTrackData, _ = db.Prepare("INSERT INTO user_track_data (guild_id, week_number, year, user_count) VALUES (?, ?, ?, ?)")
	queryUserTrackDataByGuild, _ = db.Prepare("SELECT guild_id, week_number, year, user_count FROM user_track_data WHERE guild_id = ?")
	queryUserTrackDataByGuildAndDate, _ = db.Prepare("SELECT user_count FROM user_track_data WHERE guild_id = ? AND week_number = ? AND year = ?")

	queryRandomMathSentence, _ = db.Prepare("SELECT sentence FROM math_sentence ORDER BY random() LIMIT 1")
	insertRandomMathSentence, _ = db.Prepare("INSERT INTO math_sentence (sentence) VALUES (?)")

	insertIdeasChannel, _ = db.Prepare("INSERT INTO ideas_channel (guild_id, channel_id) VALUES (?, ?)")
	deleteIdeasChannel, _ = db.Prepare("DELETE FROM ideas_channel WHERE channel_id = ?")
	queryIdeasChannelForGuild, _ = db.Prepare("SELECT channel_id FROM ideas_channel WHERE guild_id = ?")
	queryAllIdeasChannel, _ = db.Prepare("SELECT guild_id, channel_id FROM ideas_channel")

	var err error
	discord, err = discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		os.Exit(1)
	}

	discord.StateEnabled = true

	discord.AddHandler(messageCreate)
	discord.AddHandler(messageReactionAdd)

	err = discord.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
		os.Exit(1)
	}

	if len(adminGuildID) > 0 {
		// Setup admin guild
		fmt.Println("Setting up admin guild")
		adminGuild, err := discord.Guild(adminGuildID)

		if err != nil {
			fmt.Println("Could not find admin guild:", adminGuildID, err)
		} else {
			setupAdminGuild(discord, adminGuild)
		}

	}

	go cronSetup()

	fmt.Println("VPBot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	fmt.Println("VPBot is terminating...")

	discord.Close()

}

func setupAdminGuild(s *discordgo.Session, guild *discordgo.Guild) {
	fmt.Println("Setting up channels")

	modQueueChannel = setupTextChannel(guild, "mod-queue")
	logsChannel = setupTextChannel(guild, "logs")
	log.SetFlags(log.Lshortfile)
	logger := discordLogger{
		session:       discord,
		logsChannelID: logsChannel.ID,
	}
	log.SetOutput(logger)
	infoChannel = setupTextChannel(guild, "info")
}

func setupTextChannel(guild *discordgo.Guild, name string) *discordgo.Channel {
	for _, c := range guild.Channels {
		if c.Name == name {
			return c
		}
	}

	newChannel, _ := discord.GuildChannelCreate(guild.ID, name, discordgo.ChannelTypeGuildText)
	return newChannel
}

type modQueueItem struct {
	AuthorID         string `json:authorID`
	AuthorName       string `json:authorName`
	GuildID          string `json:guildID`
	GuildName        string `json:guildName`
	PostingChannelID string `json:postingChannelID`
	Content          string `json:content`
}

func messageReactionAdd(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
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

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	guild, _ := s.State.Guild(m.GuildID)
	channel, _ := s.State.Channel(m.ChannelID)

	if verbose {
		log.Printf("[%s|%s|%s#%s] (%s) %s\n", guild.Name, channel.Name, m.Author.Username, m.Author.Discriminator, m.ID, m.Content)
	}

	if strings.HasPrefix(m.Content, "!") {
		if strings.HasPrefix(m.Content, "!addidea") {
			ok, postingChannelID := hasGuildIdeasChannel(m.GuildID)

			if ok == false {
				s.ChannelMessageSend(m.ChannelID, "Guild does not have an ideas channel, ask a mod to add one")
				return
			}

			idea := strings.TrimPrefix(m.Content, "!addidea")
			idea = strings.TrimSpace(idea)

			item := modQueueItem{
				m.Author.ID,
				fmt.Sprintf("%s#%s", m.Author.Username, m.Author.Discriminator),
				guild.ID,
				guild.Name,
				postingChannelID,
				idea,
			}

			data, _ := json.MarshalIndent(item, "", "    ")
			s.ChannelMessageSend(modQueueChannel.ID, string(data))

			return
		}

		if userAllowedAdminBotCommands(s, m.GuildID, m.ChannelID, m.Author.ID) {
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

			if strings.HasPrefix(m.Content, "!ideas") {
				if setupIdeasChannel(s, m.ChannelID, m.Author) {
					s.ChannelMessageSend(m.ChannelID, "Is now Ideas channel. o7")
				} else {
					s.ChannelMessageSend(m.ChannelID, "Channel already ideas for guild. o7")
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
			log.Printf("[%s|%s] Message did not furfill requirements! deleting message (%s) from %s#%s\n", guild.Name, channel.Name, m.ID, m.Author.Username, m.Author.Discriminator)
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

func sendPoliceDM(s *discordgo.Session, user *discordgo.User, guild *discordgo.Guild, channel *discordgo.Channel, event string, reason string) {
	dm, err := s.UserChannelCreate(user.ID)
	if err == nil {
		s.ChannelMessageSend(dm.ID, fmt.Sprintf("%s in '%s' channel '%s', reason:\n%s", event, guild.Name, channel.Name, reason))
	}
}

func userAllowedAdminBotCommands(s *discordgo.Session, guildID string, channelID string, userID string) bool {
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

const urlRegexString string = `(?:(?:https?|ftp):\/\/|\b(?:[a-z\d]+\.))(?:(?:[^\s()<>]+|\((?:[^\s()<>]+|(?:\([^\s()<>]+\)))?\))+(?:\((?:[^\s()<>]+|(?:\(?:[^\s()<>]+\)))?\)|[^\s!()\[\]{};:'".,<>?«»“”‘’]))?`
