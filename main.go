package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/go-co-op/gocron"
	_ "github.com/mattn/go-sqlite3"
)

var (
	token        string
	verbose      bool
	httpPort     int
	guildID      string

	urlRegex *regexp.Regexp
	db       *sql.DB

	discord *discordgo.Session

	commandMap            = make(map[string]commandHandler)
	messageStreamHandlers = make([]func(*discordgo.Session, *discordgo.MessageCreate), 0)
)

type commandHandler struct {
	commandString string
	description   string
	modOnly       bool
	handleFunc    func(*discordgo.Session, *discordgo.MessageCreate)
}

func init() {
	token = os.Getenv("VPBOT_TOKEN")
	guildID = os.Getenv("VPBOT_GUILD_ID")
	verbose, _ = strconv.ParseBool(os.Getenv("VPBOT_VERBOSE"))
	httpPort, _ = strconv.Atoi(os.Getenv("VPBOT_HTTP_PORT"))

	flag.StringVar(&token, "t", token, "Bot Token")
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

	cron := gocron.NewScheduler(time.UTC)

	discord, err = discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		os.Exit(1)
	}

	discord.StateEnabled = true

	log.Println("Opening up connection to discord...")
	err = discord.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
		os.Exit(1)
	}

	initPoliceChannel(discord)
	initMathSentence(db)
	initUserTracking(discord, db, cron)
	initIdeasChannel(discord)
	initGithubChannel(discord)
	//initOdin()
	//initMarkov(db, cron)

	discord.AddHandler(messageCreate)
	discord.AddHandler(discordReady)
	discord.AddHandler(ideasQueueReactionAdd)

	handleCommand("ack", "Will make bot say 'ACK'", false, discordAckHandler)
	handleCommand("help", "Will print a message with all available commands to the user", false, helpHandler)
	handleCommand("version", "Will print the version of VPBot", false, versionCommandHandler)

	handleCommand("usercount", "Post the current user count for this guild", true, userCountCommandHandler)

	handleCommand("addidea",
		"Suggest an idea to add to the server's idea channel, will go into a manual review queue before being posted",
		false,
		addIdeasHandler)

	handleCommand("addmathsentence",
		"Will add a math related sentence that VPBot can say, make sure to make them about hating math",
		false,
		addMathSentenceHandler)

	//handleCommand("odinrun", "Will compile an odin code block and run it", true, odinRunHandle)

	//handleCommand("markovsave", "Force a save of the markov chain", true, markovForceSave)
	//handleCommand("markovsay", "Force a message generation in markov", false, markovForceSay)

	addMessageStreamHandler(msgStreamMathMessageHandler)
	addMessageStreamHandler(msgStreamPoliceHandler)
	addMessageStreamHandler(msgStreamGithubMessageHandler)
	//addMessageStreamHandler(msgStreamMarkovTrainHandler)
	//addMessageStreamHandler(msgStreamMarkovSayHandler)

	setupHTTP()
	log.Printf("Starting HTTP server on port %d...\n", httpPort)
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil)
		if err != nil {
			panic("Unable to start HTTP server!")
		}
	}()

	log.Println("Starting CRON services...")
	cron.StartAsync()

	log.Println("VPBot is now running.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, syscall.SIGTERM)
	<-sc
	log.Println("VPBot is terminating...")

	cron.Stop()
	_ = discord.Close()

}

func setupHTTP() {
	log.Println("Setting up HTTP handlers")
	http.HandleFunc("/github-webhook", githubWebhookHandler)
	http.HandleFunc("/ack", ackHandler)
}

func ackHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = fmt.Fprintf(w, "ACK")
}

func addMessageStreamHandler(handler func(*discordgo.Session, *discordgo.MessageCreate)) {
	messageStreamHandlers = append(messageStreamHandlers, handler)
}

func handleCommand(cmdString string,
	desc string,
	modOnly bool,
	handler func(*discordgo.Session, *discordgo.MessageCreate)) {
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
	_, _ = session.ChannelMessageSend(msg.ChannelID, "ACK")
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

	_, _ = session.ChannelMessageSend(msg.ChannelID, sb.String())
}

func discordReady(s *discordgo.Session, _ *discordgo.Ready) {
	activity := discordgo.Activity{
		Name: "users for fools, one stupid message at a time",
		Type: discordgo.ActivityTypeGame,
		URL:  "",
	}
	usd := discordgo.UpdateStatusData{Status: "online", AFK: false, Activities: []*discordgo.Activity{&activity}}
	err := s.UpdateStatusComplex(usd)
	if err != nil {
		fmt.Println("error updating status on discord,", err)
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	guild, _ := s.State.Guild(m.GuildID)
	channel, _ := s.State.Channel(m.ChannelID)

	log.Printf("[%s|%s|%s#%s] (%s) %s\n",
		guild.Name,
		channel.Name,
		m.Author.Username,
		m.Author.Discriminator,
		m.ID,
		m.Content)

	if strings.HasPrefix(m.Content, "!") {
		message := strings.SplitN(m.Content, " ", 2)
		cmd := strings.TrimPrefix(message[0], "!")

		log.Printf("Trying to find %s command for %s", cmd, m.Author.String())

		if handler, ok := commandMap[cmd]; ok {
			log.Printf("Found %s command for %s", cmd, m.Author.String())

			if handler.modOnly && userAllowedAdminBotCommands(s, m.GuildID, m.ChannelID, m.Author.ID) == false {
				log.Printf("User %s tried to use command %s but is not allowed (not a MOD)", m.Author.String(), cmd)
				_, _ = s.ChannelMessageSend(m.ChannelID, "Sorry, but we're not that type of friends </3")
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
	perm, _ := s.UserChannelPermissions(userID, channelID)
	if perm&discordgo.PermissionAdministrator != 0 {
		return true
	}

	hasRole := false

	member, _ := s.GuildMember(guildID, userID)
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

	return hasRole
}
