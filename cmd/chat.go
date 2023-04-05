package cmd

import (
	"bufio"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	tui "github.com/imfing/gptui/pkg/chat"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
	"os"
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
		message := viper.GetString("message")
		// Read the input from the pipe
		if len(message) == 0 {
			stat, err := os.Stdin.Stat()
			if err != nil {
				log.Fatal(err)
			}
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					message += scanner.Text()
				}
			}
			viper.Set("message", message)
		}

		// start TUI
		if _, err := tea.NewProgram(tui.NewModel()).Run(); err != nil {
			fmt.Println("Error running program:", err)
			os.Exit(1)
		}
	},
}

func init() {
	chatCmd.Flags().String("model", chatModel, "Model to use.")
	chatCmd.Flags().String("endpoint", chatEndpoint, "Chat completion API endpoint.")
	chatCmd.Flags().StringP("message", "m", "", "Message for ChatGPT.")

	err := viper.BindPFlags(chatCmd.Flags())
	if err != nil {
		log.Fatal(err)
	}

	rootCmd.AddCommand(chatCmd)
}
