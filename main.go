package main

import (
	"database/sql"
	"flag"
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
)

var (
	token    string
	urlRegex *regexp.Regexp
	db       *sql.DB

	insertPoliceChannel           *sql.Stmt
	queryPoliceChannel            *sql.Stmt
	deletePoliceChannel           *sql.Stmt
	queryAllPoliceChannelForGuild *sql.Stmt
)

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

	statement, _ := db.Prepare("CREATE TABLE IF NOT EXISTS police_channels (id INTEGER PRIMARY KEY, guild_id TEXT, channel_id TEXT)")
	statement.Exec()

	insertPoliceChannel, _ = db.Prepare("INSERT INTO police_channels (guild_id, channel_id) VALUES (?, ?)")
	deletePoliceChannel, _ = db.Prepare("DELETE FROM police_channels WHERE channel_id = ?")
	queryPoliceChannel, _ = db.Prepare("SELECT guild_id, channel_id FROM police_channels WHERE channel_id = ?")
	queryAllPoliceChannelForGuild, _ = db.Prepare("SELECT channel_id FROM police_channels WHERE guild_id = ?")

	discord, err := discordgo.New("Bot " + token)
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
			}

			if strings.HasPrefix(m.Content, "!police") {
				if policeChannel(s, m.ChannelID, m.Author) {
					s.ChannelMessageSend(m.ChannelID, "Policing channel. o7")
				} else {
					s.ChannelMessageSend(m.ChannelID, "Channel already policed. o7")
				}
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
			}

			if strings.HasPrefix(m.Content, "!unpolice") {
				if unpoliceChannel(s, m.ChannelID, m.Author) {
					s.ChannelMessageSend(m.ChannelID, "Stopping policing channel. o7")
				} else {
					s.ChannelMessageSend(m.ChannelID, "Channel not policed!")
				}
			}

			if strings.HasPrefix(m.Content, "!usercount") {
				guild, _ := s.State.Guild(m.GuildID)
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Current user count: %d", guild.MemberCount))
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
					msg := fmt.Sprintf("%s MATH IS THE WORST THING ON EARH", m.Author.Mention())
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
