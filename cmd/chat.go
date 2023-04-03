package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// chatCmd represents the chat command
var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "ChatGPT Terminal UI",
	Long:  `Given a chat conversation, the model will return a chat completion response.`,
	Run: func(cmd *cobra.Command, args []string) {
		p := tea.NewProgram(initialModel())

		if _, err := p.Run(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	chatCmd.Flags().StringP("model", "m", "gpt-3.5-turbo", "Model to use. Default is gpt-3.5-turbo.")
	viper.BindPFlags(chatCmd.Flags())

	rootCmd.AddCommand(chatCmd)
}

type model struct {
	viewport    viewport.Model
	messages    []string
	textarea    textarea.Model
	senderStyle lipgloss.Style
	err         error
	waiting     bool
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.EnterAltScreen)
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 0

	ta.SetWidth(50)
	ta.SetHeight(3)

	// Remove cursor line styling
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	ta.ShowLineNumbers = false

	ta.KeyMap.InsertNewline.SetKeys("enter")

	vp := viewport.New(50, 5)
	vp.SetContent(fmt.Sprintf(
		"%s\n\n%s%s\n%s",
		"Welcome to use gptui Chat",
		"You are using model: ",
		viper.GetString("model"),
		"Type a message and press Enter to send."))

	return model{
		textarea:    ta,
		messages:    []string{},
		viewport:    vp,
		senderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		err:         nil,
		waiting:     false,
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			fmt.Println(m.textarea.Value())
			return m, tea.Quit
		case tea.KeyEnter:
			input := m.textarea.Value()
			m.messages = append(m.messages, m.senderStyle.Render("You: ")+input)
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.textarea.Reset()
			m.viewport.GotoBottom()
			m.waiting = true
			return m, sendChatCompletionRequest(viper.GetString("openai-api-key"), viper.GetString("model"), input)
		}

	case ChatCompletionResponse:
		m.waiting = false
		m.messages = append(m.messages, "ChatGPT: "+msg.Choices[0].Message.Content)
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	// We handle errors just like any other message
	case error:
		// TODO: properly display error
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

type ChatCompletionResponse struct {
	ID      string `json:"id,omitempty"`
	Object  string `json:"object,omitempty"`
	Created int64  `json:"created,omitempty"`
	Choices []struct {
		Index        int `json:"index,omitempty"`
		Message      Message
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices,omitempty"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens,omitempty"`
		CompletionTokens int `json:"completion_tokens,omitempty"`
		TotalTokens      int `json:"total_tokens,omitempty"`
	} `json:"usage,omitempty"`
}

type ChatCompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func sendChatCompletionRequest(token string, model string, message string) tea.Cmd {
	return func() tea.Msg {
		// if length of token is zero, return error
		if len(token) == 0 {
			return error(fmt.Errorf("OpenAI API key is not set. Please set it with the --openai-api-key flag"))
		}

		// TODO: make url a flag
		// url := "https://api.openai.com/v1/chat/completions"
		url := "http://localhost:8081"

		chatCompletionRequest := ChatCompletionRequest{
			Model: model,
			Messages: []Message{
				{Role: "user", Content: message},
			},
		}

		payload, err := json.Marshal(chatCompletionRequest)
		if err != nil {
			return error(err)
		}

		req, _ := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		client := &http.Client{
			Timeout: 10 * time.Second,
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}

		if resp.StatusCode != http.StatusOK {
			return error(fmt.Errorf("status code: %d, body: %s", resp.StatusCode, string(body)))
		}

		var chatCompletion ChatCompletionResponse
		if err := json.Unmarshal(body, &chatCompletion); err != nil {
			return error(err)
		}

		return chatCompletion
	}
}

func (m model) View() string {
	helpText := "(ctrl+c to quit)"
	secondLine := m.textarea.View()
	if m.waiting {
		secondLine = "Waiting for response"
	}

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		m.viewport.View(),
		secondLine,
		helpText,
	) + "\n\n"
}
