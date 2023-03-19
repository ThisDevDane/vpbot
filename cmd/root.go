package cmd

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var developmentLogging bool
var rootCmd = &cobra.Command{
	Use: "vpbot",
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		if developmentLogging {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		}
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&developmentLogging, "dev-logging", false, "Enable pretty printing to the console for development")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
