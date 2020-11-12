package main

import (
	"database/sql"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/jasonlvhit/gocron"
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

	commandMap            = make(map[string]commandHandler)
	messageStreamHandlers = make([]func(*discordgo.Session, *discordgo.MessageCreate), 0)
)

type userTrackChannel struct {
	guildID       string
	postChannelID string
}

type commandHandler struct {
	commandString string
	description   string
	modOnly       bool
	handleFunc    func(*discordgo.Session, *discordgo.MessageCreate)
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

func dbPrepare(db *sql.DB, query string) *sql.Stmt {
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Println(err)
	}

	return stmt
}

func main() {
	log.SetFlags(log.Lshortfile)

	if token == "" {
		log.Println("No token provided. Please run: vpbot -t <bot token> or set the VPBOT_TOKEN environment variable")
		os.Exit(1)
	}

	urlRegex, _ = regexp.Compile(urlRegexString)
	var err error
	db, err = sql.Open("sqlite3", "./vpbot.db")

	if err != nil {
		log.Panic(err)
	}

	initPoliceChannel(db)
	initMathSentence(db)
	initUserTracking(db)
	initIdeasChannel(db)
	initGithubChannel(db)
	initInfo()
	initOdin()
	initMarkov(db)

	discord, err = discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		os.Exit(1)
	}

	discord.UpdateStatus(0, "Ruining users lives, one stupid message at a time")


	discord.StateEnabled = true

	discord.AddHandler(messageCreate)
	discord.AddHandler(ideasQueueReactionAdd)

	handleCommand("ack", "Will make bot say 'ACK'", false, discordAckHandler)
	handleCommand("help", "Will print a message with all available commands to the user", false, helpHandler)
	handleCommand("version", "Will print the version of VPBot", false, versionCommandHandler)

	handleCommand("usercount", "Post the current user count for this guild", true, userCountCommandHandler)
	handleCommand("usertrack", "Tell VPBot to track the user count of this guild an post weekly updates (every sunday at 3pm UTC) to this channel", true, addUserTrackingHandler)
	handleCommand("useruntrack", "Tell VPBot to stop tracking the user count of this guild", true, removeUserTrackingHandler)

	handleCommand("addidea", "Suggest an idea to add to the server's idea channel, will go into a manual review queue before being posted", false, addIdeasHandler)
	handleCommand("ideas", "Setup the channel to be where ideas added with !addideas are posted after moderation", true, setupIdeasHandler)

	handleCommand("police", "channel to be policed (only messages containing links or attachments are allowed), messages not furfilling [sic] criteria will be deleted and a message will be sent to the offending user about why", true, addPoliceChannelHandler)
	handleCommand("unpolice", "Remove this channel from the policing list", true, removePoliceChannelHandler)
	handleCommand("policeinfo", "Shows what channels are being policed at the moment", true, infoPoliceChannelHandler)

	handleCommand("githubchan", "Setup a channel as a github channel for webhooks messages", true, githubCommandHandler)

	handleCommand("addmathsentence", "Will add a math related sentence that VPBot can say, make sure to make them about hating math", false, addMathSentenceHandler)

	handleCommand("odinrun", "Will compile an odin code block and run it", true, odinRunHandle)

	handleCommand("markovsave", "Force a save of the markov chain", true, markovForceSave)
	handleCommand("markovsay", "Force a message generation in markov", false, markovForceSay)

	addMessageStreamHandler(msgStreamMathMessageHandler)
	addMessageStreamHandler(msgStreamPoliceHandler)
	addMessageStreamHandler(msgStreamGithubMessageHandler)
	addMessageStreamHandler(msgStreamMarkovTrainHandler)
	addMessageStreamHandler(msgStreamMarkovSayHandler)

	log.Println("Opening up connection to discord...")
	err = discord.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
		os.Exit(1)
	}

	if len(adminGuildID) > 0 {
		// Setup admin guild
		log.Println("Setting up admin guild")
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
	<-gocron.Start()

	log.Println("VPBot is now running.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	log.Println("VPBot is terminating...")

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

func addMessageStreamHandler(handler func(*discordgo.Session, *discordgo.MessageCreate)) {
	messageStreamHandlers = append(messageStreamHandlers, handler)
}

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

func helpHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	var sb strings.Builder

	user := msg.Author
	if len(msg.Mentions) > 0 {
		user = msg.Mentions[0]
	}

	sb.WriteString("Following commands are available to ")
	sb.WriteString(user.Mention())
	sb.WriteString(";\n")

	for _, h := range commandMap {
		if h.modOnly == false || userAllowedAdminBotCommands(session, msg.GuildID, msg.ChannelID, user.ID) {
			if len(h.description) > 0 {
				sb.WriteString("`!")
				sb.WriteString(h.commandString)
				sb.WriteString("` ")
				sb.WriteString(h.description)
				sb.WriteString("\n")
			}
		}
	}

	session.ChannelMessageSend(msg.ChannelID, sb.String())
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	guild, _ := s.State.Guild(m.GuildID)
	channel, _ := s.State.Channel(m.ChannelID)

	log.Printf("[%s|%s|%s#%s] (%s) %s\n", guild.Name, channel.Name, m.Author.Username, m.Author.Discriminator, m.ID, m.Content)

	if strings.HasPrefix(m.Content, "!") {
		message := strings.SplitN(m.Content, " ", 2)
		cmd := strings.TrimPrefix(message[0], "!")

		log.Printf("Trying to find %s command for %s", cmd, m.Author.String())

		if handler, ok := commandMap[cmd]; ok {
			log.Printf("Found %s command for %s", cmd, m.Author.String())

			if handler.modOnly && userAllowedAdminBotCommands(s, m.GuildID, m.ChannelID, m.Author.ID) == false {
				log.Printf("User %s tried to use command %s but is not allowed (not a MOD)", m.Author.String(), cmd)
				s.ChannelMessageSend(m.ChannelID, "Sorry, but we're not that type of friends </3")
				return
			}

			log.Printf("Running %s command handler for %s", cmd, m.Author.String())
			handler.handleFunc(s, m)
			return
		}
	}

	for _, h := range messageStreamHandlers {
		h(s, m)
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
