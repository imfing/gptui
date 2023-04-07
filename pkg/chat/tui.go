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
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

var (
	textAreaHeight = 4
	chatGPTName    = "ChatGPT"
	userName       = "You"
)

type keymap struct {
	Help, Esc, Quit, Send, Multiline key.Binding
}

var keys = keymap{
	Help: key.NewBinding(
		key.WithKeys("ctrl+h"),
		key.WithHelp("ctrl+h", "help"),
	),
	Esc: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "exit fullscreen"),
	),
	Send: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "send"),
	),
	Multiline: key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp("ctrl+l", "toggle multi-line"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	),
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Send, k.Quit}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Send, k.Quit},
		{k.Multiline, k.Esc},
	}
}

// Model stores the state
type Model struct {
	client       *Client
	viewport     viewport.Model
	textarea     textarea.Model
	spinner      spinner.Model
	renderer     *glamour.TermRenderer
	help         help.Model
	keys         keymap
	messages     []string
	streamDeltas string
	multiline    bool
	waiting      bool
	width        int
	height       int
	err          error
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		tea.EnterAltScreen,
		m.spinner.Tick,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd    tea.Cmd
		vpCmd    tea.Cmd
		commands []tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)
	commands = []tea.Cmd{tiCmd, vpCmd}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Help):
			// toggle help
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Esc):
			return m, tea.ExitAltScreen
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Multiline):
			// toggle multiline
			m.multiline = !m.multiline
			m.textarea.ShowLineNumbers = m.multiline
			// refresh textarea width
			m.textarea.SetWidth(m.width - appStyle.GetHorizontalFrameSize())
		case key.Matches(msg, m.keys.Send):
			if !m.multiline && !m.waiting {
				input, _ := m.renderer.Render(m.textarea.Value())
				m.client.history = append(m.client.history, Message{Role: "user", Content: input})
				m.messages = append(m.messages, senderStyle.Render(userName)+"\n"+input)
				m.viewport.SetContent(strings.Join(m.messages, "\n"))

				commands = append(commands, createCompletionCmd(m.client, m.textarea.Value()))
				if m.client.stream {
					commands = append(commands, waitEventsCmd(m.client))
				}

				m.textarea.Reset()
				m.viewport.GotoBottom()
				// set waiting to true so spinner will be visible
				m.waiting = true
			}
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		h := appStyle.GetHorizontalFrameSize()
		m.viewport.Width = msg.Width - h
		m.viewport.Height = msg.Height - (8 + textAreaHeight)
		m.textarea.SetWidth(msg.Width - h)

		glamourStyle := LightStyleConfig
		if termenv.HasDarkBackground() {
			glamourStyle = DarkStyleConfig
		}
		m.renderer, _ = glamour.NewTermRenderer(
			glamour.WithStyles(glamourStyle),
			glamour.WithWordWrap(msg.Width-h-2),
		)
		// TODO: re-render messages based on new word wrap width

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		commands = append(commands, cmd)

	case CompletionResponse:
		m.waiting = false
		choice := msg.Choices[0]
		m.client.history = append(m.client.history, Message{Role: choice.Message.Role, Content: choice.Message.Content})
		output, _ := m.renderer.Render(choice.Message.Content)
		m.messages = append(m.messages, chatStyle.Render(chatGPTName)+"\n"+output)
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()

	case CompletionStreamResponse:
		choice := msg.Choices[0]
		if choice.FinishReason == "stop" {
			m.waiting = false
			output, _ := m.renderer.Render(m.streamDeltas)
			m.messages = append(m.messages, chatStyle.Render(chatGPTName)+"\n"+output)
			// save stream response to client history
			m.client.history = append(m.client.history, Message{Role: "assistant", Content: m.streamDeltas})
			// reset stream message
			m.streamDeltas = ""
		} else {
			// waiting for next event message
			commands = append(commands, waitEventsCmd(m.client))
			if len(choice.Delta.Content) > 0 {
				m.streamDeltas += choice.Delta.Content
				output, _ := m.renderer.Render(m.streamDeltas)
				content := chatStyle.Render(chatGPTName) + "\n" + output + "\n"
				m.viewport.SetContent(strings.Join(m.messages, "\n") + content)
				m.viewport.GotoBottom()
			}
		}

	// handle errors just like any other message
	case error:
		m.err = msg
		return m, nil
	}

	return m, tea.Batch(commands...)
}

// View renders the UI
func (m Model) View() string {
	var s string
	s += m.viewport.View() + "\n\n"

	if m.err == nil {
		if !m.waiting {
			// textarea
			s += m.textarea.View() + "\n"
		} else {
			// spinner
			s += m.spinner.View() + " sending...\n\n"
		}
		// help view
		s += m.help.View(m.keys)
	} else {
		// display error
		s += errorStyle.Render(fmt.Sprintf("error: %v\n\n", m.err))
	}

	return appStyle.Render(s)
}

// newTextArea creates a text area model
func newTextArea() textarea.Model {
	t := textarea.New()
	t.Prompt = ""
	t.Placeholder = "Send a message..."
	t.CharLimit = -1
	t.FocusedStyle.CursorLine = lipgloss.NewStyle()
	t.FocusedStyle.EndOfBuffer = helpStyle
	t.FocusedStyle.Base = textAreaStyle
	t.ShowLineNumbers = false
	t.KeyMap.DeleteCharacterBackward = key.NewBinding(key.WithKeys("backspace"))
	t.Blur()
	return t
}

// NewModel creates a new chat tui model
func NewModel() Model {
	ta := newTextArea()
	ta.SetWidth(50)
	ta.SetHeight(textAreaHeight)
	ta.Focus()

	// read message from flag or pipe
	if msg := viper.GetString("message"); len(msg) > 0 {
		ta.SetValue(msg)
	}

	chatModel := viper.GetString("model")
	baseURL := viper.GetString("base-url")
	token := viper.GetString("openai-api-key")
	stream := viper.GetBool("stream")

	welcomeMessage := fmt.Sprintf("%s\n\n%s\n%s",
		"ChatGPT Terminal UI",
		helpStyle.Render("Model: "+chatModel+"\n"),
		"Type a message and press Enter to send.")

	// init viewport where the conversations will be displayed
	vp := viewport.New(50, 10)
	vp.SetContent(welcomeMessage)

	s := spinner.New(spinner.WithStyle(spinnerStyle))

	return Model{
		textarea: ta,
		viewport: vp,
		spinner:  s,
		help:     help.New(),
		keys:     keys,
		messages: []string{},
		client:   NewChatClient(baseURL, token, chatModel, stream),
	}
}

// createCompletionCmd returns a tea.Cmd which constructs the CompletionRequest
// and returns CompletionResponse if stream is set to false
func createCompletionCmd(client *Client, message string) tea.Cmd {
	return func() tea.Msg {
		req := &CompletionRequest{
			Model: client.model,
			// TODO: include chat history without overflowing the token limit
			Messages: []Message{
				{Role: "user", Content: message},
			},
		}
		// Blocking call to send completion request
		resp, err := client.CreateCompletion(req)
		if err != nil {
			return err
		}

		// Return CompletionResponse if stream set to false
		if !client.stream && resp != nil {
			return resp
		}
		return nil
	}
}

// waitEventsCmd listen to the events channel
// Returns the value when received from the channel
func waitEventsCmd(client *Client) tea.Cmd {
	return func() tea.Msg {
		return <-client.events
	}
}
