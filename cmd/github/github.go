package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/thisdevdane/vpbot/cmd/shared"
	"github.com/thisdevdane/vpbot/internal/gateway"
)

var (
	channelId       string
	roleId          string
	httpPort        int
	rdb             *redis.Client
	urlRegex        *regexp.Regexp
	outgoingGateway *gateway.Client
)

var GithubCmd = &cobra.Command{
	Use: "github",
	Run: func(cmd *cobra.Command, _ []string) {
		gatewayOpts := gateway.GatewayOpts{
			Host:     shared.RedisAddr,
			Password: shared.RedisPassword,
		}
		redisOpts := redis.Options{
			Addr:     shared.RedisAddr,
			Password: shared.RedisPassword,
			DB:       0,
		}

		outgoingGateway = gateway.CreateClient(cmd.Context(), gatewayOpts)
		rdb = redis.NewClient(&redisOpts)

		http.HandleFunc("/github-webhook", githubWebhookHandler)
		go func() {
			log.Info().Msgf("opened http listenner on port %d", httpPort)
			if err := http.ListenAndServe(fmt.Sprintf(":%d", httpPort), nil); err != nil {
				log.Fatal().Err(err).Msg("fatal error in http listener, shutting down")
			}
		}()

		go func() {
		outer:
			for {
				rdb.HSet(cmd.Context(), "cmd_info:github", "version", cmd.Version, "date", "TODO")
				rdb.Expire(cmd.Context(), "cmd_info:github", 5*time.Second)
				ctx, cancel := context.WithTimeout(cmd.Context(), 1*time.Second)
				defer cancel()
				select {
				case <-cmd.Context().Done():
					break outer

				case <-ctx.Done():
					continue
				}
			}
		}()

		log.Info().Msgf("github is now running on ID %s. Press CTRL-C to exit.", channelId)
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
		<-sc

		outgoingGateway.Close()
	},
}

type githubWebhookCheckRun struct {
	Action   string `json:"action"`
	CheckRun struct {
		Conclusion string `json:"conclusion"`
		Name       string `json:"name"`
		DetailsURL string `json:"details_url"`
		CheckSuite struct {
			ID         int    `json:"id"`
			HeadSha    string `json:"head_sha"`
			HeadBranch string `json:"head_branch"`
		} `json:"check_suite"`
	} `json:"check_run"`
}

func githubWebhookHandler(w http.ResponseWriter, req *http.Request) {
	event := req.Header.Get("X-Github-Event")
	log.Info().Msgf("got event: %s", event)
	if event != "check_run" {
		w.WriteHeader(http.StatusOK)
		return
	}

	run := githubWebhookCheckRun{}
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&run)
	if err != nil {
		log.Error().Err(err).Send()
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Trace().Msgf("decoded %+v", run)

	if !isFailedRun(run) {
		w.WriteHeader(http.StatusOK)
		return
	}

	failed, err := pushAndGetCache(req.Context(), run, run.CheckRun.CheckSuite.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var sb strings.Builder
	for _, v := range failed {
		sb.WriteString(" - ")
		sb.WriteString(v)
		sb.WriteString("\n")
	}

	msg := fmt.Sprintf("CI is failing again... Somebody messed up... Wonder who... *eyes BDFL* (commit: %s) %s\nFailed jobs:\n%s",
		run.CheckRun.CheckSuite.HeadSha,
		fmt.Sprintf("<@&%s>", roleId),
		sb.String())

	internalId := fmt.Sprintf("github:suite:%d:msg", run.CheckRun.CheckSuite.ID)
	outgoingGateway.PublishMessage(gateway.GatewayOutChannel, gateway.OutgoingMsg{
		InternalID: &internalId,
		ChannelID:  channelId,
		Content:    msg,
	})
	w.WriteHeader(http.StatusOK)
}

func isFailedRun(run githubWebhookCheckRun) bool {
	if run.CheckRun.CheckSuite.HeadBranch != "master" {
		return false
	}

	log.Info().Msgf("action status: %s", run.Action)
	if run.Action != "completed" {
		return false
	}

	if run.CheckRun.Conclusion != "failure" {
		return false
	}

	return true
}

func pushAndGetCache(ctx context.Context, run githubWebhookCheckRun, suiteID int) ([]string, error) {
	err := pushFailedToCache(ctx, run, suiteID)
	if err == nil {
		return getFailedFromCache(ctx, suiteID)
	}

	return nil, err
}

func pushFailedToCache(ctx context.Context, run githubWebhookCheckRun, suiteID int) error {
	setKey := fmt.Sprintf("github:suite:%d:failed", suiteID)
	_, err := rdb.SAdd(ctx, setKey, fmt.Sprintf("%s: <%s>", run.CheckRun.Name, run.CheckRun.DetailsURL)).Result()
	rdb.Expire(ctx, setKey, 48*time.Hour)
	if err != nil {
		log.Error().Err(err).Str("key", setKey).Msg("failed to add member to set")
	}

	return err
}

func getFailedFromCache(ctx context.Context, suiteID int) ([]string, error) {
	setKey := fmt.Sprintf("github:suite:%d:failed", suiteID)
	failed, err := rdb.SMembers(ctx, setKey).Result()
	if err != nil {
		log.Error().Err(err).Str("key", setKey).Msg("failed to retrieve set members")
	}

	return failed, err
}

func init() {
	GithubCmd.Flags().StringVar(&channelId, "channel-id", "", "The ID of the channel to moderate as a showcase channel")
	GithubCmd.MarkFlagRequired("channel-id")
	GithubCmd.Flags().StringVar(&roleId, "role-id", "", "The ID of the role to mention")
	GithubCmd.MarkFlagRequired("role-id")
	GithubCmd.Flags().IntVar(&httpPort, "http-port", 8080, "Port to listen for webhooks on")
}
