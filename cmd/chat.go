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

const defaultModel = "gpt-3.5-turbo"

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
	chatCmd.Flags().String("model", defaultModel, "model to use for chat completion")
	chatCmd.Flags().StringP("message", "m", "", "message for the chat input")
	chatCmd.Flags().String("system", "", "system message that helps set the behavior of the assistant")
	chatCmd.Flags().Int("max-context-length", 1024, "maximum number of tokens for GPT context")
	chatCmd.Flags().String("history", "", "path to conversation history file to restore from")
	chatCmd.Flags().Bool("stream", true, "if set, partial message deltas will be sent, like in ChatGPT")

	err := viper.BindPFlags(chatCmd.Flags())
	if err != nil {
		log.Fatal(err)
	}

	rootCmd.AddCommand(chatCmd)
}
