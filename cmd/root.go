package cmd

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/spf13/cobra"
	gateway "github.com/thisdevdane/vpbot/cmd/gateway"
	github "github.com/thisdevdane/vpbot/cmd/github"
	"github.com/thisdevdane/vpbot/cmd/shared"
	showcase "github.com/thisdevdane/vpbot/cmd/showcase"
)

var developmentLogging bool
var rootCmd = &cobra.Command{
	Use: "vpbot",
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Help()
	},
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		if developmentLogging {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
			zerolog.SetGlobalLevel(zerolog.TraceLevel)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&developmentLogging, "dev-logging", false, "Enable pretty printing to the console for development")
	rootCmd.PersistentFlags().StringVar(&shared.RedisAddr, "redis-addr", "localhost:6379", "")
	rootCmd.PersistentFlags().StringVar(&shared.RedisPassword, "redis-pass", "", "")

	rootCmd.AddCommand(gateway.GatewayCmd)
	rootCmd.AddCommand(showcase.ShowcaseCmd)
	rootCmd.AddCommand(github.GithubCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Send()
	}
}
