package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

var (
	modQueueChannel *discordgo.Channel
	logsChannel     *discordgo.Channel
	infoChannel     *discordgo.Channel
)

func setupAdminGuild(s *discordgo.Session, guild *discordgo.Guild) {
	fmt.Println("Setting up channels")

	var everyoneRole *discordgo.Role
	var modRole *discordgo.Role

	for _, r := range guild.Roles {
		if r.Name == "@everyone" {
			everyoneRole = r
		}

		if r.Name == "mod" || r.Name == "Mod" {
			modRole = r
		}
	}

	var category *discordgo.Channel
	var err error

	overwrites := []*discordgo.PermissionOverwrite{
		{
			ID:    s.State.User.ID,
			Allow: discordgo.PermissionAll,
			Type:  "member",
		},
		{
			ID:    modRole.ID,
			Allow: discordgo.PermissionReadMessages | discordgo.PermissionAddReactions | discordgo.PermissionManageMessages,
			Type:  "role",
		},
		{
			ID:   everyoneRole.ID,
			Type: "role",
			Deny: discordgo.PermissionAll,
		},
	}

	for _, c := range guild.Channels {
		if c.Name == "vpbot" && c.Type == discordgo.ChannelTypeGuildCategory {
			edit := discordgo.ChannelEdit{
				PermissionOverwrites: overwrites,
			}
			category, err = s.ChannelEditComplex(c.ID, &edit)
			if err != nil {
				log.Println("Could not edit the VPBot category for administration", err)
				return
			}
		}
	}
	if category == nil {
		categoryData := discordgo.GuildChannelCreateData{
			Name:                 "vpbot",
			Type:                 discordgo.ChannelTypeGuildCategory,
			PermissionOverwrites: overwrites,
		}

		category, err = s.GuildChannelCreateComplex(guild.ID, categoryData)
		if err != nil {
			log.Println("Could not setup the VPBot category for administration", err)
			return
		}
	}

	modQueueChannel = setupTextChannel(s, guild, "mod-queue", category.ID)
	logsChannel = setupTextChannel(s, guild, "logs", category.ID)
	if logsChannel != nil {
		log.SetFlags(log.Lshortfile)
		logger := discordLogger{
			session:       discord,
			logsChannelID: logsChannel.ID,
		}
		log.SetOutput(logger)
	}
	infoChannel = setupTextChannel(s, guild, "info", category.ID)
}

func setupTextChannel(s *discordgo.Session, guild *discordgo.Guild, name string, parentID string) *discordgo.Channel {
	for _, c := range guild.Channels {
		if c.Name == name && c.Type == discordgo.ChannelTypeGuildText {
			if len(parentID) > 0 && c.ParentID != parentID {
				edit := discordgo.ChannelEdit{
					ParentID: parentID,
				}

				channel, err := s.ChannelEditComplex(c.ID, &edit)
				if err != nil {
					log.Println("Could not edit", name, err)
				}
				return channel
			}

			return c
		}
	}

	data := discordgo.GuildChannelCreateData{
		Name:     name,
		Type:     discordgo.ChannelTypeGuildText,
		ParentID: parentID,
	}

	newChannel, err := s.GuildChannelCreateComplex(guild.ID, data)
	if err != nil {
		log.Println("Could not create", name, err)
	}
	return newChannel
}
