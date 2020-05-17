package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
)

// THESE VALUES IS SET A BULID TIME
var buildTimeStr = "DEV"
var versionStr = "DEV"

func versionCommandHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	session.ChannelMessageSend(msg.ChannelID, fmt.Sprintf("*Opens heart*\n`Version: %s`\n`Build Time: %s`", versionStr, buildTimeStr))
}
