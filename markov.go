package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/go-co-op/gocron"
	"github.com/mb-14/gomarkov"
)

var (
	insertMarkovVersion *sql.Stmt
	chain               *gomarkov.Chain
)

func initMarkov(db *sql.DB, scheduler *gocron.Scheduler) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS markov (id INTEGER PRIMARY KEY, create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP, json TEXT)`)
	if err != nil {
		log.Panic(err)
	}

	insertMarkovVersion = dbPrepare(db, "INSERT INTO markov (json) VALUES (?)")

	chain = GetMarkovChain()

	scheduler.Every(2).Hours().Do(saveMarkovChain)
}

func markovForceSave(session *discordgo.Session, msg *discordgo.MessageCreate) {
	saveMarkovChain()
	session.ChannelMessageSend(msg.ChannelID, "Saved chain...")
}

func markovForceSay(session *discordgo.Session, msg *discordgo.MessageCreate) {
	content, _ := msg.ContentWithMoreMentionsReplaced(session)

	sentence := strings.TrimPrefix(content, "!markovsay")
	sentence = strings.TrimSpace(sentence)

	message := markovGenerateMessage()
	session.ChannelMessageSend(msg.ChannelID, message)
}

func GetMarkovChain() *gomarkov.Chain {
	var result *gomarkov.Chain
	row := db.QueryRow("SELECT json FROM markov ORDER BY create_time DESC LIMIT 1")
	var model string
	err := row.Scan(&model)
	if err != nil {
		log.Println("No markov model found in DB, creating fresh one")
		result = gomarkov.NewChain(2)
	} else {
		err = json.Unmarshal([]byte(model), &result)
		if err != nil {
			log.Println("Error loading latest markov model in DB, creating fresh one")
			result = gomarkov.NewChain(2)
		} else {
			log.Println("Loaded markov model from DB!")
		}
	}

	return result
}

func saveMarkovChain() {
	json, _ := chain.MarshalJSON()
	insertMarkovVersion.Exec(string(json))
}

func msgStreamMarkovTrainHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if msg.Author.ID == session.State.User.ID {
		return
	}

	content, err := msg.ContentWithMoreMentionsReplaced(session)
	if err != nil {
		log.Printf("Couldn't replace all mentions in '%s'\n", content)
	}

	data := strings.Split(content, " ")
	//log.Printf("Adding %v to chain", data)
	chain.Add(data)
}

func msgStreamMarkovSayHandler(session *discordgo.Session, msg *discordgo.MessageCreate) {
	if msg.Author.ID == session.State.User.ID {
		return
	}

	//for _, mention := range msg.Mentions {
	//	if mention.ID == session.State.User.ID {
	//		if rand.Float32() > 0.75 {
	//			markovMsg := markovGenerateMessage()
	//			session.ChannelMessageSend(msg.ChannelID, markovMsg)
	//			return
	//		}
	//	}
	//}

	//if rand.Float32() > 0.95 {
	//	markovMsg := markovGenerateMessage()
	//	//session.ChannelMessageSend(msg.ChannelID, markovMsg)
	//	log.Printf("Markov would have said; %s", markovMsg)
	//}
}

func markovGenerateMessage() string {
	tokens := []string{gomarkov.StartToken, gomarkov.StartToken}
	for tokens[len(tokens)-1] != gomarkov.EndToken {
		next, _ := chain.Generate(tokens[(len(tokens) - 1):])
		tokens = append(tokens, next)
	}
	return strings.Join(tokens[1:len(tokens)-1], " ")
}
