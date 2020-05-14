package main

import (
	"database/sql"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	token        string
	verbose      bool
	adminGuildID string
	httpPort     int

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

	insertMessageLogChannel        *sql.Stmt
	queryMessageLogChannelForGuild *sql.Stmt

	insertGithubChannel        *sql.Stmt
	queryGithubChannelForGuild *sql.Stmt
	queryGithubChannelForRepo  *sql.Stmt
)

type userTrackChannel struct {
	guildID       string
	postChannelID string
}

func init() {
	token = os.Getenv("VPBOT_TOKEN")
	adminGuildID = os.Getenv("VPBOT_ADMINGUILD_ID")
	verbose, _ = strconv.ParseBool(os.Getenv("VPBOT_VERBOSE"))
	httpPort, _ = strconv.Atoi(os.Getenv("VPBOT_HTTP_PORT"))

	flag.StringVar(&token, "t", token, "Bot Token")
	flag.StringVar(&adminGuildID, "a", adminGuildID, "Admin Guild ID")
	flag.BoolVar(&verbose, "v", false, "Verbose Output")
	flag.IntVar(&httpPort, "p", 13373, "HTTP port")
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

func setupDatabase() {
	db, _ := sql.Open("sqlite3", "./vpbot.db")

	db.Exec("CREATE TABLE IF NOT EXISTS police_channels (id INTEGER PRIMARY KEY, guild_id TEXT, channel_id TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS user_track_channel (id INTEGER PRIMARY KEY, guild_id TEXT, post_channel_id TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS user_track_data (id INTEGER PRIMARY KEY, guild_id TEXT, week_number INT, year INT, user_count INT)")
	db.Exec("CREATE TABLE IF NOT EXISTS math_sentence (id INTEGER PRIMARY KEY, sentence TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS ideas_channel (id INTEGER PRIMARY KEY, guild_id TEXT, channel_id TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS msglog_channel (id INTEGER PRIMARY KEY, guild_id TEXT, channel_id TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS github_channel (id INTEGER PRIMARY KEY, guild_id TEXT, channel_id TEXT, role_id TEXT, repo_id TEXT)")

	insertPoliceChannel = dbPrepare(db, "INSERT INTO police_channels (guild_id, channel_id) VALUES (?, ?)")
	deletePoliceChannel = dbPrepare(db, "DELETE FROM police_channels WHERE channel_id = ?")
	queryPoliceChannel = dbPrepare(db, "SELECT guild_id, channel_id FROM police_channels WHERE channel_id = ?")
	queryAllPoliceChannelForGuild = dbPrepare(db, "SELECT channel_id FROM police_channels WHERE guild_id = ?")
	queryAllPoliceChannel = dbPrepare(db, "SELECT guild_id, channel_id FROM police_channels")

	insertUserTrackChannel = dbPrepare(db, "INSERT INTO user_track_channel (guild_id, post_channel_id) VALUES (?, ?)")
	queryAllUserTrackChannel = dbPrepare(db, "SELECT guild_id, post_channel_id FROM user_track_channel")
	queryUserTrackChannel = dbPrepare(db, "SELECT guild_id, post_channel_id FROM user_track_channel WHERE guild_id = ?")
	deleteUserTrackChannel = dbPrepare(db, "DELETE FROM user_track_channel WHERE guild_id = ?")

	insertUserTrackData = dbPrepare(db, "INSERT INTO user_track_data (guild_id, week_number, year, user_count) VALUES (?, ?, ?, ?)")
	queryUserTrackDataByGuild = dbPrepare(db, "SELECT guild_id, week_number, year, user_count FROM user_track_data WHERE guild_id = ?")
	queryUserTrackDataByGuildAndDate = dbPrepare(db, "SELECT user_count FROM user_track_data WHERE guild_id = ? AND week_number = ? AND year = ?")

	queryRandomMathSentence = dbPrepare(db, "SELECT sentence FROM math_sentence ORDER BY random() LIMIT 1")
	insertRandomMathSentence = dbPrepare(db, "INSERT INTO math_sentence (sentence) VALUES (?)")

	insertIdeasChannel = dbPrepare(db, "INSERT INTO ideas_channel (guild_id, channel_id) VALUES (?, ?)")
	deleteIdeasChannel = dbPrepare(db, "DELETE FROM ideas_channel WHERE channel_id = ?")
	queryIdeasChannelForGuild = dbPrepare(db, "SELECT channel_id FROM ideas_channel WHERE guild_id = ?")
	queryAllIdeasChannel = dbPrepare(db, "SELECT guild_id, channel_id FROM ideas_channel")

	insertMessageLogChannel = dbPrepare(db, "INSERT INTO msglog_channel (guild_id, channel_id) VALUES (?, ?)")
	queryMessageLogChannelForGuild = dbPrepare(db, "SELECT channel_id FROM msglog_channel WHERE guild_id = ?")

	insertGithubChannel = dbPrepare(db, "INSERT INTO github_channel (guild_id, channel_id, repo_id, role_id) VALUES (?, ?, ?, ?)")
	queryGithubChannelForGuild = dbPrepare(db, "SELECT channel_id FROM github_channel WHERE guild_id = ?")
	queryGithubChannelForRepo = dbPrepare(db, "SELECT channel_id, role_id FROM github_channel WHERE repo_id = ?")
}

func dbPrepare(db *sql.DB, query string) *sql.Stmt {
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Println(err)
	}

	return stmt
}

func main() {
	if token == "" {
		fmt.Println("No token provided. Please run: vpbot -t <bot token> or set the VPBOT_TOKEN environment variable")
		os.Exit(1)
	}

	urlRegex, _ = regexp.Compile(urlRegexString)
	setupDatabase()

	var err error
	discord, err = discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		os.Exit(1)
	}

	discord.StateEnabled = true

	discord.AddHandler(messageCreate)
	discord.AddHandler(ideasQueueReactionAdd)

	handleCommand("ack", "Will make bot say 'ACK'", true, discordAckHandler)

	handleCommand("usercount", "Post the current user count for this guild", true, userCountCommandHandler)
	handleCommand("usertrack", "Tell VPBot to track the user count of this guild an post weekly updates (every sunday at 3pm UTC) to this channel", true, addUserTrackingHandler)
	handleCommand("useruntrack", "Tell VPBot to stop tracking the user count of this guild", true, removeUserTrackingHandler)

	handleCommand("addidea", "Suggest an idea to add to the server's idea channel, will go into a manual review queue before being posted", false, addIdeasHandler)
	handleCommand("ideas", "Setup the channel to be where ideas added with !addideas are posted after moderation", true, setupIdeasHandler)

	handleCommand("police", "channel to be policed (only messages containing links or attachments are allowed), messages not furfilling [sic] criteria will be deleted and a message will be sent to the offending user about why", true, addPoliceChannelHandler)
	handleCommand("unpolice", "Remove this channel from the policing list", true, removePoliceChannelHandler)
	handleCommand("policeinfo", "Shows what channels are being policed at the moment", true, infoPoliceChannelHandler)

	handleCommand("githubchan", "Setup a channel as a github channel for webhooks messages", true, githubCommandHandler)

	handleCommand("addmathsentence", "Will add a math related sentence that VPBot can say, make sure to make them about hating math", false, addMathSentence)

	log.Println("Opening up connection to discord...")
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

	setupHTTP()
	log.Printf("Starting HTTP server on port %d...\n", httpPort)
	go http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil)

	log.Println("Starting CRON services...")
	go cronSetup()

	log.Println("VPBot is now running.")
	fmt.Println("VPBot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	log.Println("VPBot is terminating...")
	fmt.Println("VPBot is terminating...")

	discord.Close()

}

func setupHTTP() {
	log.Println("Setting up HTTP handlers")
	http.HandleFunc("/github-webhook", githubWebhookHandler)
	http.HandleFunc("/ack", ackHandler)
}

func ackHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "ACK")
}

type commandHandler struct {
	commandString string
	description   string
	modOnly       bool
	handleFunc    func(*discordgo.Session, *discordgo.MessageCreate)
}

var commandMap = make(map[string]commandHandler)

func handleCommand(cmdString string, desc string, modOnly bool, handler func(*discordgo.Session, *discordgo.MessageCreate)) {
	if _, ok := commandMap[cmdString]; ok == false {
		cmdH := commandHandler{
			cmdString,
			desc,
			modOnly,
			handler,
		}
		commandMap[cmdString] = cmdH
	} else {
		log.Fatalf("Tried adding handler for '%s' when it already has one!", cmdString)
	}
}

func discordAckHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	session.ChannelMessageSend(msg.ChannelID, "ACK")
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
		message := strings.SplitN(m.Content, " ", 2)
		cmd := strings.TrimPrefix(message[0], "!")
		if handler, ok := commandMap[cmd]; ok {
			if handler.modOnly && userAllowedAdminBotCommands(s, m.GuildID, m.ChannelID, m.Author.ID) == false {
				s.ChannelMessageSend(m.ChannelID, "You are not allowed to use this command! SHAME ON YOU!")
				return
			}

			handler.handleFunc(s, m)
			return
		}

		if strings.HasPrefix(m.Content, "!help") {
			var sb strings.Builder

			user := m.Author
			if len(m.Mentions) > 0 {
				user = m.Mentions[0]
			}

			sb.WriteString("Following commands are available to ")
			sb.WriteString(user.Mention())
			sb.WriteString(";\n")

			for _, h := range commandMap {
				if h.modOnly == false || userAllowedAdminBotCommands(s, m.GuildID, m.ChannelID, user.ID) {
					if len(h.description) > 0 {
						sb.WriteString("`!")
						sb.WriteString(h.commandString)
						sb.WriteString("` ")
						sb.WriteString(h.description)
						sb.WriteString("\n")
					}
				}
			}

			s.ChannelMessageSend(m.ChannelID, sb.String())
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

func snowflakeCreationTime(ID string) (t time.Time, err error) {
	i, err := strconv.ParseInt(ID, 10, 64)
	if err != nil {
		return
	}
	timestamp := (i >> 22) + 1420070400000
	t = time.Unix(timestamp/1000, 0)
	return
}

const urlRegexString string = `(?:(?:https?|ftp):\/\/|\b(?:[a-z\d]+\.))(?:(?:[^\s()<>]+|\((?:[^\s()<>]+|(?:\([^\s()<>]+\)))?\))+(?:\((?:[^\s()<>]+|(?:\(?:[^\s()<>]+\)))?\)|[^\s!()\[\]{};:'".,<>?«»“”‘’]))?`
