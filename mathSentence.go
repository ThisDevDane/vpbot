package main

import (
	"database/sql"
	"github.com/bwmarrin/discordgo"
	"log"
	"strings"
)

var (
	queryRandomMathSentence  *sql.Stmt
	insertRandomMathSentence *sql.Stmt
)

func initMathSentence(db *sql.DB) {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS math_sentence (id INTEGER PRIMARY KEY, sentence TEXT)")
	if err != nil {
		log.Panic(err)
	}

	queryRandomMathSentence = dbPrepare(db, "SELECT sentence FROM math_sentence ORDER BY random() LIMIT 1")
	insertRandomMathSentence = dbPrepare(db, "INSERT INTO math_sentence (sentence) VALUES (?)")
}

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
