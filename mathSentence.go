package main

import (
	"github.com/bwmarrin/discordgo"
	"strings"
)

func addMathSentence(session *discordgo.Session, msg *discordgo.MessageCreate) {
	sentence := strings.TrimPrefix(msg.Content, "!addmathsentence")
	sentence = strings.TrimSpace(sentence)
	if len(sentence) <= 1 {
		session.ChannelMessageSend(msg.ChannelID, "Remember to include sentence in command...")
		return
	}
	insertRandomMathSentence.Exec(sentence)
	session.ChannelMessageSend(msg.ChannelID, "Added sentence to set! o7")
}
