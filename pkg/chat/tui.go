package chat

import (
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
	"strings"
)

type Model struct {
	Viewport    viewport.Model
	Messages    []string
	Textarea    textarea.Model
	SenderStyle lipgloss.Style
	Err         error
	Waiting     bool
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.EnterAltScreen)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.Textarea, tiCmd = m.Textarea.Update(msg)
	m.Viewport, vpCmd = m.Viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			fmt.Println(m.Textarea.Value())
			return m, tea.Quit
		case tea.KeyEnter:
			input := m.Textarea.Value()
			m.Messages = append(m.Messages, m.SenderStyle.Render("You: ")+input)
			m.Viewport.SetContent(strings.Join(m.Messages, "\n"))
			m.Textarea.Reset()
			m.Viewport.GotoBottom()
			m.Waiting = true
			return m, sendChatCompletionRequest(viper.GetString("endpoint"), viper.GetString("openai-api-key"), viper.GetString("ChatModel"), input)
		}

	case CompletionResponse:
		m.Waiting = false
		m.Messages = append(m.Messages, "ChatGPT: "+msg.Choices[0].Message.Content)
		m.Viewport.SetContent(strings.Join(m.Messages, "\n"))
		m.Viewport.GotoBottom()

	// We handle errors just like any other message
	case error:
		// TODO: properly display error
		m.Err = msg
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m Model) View() string {
	helpText := "(ctrl+c to quit)"
	secondLine := m.Textarea.View()
	if m.Waiting {
		secondLine = "Waiting for response"
	}

	return fmt.Sprintf(
		"%s\n\n%s\n\n%s",
		m.Viewport.View(),
		secondLine,
		helpText,
	) + "\n\n"
}

func New() Model {
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

	vp := viewport.New(50, 10)
	vp.SetContent(fmt.Sprintf(
		"%s\n\n%s%s\n%s",
		"Welcome to use gptui Chat",
		"You are using model: ",
		viper.GetString("model"),
		"Type a message and press Enter to send."))

	return Model{
		Textarea:    ta,
		Messages:    []string{},
		Viewport:    vp,
		SenderStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("5")),
		Err:         nil,
		Waiting:     false,
	}
}
