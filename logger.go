package main

import (
	"github.com/bwmarrin/discordgo"
)

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
