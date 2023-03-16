package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var (
	queryRandomMathSentence  *sql.Stmt
	insertRandomMathSentence *sql.Stmt
)

func initMathSentence(db *sql.DB) {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS math_sentence (id SERIAL PRIMARY KEY, sentence TEXT)")
	if err != nil {
		log.Panic(err)
	}

	queryRandomMathSentence = dbPrepare(db, "SELECT sentence FROM math_sentence ORDER BY random() LIMIT 1")
	insertRandomMathSentence = dbPrepare(db, "INSERT INTO math_sentence (sentence) VALUES ($1)")
}

func msgStreamMathMessageHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if len(msg.Mentions) > 0 {
		for _, mention := range msg.Mentions {
			if mention.ID == session.State.User.ID {
				str := strings.ToLower(msg.Content)
				if strings.Contains(str, "math") {

					var sentence string
					row := queryRandomMathSentence.QueryRow()
					err := row.Scan(&sentence)
					if err == sql.ErrNoRows {
						sentence = "MATH IS THE WORST THING ON EARH"
					}

					recepient := msg.Author

					if len(msg.Mentions) > 1 {
						recepient = msg.Mentions[1]
					}

					resp := fmt.Sprintf("%s %s", recepient.Mention(), sentence)
					session.ChannelMessageSend(msg.ChannelID, resp)
				}
				break
			}
		}
	}
}

func addMathSentenceHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	sentence := strings.TrimPrefix(msg.Content, "!addmathsentence")
	sentence = strings.TrimSpace(sentence)
	if len(sentence) <= 1 {
		session.ChannelMessageSend(msg.ChannelID, "Remember to include sentence in command...")
		return
	}
	insertRandomMathSentence.Exec(sentence)
	session.ChannelMessageSend(msg.ChannelID, "Added sentence to set! o7")
}
