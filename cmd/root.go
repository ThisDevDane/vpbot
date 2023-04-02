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
	snark "github.com/thisdevdane/vpbot/cmd/snark"
	usertrack "github.com/thisdevdane/vpbot/cmd/usertrack"
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

	addCmd(gateway.GatewayCmd)
	addCmd(showcase.ShowcaseCmd)
	addCmd(github.GithubCmd)
	addCmd(usertrack.UsertrackCmd)
	addCmd(snark.SnarkCmd)
}

func addCmd(cmd *cobra.Command) {
	cmd.Version = rootCmd.Version
	rootCmd.AddCommand(cmd)

}

func Execute(version string) {
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Send()
	}
}
