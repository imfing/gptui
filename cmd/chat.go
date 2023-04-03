package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	tui "github.com/imfing/gptui/pkg/chat"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
)

const (
	chatModel    = "gpt-3.5-turbo"
	chatEndpoint = "https://api.openai.com/v1/chat/completions"
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "ChatGPT Terminal UI",
	Long:  `Given a chat conversation, the model will return a chat completion response.`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: support pipe in message
		if _, err := tea.NewProgram(tui.NewModel()).Run(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	chatCmd.Flags().StringP("model", "m", chatModel, "Model to use.")
	chatCmd.Flags().String("endpoint", chatEndpoint, "Chat completion API endpoint.")

	err := viper.BindPFlags(chatCmd.Flags())
	if err != nil {
		log.Fatal(err)
	}

	rootCmd.AddCommand(chatCmd)
}
