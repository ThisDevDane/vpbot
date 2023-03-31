package cmd

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/bwmarrin/discordgo"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/thisdevdane/vpbot/cmd/shared"
	"github.com/thisdevdane/vpbot/internal/gateway"
)

var (
	channelId string
	botToken  string

	dbHost string
	dbDb   string
	dbUser string
	dbPass string
)

var UsertrackCmd = &cobra.Command{
	Use: "usertrack",
	Run: func(cmd *cobra.Command, _ []string) {
		gatewayOpts := gateway.GatewayOpts{
			Host:     shared.RedisAddr,
			Password: shared.RedisPassword,
		}
		outgoingGateway := gateway.CreateClient(cmd.Context(), gatewayOpts)
		defer outgoingGateway.Close()

		session, err := discordgo.New("Bot " + botToken)
		if err != nil {
			panic(err)
		}

		session.Identify.Intents = discordgo.IntentsGuilds
		session.StateEnabled = true

		err = session.Open()
		defer session.Close()
		if err != nil {
			log.Fatal().Msgf("error opening connection,", err)
		}

		userdb := prepareDb()
		postUserCount(session, userdb, outgoingGateway)
		postUserGraph(userdb, outgoingGateway)
	},
}

type userDb struct {
	db *sql.DB

	insertUserTrackData              *sql.Stmt
	queryUserTrackDataByGuildAndDate *sql.Stmt
	queryLastYearsData               *sql.Stmt
}

func prepareDb() (u *userDb) {
	u = new(userDb)
	db, err := sql.Open("postgres", fmt.Sprintf("host=%s port=5432 user=%s password=%s dbname=%s sslmode=disable", dbHost, dbUser, dbPass, dbDb))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open connection to db")
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS user_track_data (id SERIAL PRIMARY KEY, guild_id TEXT, week_number INT, year INT, user_count INT)")
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create table")
	}

	u.insertUserTrackData = stmtPrepare(db, "INSERT INTO user_track_data (guild_id, week_number, year, user_count) VALUES ($1, $2, $3, $4)")
	u.queryUserTrackDataByGuildAndDate = stmtPrepare(db, "SELECT user_count FROM user_track_data WHERE guild_id = $1 AND week_number = $2 AND year = $3")
	u.queryLastYearsData = stmtPrepare(db, `SELECT year, week_number, user_count FROM user_track_data
                                            ORDER BY year DESC, week_number DESC
                                            LIMIT 26`)

	return
}

func stmtPrepare(db *sql.DB, query string) *sql.Stmt {
	stmt, err := db.Prepare(query)
	if err != nil {
		log.Fatal().Err(err).Str("stmt", query).Msg("failed to prepare stmt")
	}

	return stmt
}

func postUserCount(s *discordgo.Session, udb *userDb, out *gateway.Client) {
	ch, _ := s.Channel(channelId)
	guild, err := s.State.Guild(ch.GuildID)
	if err != nil {
		log.Fatal().Err(err).Str("guild_id", ch.GuildID).Msg("failed to get guild from discord")
		return
	}

	currentUserCount := guild.MemberCount
	currentYear, currentWeek := time.Now().UTC().ISOWeek()
	queryWeek := currentWeek - 1
	queryYear := currentYear
	if currentWeek <= 0 {
		queryYear--
		queryWeek = 52
	}

	_, err = udb.insertUserTrackData.Exec(guild.ID, currentWeek, currentYear, currentUserCount)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to insert user count")
	}
	queryUserCountResult := int(0)

	err = udb.queryUserTrackDataByGuildAndDate.QueryRow(guild.ID, queryWeek, queryYear).Scan(&queryUserCountResult)
	if err != nil && err != sql.ErrNoRows {
		log.Fatal().Err(err).Msg("failed to query previous user count")
	}

	diff := guild.MemberCount - queryUserCountResult
	percent := float32(diff) / float32(queryUserCountResult) * 100

	symbol := "up"
	if percent < 0 {
		symbol = "down"
	}

	out.PublishMessage(gateway.GatewayOutChannel, gateway.OutgoingMsg{
		ChannelID: channelId,
		Content: fmt.Sprintf("User count in week %v %v: %v (%s %.2f%%) (last week: %v)",
			currentWeek,
			currentYear,
			guild.MemberCount,
			symbol,
			percent,
			queryUserCountResult),
	})

}

const GRAPH_WIDTH = 52.0

type userCountValue struct {
	Year  int
	Week  int
	Count int
}

func postUserGraph(udb *userDb, out *gateway.Client) {
	rows, err := udb.queryLastYearsData.Query()
	if err != nil {
		log.Error().Stack().Err(err).Msg("couldn't retrieve last years data")
		return
	}
	defer rows.Close()

	dataPoints := []userCountValue{}
	for rows.Next() {
		d := userCountValue{}
		err = rows.Scan(&d.Year, &d.Week, &d.Count)
		if err != nil {
			log.Error().Stack().Err(err).Msg("couldn't iterate last years data")
		}
		dataPoints = append(dataPoints, d)
	}
	err = rows.Err()
	if err != nil {
		log.Error().Stack().Err(err).Msg("couldn't iterate last years data")
	}

	var sb strings.Builder
	sb.WriteString("```")

	lastUserCount := 0
	min, max := getMinMax(dataPoints)
	diffAccum := 0
	perAccum := float32(0)
	for i := len(dataPoints) - 1; i >= 0; i-- {
		v := dataPoints[i]
		sb.WriteString(fmt.Sprintf("Y%dW%02d: ", v.Year, v.Week))
		p := float32(v.Count-min) / float32(max-min)
		width := int(p * GRAPH_WIDTH)

		for i := 0; i < width+1; i++ {
			sb.WriteRune('#')
		}
		sb.WriteString(fmt.Sprintf(" (%d) ", v.Count))

		if lastUserCount == 0 {
			lastUserCount = v.Count
			sb.WriteString("\n")
			continue
		}

		diff := v.Count - lastUserCount
		diffAccum += diff
		percent := float32(diff) / float32(lastUserCount) * 100
		perAccum += float32(percent)

		symbol := "up"
		if percent < 0 {
			symbol = "down"
		}

		sb.WriteString(fmt.Sprintf("%s %.2f%%", symbol, percent))
		lastUserCount = v.Count

		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("total change: %d avg: %d avg_p: %.2f", diffAccum, diffAccum/(len(dataPoints)-1), perAccum/(float32(len(dataPoints))-1)))
	sb.WriteString("```")
	out.PublishMessage(gateway.GatewayOutChannel, gateway.OutgoingMsg{
		ChannelID: channelId,
		Content:   sb.String(),
	})
}

func getMinMax(points []userCountValue) (min int, max int) {
	min = math.MaxInt
	max = 0

	for _, v := range points {
		if v.Count < min {
			min = v.Count
		}

		if v.Count > max {
			max = v.Count
		}
	}

	return
}

func init() {
	UsertrackCmd.Flags().StringVar(&channelId, "channel-id", "", "The ID of the channel to moderate as a showcase channel")
	UsertrackCmd.MarkFlagRequired("channel-id")
	UsertrackCmd.Flags().StringVar(&botToken, "token", "", "Bot token for connecting to Discord Gateway")
	UsertrackCmd.MarkFlagRequired("token")

	UsertrackCmd.Flags().StringVar(&dbHost, "db-host", "", "The host for the database to store stats")
	UsertrackCmd.MarkFlagRequired("db-host")
	UsertrackCmd.Flags().StringVar(&dbDb, "db-db", "vpbot", "The database for the database to store stats")
	UsertrackCmd.Flags().StringVar(&dbUser, "db-user", "", "The user for the database to store stats")
	UsertrackCmd.MarkFlagRequired("db-user")
	UsertrackCmd.Flags().StringVar(&dbPass, "db-pass", "", "The pass for the database to store stats")
	UsertrackCmd.MarkFlagRequired("db-pass")
}
