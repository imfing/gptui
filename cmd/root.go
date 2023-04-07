package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const BaseURL = "https://api.openai.com/v1"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gptui",
	Short: "Terminal UI for OpenAI GPT",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("openai-api-key", "", "OpenAI API Key")
	rootCmd.PersistentFlags().String("base-url", BaseURL, "Base URL for OpenAI API")
}

func initConfig() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	viper.BindPFlags(rootCmd.PersistentFlags())
}
