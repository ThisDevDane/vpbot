package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var (
	insertGithubChannel        *sql.Stmt
	queryGithubChannelForGuild *sql.Stmt
	queryGithubChannelForRepo  *sql.Stmt

	shurrupRegex *regexp.Regexp

	snarkyComeback = []string{
		"Well if you wouldn't keep breaking it, I wouldn't have to yell at you!",
		"NO! YOU SHURRUP! I HATE U!",
		"When pigs fly",
		"Can you you stop breaking things then? hmm? HMMM? >:|",
		"Oh I'm sorry mister, I'm only pointing out __**your**__ stupid mistakes :)",
		"Stop yelling, that is __**MY**__ job!",
	}
)

const shurrupRegexString = "(?i)shurrup"

func initGithubChannel(db *sql.DB) {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS github_channel (id INTEGER PRIMARY KEY, guild_id TEXT, channel_id TEXT, role_id TEXT, repo_id TEXT)")
	if err != nil {
		log.Panic(err)
	}

	insertGithubChannel = dbPrepare(db,
		"INSERT INTO github_channel (guild_id, channel_id, repo_id, role_id) VALUES (?, ?, ?, ?)")
	queryGithubChannelForGuild = dbPrepare(db, "SELECT channel_id FROM github_channel WHERE guild_id = ?")
	queryGithubChannelForRepo = dbPrepare(db, "SELECT channel_id, role_id FROM github_channel WHERE repo_id = ?")

	shurrupRegex, _ = regexp.Compile(shurrupRegexString)
}

func githubWebhookHandler(w http.ResponseWriter, req *http.Request) {
	event := req.Header.Get("X-Github-Event")
	if event != "check_run" {
		return
	}

	decoder := json.NewDecoder(req.Body)
	var data map[string]interface{}
	err := decoder.Decode(&data)
	if err != nil {
		log.Panic(err)
		return
	}

	if data["action"].(string) != "completed" {
		return
	}

	if unwrapJson(data, "check_run", "check_suite", "head_branch").(string) != "master" {
		return
	}

	if unwrapJson(data, "check_run", "conclusion").(string) != "failure" {
		return
	}


	repoIDfloat := unwrapJson(data, "repository", "id").(float64)
	repoIDint := int(repoIDfloat)
	repoID := strconv.Itoa(repoIDint)

	chanID, roleID, ok := repoHasGithubChannel(repoID)
	if ok == false {
		return

	}


	jobName := unwrapJson(data, "check_run", "name").(string)
	url := unwrapJson(data, "check_run", "details_url").(string)
	commitSha := unwrapJson(data, "check_run", "check_suite", "head_sha").(string)

	msg := fmt.Sprintf("CI job '%s' is failing again... Somebody messed up... Wonder who... *eyes BDFL* (commit: %s) %s\n Link: %s",
		jobName,
		commitSha,
		roleID,
		url)
	discord.ChannelMessageSend(chanID, msg)
}

func unwrapJson(obj map[string]interface{}, keys ...string) interface{} {
	root := obj
	for _, k := range keys {
		root = root[k].(map[string]interface{})
	}

	return root
}

func msgStreamGithubMessageHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if shurrupRegex.MatchString(msg.Content) {
		resp := fmt.Sprintf("%s %s", msg.Author.Mention(), snarkyComeback[rand.Int31n(int32(len(snarkyComeback)))])
		session.ChannelMessageSend(msg.ChannelID, resp)
	}
}

func githubCommandHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	mesg := strings.TrimPrefix(msg.Content, "!githubchan")
	mesg = strings.TrimSpace(mesg)
	parts := strings.Split(mesg, " ")
	if setupGithubChannel(session, msg.ChannelID, parts[0], parts[1], msg.Author) {
		session.ChannelMessageSend(msg.ChannelID, "Using channel as github channel. o7")
	} else {
		session.ChannelMessageSend(msg.ChannelID, "Guild already has a github channel. o7")
	}
}

func setupGithubChannel(s *discordgo.Session,
	channelID string,
	repoID string,
	roleID string,
	user *discordgo.User) bool {
	channel, _ := s.State.Channel(channelID)
	guild, _ := s.State.Guild(channel.GuildID)

	if ok, _ := guildHasGithubChannel(guild.ID); ok {
		log.Printf("%s#%s tried to github channel in '%s' but guild '%s' already has one\n",
			user.Username,
			user.Discriminator,
			channel.Name,
			guild.Name)
		return false
	}

	insertGithubChannel.Exec(guild.ID, channel.ID, repoID, roleID)
	log.Printf("Setup github channel '%s'(%s) in '%s', requested by %s#%s\n",
		channel.Name,
		channel.ID,
		guild.Name,
		user.Username,
		user.Discriminator)

	return true
}

func guildHasGithubChannel(guildID string) (bool, string) {
	row := queryGithubChannelForGuild.QueryRow(guildID)
	var channelID string
	err := row.Scan(&channelID)
	if err == sql.ErrNoRows {
		return false, ""
	}

	return true, channelID
}

func repoHasGithubChannel(repoID string) (string, string, bool) {
	row := queryGithubChannelForRepo.QueryRow(repoID)
	var channelID string
	var roleMention string
	err := row.Scan(&channelID, &roleMention)
	if err == sql.ErrNoRows {
		return "", "", false
	}

	return channelID, roleMention, true
}
