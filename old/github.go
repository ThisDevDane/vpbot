package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"

	"github.com/bwmarrin/discordgo"
)

var (
	shurrupRegex  *regexp.Regexp
	githubChannel *discordgo.Channel
	githubMentionRole *discordgo.Role

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

func initGithubChannel(s *discordgo.Session) {
	channelId := os.Getenv("VPBOT_GITHUB_CHANNEL")
	MentionRoleId := os.Getenv("VPBOT_GITHUB_MENTION_ROLE")

	if len(channelId) <= 0 {
		return
	}

	var err error
	githubChannel, err = s.Channel(channelId)

	if err != nil {
		log.Printf("Couldn't find the github channel with ID: %s, %s", channelId, err)
		return
	}

	githubMentionRole, err = s.State.Role(guildID, MentionRoleId)
	if err != nil {
		log.Printf("Couldn't find the github mention role with ID: %s %s", MentionRoleId, err)
		githubChannel = nil
		return
	}

	shurrupRegex, _ = regexp.Compile(shurrupRegexString)
}

func githubWebhookHandler(w http.ResponseWriter, req *http.Request) {
	if githubChannel == nil {
		return
	}

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


	jobName := unwrapJson(data, "check_run", "name").(string)
	url := unwrapJson(data, "check_run", "details_url").(string)
	commitSha := unwrapJson(data, "check_run", "check_suite", "head_sha").(string)

	msg := fmt.Sprintf("CI job '%s' is failing again... Somebody messed up... Wonder who... *eyes BDFL* (commit: %s) %s\n Link: <%s>",
		jobName,
		commitSha,
		githubMentionRole.Mention(),
		url)

	discord.ChannelMessageSend(githubChannel.ID, msg)
}

func unwrapJson(obj map[string]interface{}, keys ...string) interface{} {
	root := obj
	for idx, k := range keys {
		if idx+1 == len(keys) {
			return root[k]
		}
		root = root[k].(map[string]interface{})
	}

	return root
}

func msgStreamGithubMessageHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if githubChannel == nil {
		return
	}

	if msg.ChannelID != githubChannel.ID {
		return
	}

	if shurrupRegex.MatchString(msg.Content) {
		resp := fmt.Sprintf("%s %s", msg.Author.Mention(), snarkyComeback[rand.Int31n(int32(len(snarkyComeback)))])
		session.ChannelMessageSend(msg.ChannelID, resp)
	}
}
