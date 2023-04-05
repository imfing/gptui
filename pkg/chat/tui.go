package chat

import (
	"fmt"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
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
	appStyle      = lipgloss.NewStyle().Margin(1, 2, 0, 2)
	senderStyle   = lipgloss.NewStyle().Background(lipgloss.Color("5")).Foreground(lipgloss.Color("#FAFAFA")).Padding(0, 1)
	chatStyle     = lipgloss.NewStyle().Background(lipgloss.Color("36")).Foreground(lipgloss.Color("#FAFAFA")).Padding(0, 1)
	textAreaStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	spinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).MarginTop(4)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 0)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

var textAreaHeight = 4

type keymap struct {
	Esc, Quit, Send, NewLine key.Binding
}

var keys = keymap{
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "exit fullscreen"),
	),
	Send: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "send"),
	),
	NewLine: key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "newline"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.Send, k.NewLine, k.Esc, k.Quit}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Send, k.NewLine, k.Esc, k.Quit},
	}
}

type Model struct {
	viewport viewport.Model
	messages []string
	textarea textarea.Model
	spinner  spinner.Model
	renderer *glamour.TermRenderer
	err      error
	waiting  bool
	keys     keymap
	help     help.Model
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
		switch {
		case key.Matches(msg, m.keys.Esc):
			return m, tea.ExitAltScreen
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Send):
			input, _ := m.renderer.Render(m.textarea.Value())
			m.messages = append(m.messages, senderStyle.Render("You")+"\n"+input)
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
			glamour.WithWordWrap(msg.Width-x-2),
		)
		// TODO: re-render messages based on new word wrap width

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case CompletionResponse:
		m.waiting = false
		output, _ := m.renderer.Render(msg.Choices[0].Message.Content)
		m.messages = append(m.messages, chatStyle.Render("ChatGPT")+"\n"+output)
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	// handle errors just like any other message
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

	s += m.help.View(m.keys)

	return appStyle.Render(s)
}

// newTextArea creates a text area model
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

// NewModel creates a new chat tui model
func NewModel() Model {
	ta := newTextArea()
	ta.SetWidth(50)
	ta.SetHeight(textAreaHeight)
	ta.Focus()

	chatModel := viper.GetString("model")

	vp := viewport.New(50, 10)
	vp.SetContent(fmt.Sprintf(
		"%s\n\n%s\n%s",
		"Welcome to use ChatGPT terminal UI",
		helpStyle.Render("Model: "+chatModel),
		"Type a message and press Ctrl+S to send."))

	s := spinner.New()
	s.Style = spinnerStyle

	return Model{
		textarea: ta,
		messages: []string{},
		viewport: vp,
		err:      nil,
		waiting:  false,
		spinner:  s,
		help:     help.New(),
		keys:     keys,
	}
}
