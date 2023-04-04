package chat

import (
	"fmt"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/spf13/viper"
	"strings"
)

var (
	senderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).MarginTop(4)
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	appStyle     = lipgloss.NewStyle().Margin(1, 2, 0, 2)
	chatStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("36"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	textAreaStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1)
)

var textAreaHeight = 4

type Model struct {
	viewport viewport.Model
	messages []string
	textarea textarea.Model
	spinner  spinner.Model
	renderer *glamour.TermRenderer
	err      error
	waiting  bool
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, tea.EnterAltScreen, m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	endpoint := viper.GetString("endpoint")
	token := viper.GetString("openai-api-key")
	chatModel := viper.GetString("model")

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			return m, tea.ExitAltScreen
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEnter:
			input, _ := m.renderer.Render(m.textarea.Value())
			m.messages = append(m.messages, senderStyle.Render("You: ")+input)
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.textarea.Reset()
			m.viewport.GotoBottom()
			m.waiting = true
			return m, sendChatCompletionRequest(endpoint, token, chatModel, m.textarea.Value())
		}

	case tea.WindowSizeMsg:
		x, _ := appStyle.GetFrameSize()
		m.viewport.Width = msg.Width - x
		m.viewport.Height = msg.Height - (8 + textAreaHeight)
		m.textarea.SetWidth(msg.Width - x)

		glamourStyle := LightStyleConfig
		if termenv.HasDarkBackground() {
			glamourStyle = DarkStyleConfig
		}
		m.renderer, _ = glamour.NewTermRenderer(
			glamour.WithStyles(glamourStyle),
			glamour.WithWordWrap(msg.Width-x),
		)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case CompletionResponse:
		m.waiting = false
		output, _ := m.renderer.Render(msg.Choices[0].Message.Content)
		m.messages = append(m.messages, chatStyle.Render("ChatGPT: ")+output+"\n")
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	// We handle errors just like any other message
	case error:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m Model) View() string {
	var s string

	s += m.viewport.View() + "\n\n"

	if m.err != nil {
		return s + errorStyle.Render(fmt.Sprintf("error: %v", m.err))
	}

	if !m.waiting {
		s += m.textarea.View() + "\n"
	} else {
		s += m.spinner.View() + " sending...\n\n"
	}

	s += helpStyle.Render("(ctrl+c to quit)")

	return appStyle.Render(s)
}

func newTextArea() textarea.Model {
	t := textarea.New()
	t.Prompt = ""
	t.Placeholder = "Send a message..."
	t.CharLimit = 0
	t.FocusedStyle.CursorLine = lipgloss.NewStyle()
	t.FocusedStyle.Base = textAreaStyle
	t.ShowLineNumbers = false
	t.Blur()
	return t
}

func NewModel() Model {
	ta := newTextArea()
	ta.SetWidth(50)
	ta.SetHeight(textAreaHeight)
	ta.Focus()

	chatModel := viper.GetString("model")

	vp := viewport.New(50, 10)
	vp.SetContent(fmt.Sprintf(
		"%s\n\n%s\n%s",
		"Welcome to use gptui Chat",
		helpStyle.Render("Model: "+chatModel),
		"Type a message and press Enter to send."))

	s := spinner.New()
	s.Style = spinnerStyle

	return Model{
		textarea: ta,
		messages: []string{},
		viewport: vp,
		err:      nil,
		waiting:  false,
		spinner:  s,
	}
}
