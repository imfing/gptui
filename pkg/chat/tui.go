package chat

import (
	"encoding/json"
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
	"log"
	"os"
	"path"
	"strings"
	"time"
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
	streamDeltas string
	sessionId    string
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
				m.client.history = append(m.client.history, Message{Role: "user", Content: m.textarea.Value()})
				content, _ := m.renderMessages(m.client.history)
				m.viewport.SetContent(content)

				req := newCompletionRequest(m.client, m.textarea.Value())
				commands = append(commands, createCompletionCmd(m.client, req))
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

		if m.viewport.Height <= 0 {
			m.err = fmt.Errorf("terminal size too small")
			return m, nil
		}

		m.renderer, _ = newGlamourRenderer(msg.Width - h - 2)

		// re-render the conversation
		if !m.waiting && len(m.client.history) > 0 {
			content, _ := m.renderMessages(m.client.history)
			m.viewport.SetContent(content)
			m.viewport.GotoBottom()
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		commands = append(commands, cmd)

	case CompletionResponse:
		m.waiting = false
		choice := msg.Choices[0]
		m.client.history = append(m.client.history, choice.Message)
		content, _ := m.renderMessages(m.client.history)

		m.viewport.SetContent(content)
		m.viewport.GotoBottom()

	case CompletionStreamResponse:
		choice := msg.Choices[0]
		if choice.FinishReason == "stop" {
			m.waiting = false
			// save stream response to client history
			m.client.history = append(m.client.history, Message{Role: "assistant", Content: m.streamDeltas})
			// reset stream message
			m.streamDeltas = ""

			m.saveHistory()
		} else {
			// waiting for next event message
			commands = append(commands, waitEventsCmd(m.client))
			if len(choice.Delta.Content) > 0 {
				m.streamDeltas += choice.Delta.Content
				delta, _ := m.renderer.Render(m.streamDeltas)
				output := chatStyle.Render(chatGPTName) + "\n" + delta + "\n"
				history, _ := m.renderMessages(m.client.history)
				m.viewport.SetContent(history + output)
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

// newGlamourRenderer creates new glamour Markdown renderer with given wordWrap width
func newGlamourRenderer(wordWrap int) (*glamour.TermRenderer, error) {
	glamourStyle := LightStyleConfig
	if termenv.HasDarkBackground() {
		glamourStyle = DarkStyleConfig
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(glamourStyle),
		glamour.WithWordWrap(wordWrap),
	)
	return renderer, err
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
	system := viper.GetString("system")
	history := viper.GetString("history")
	stream := viper.GetBool("stream")

	sessionId := time.Now().Format("2006-01-02_15-04-05")

	welcomeMessage := fmt.Sprintf("%s\n\n%s\n%s",
		"ChatGPT Terminal UI",
		helpStyle.Render("Model: "+chatModel+"\n"),
		"Type a message and press Enter to send.")

	// init viewport where the conversations will be displayed
	vp := viewport.New(50, 10)
	vp.SetContent(welcomeMessage)

	s := spinner.New(spinner.WithStyle(spinnerStyle))

	client := NewChatClient(baseURL, token, chatModel, system, stream)
	m := Model{
		textarea:  ta,
		viewport:  vp,
		spinner:   s,
		help:      help.New(),
		keys:      keys,
		sessionId: sessionId,
		client:    client,
	}

	// restore history if necessary
	if len(history) > 0 {
		err := m.loadHistory(history)
		if err != nil {
			log.Fatal(err)
		}
		fileName := path.Base(history)
		m.sessionId = strings.TrimSuffix(fileName, path.Ext(fileName))
	}
	return m
}

// newCompletionRequest creates new CompletionRequest
func newCompletionRequest(client *Client, message string) *CompletionRequest {
	var messages []Message
	// TODO: include chat history without overflowing the token limit
	if len(client.system) > 0 && len(client.history) == 0 {
		messages = append(messages, Message{Role: "system", Content: client.system})
	}
	messages = append(messages, Message{Role: "user", Content: message})
	return &CompletionRequest{Model: client.model, Messages: messages}
}

// createCompletionCmd returns a tea.Cmd which constructs the CompletionRequest
// and returns CompletionResponse if stream is set to false
func createCompletionCmd(client *Client, req *CompletionRequest) tea.Cmd {
	return func() tea.Msg {
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

// renderMessages renders the content of Markdown messages
func (m Model) renderMessages(messages []Message) (string, error) {
	var renderedMessages []string

	user := senderStyle.Render(userName) + "\n"
	chat := chatStyle.Render(chatGPTName) + "\n"

	for _, message := range messages {
		output, err := m.renderer.Render(message.Content)
		if err != nil {
			return "", err
		}
		var author string
		switch message.Role {
		case "user":
			author = user
		case "assistant":
			author = chat
		default:
			continue
		}
		output = author + output
		renderedMessages = append(renderedMessages, output)
	}
	return strings.Join(renderedMessages, "\n"), nil
}

// loadHistory reads conversation history from a JSON file
func (m Model) loadHistory(filePath string) error {
	// handle path starts with "~/"
	if strings.HasPrefix(filePath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		filePath = path.Join(homeDir, filePath[2:])
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err != nil {
			return err
		}
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &m.client.history)
	if err != nil {
		return err
	}
	return nil
}

// saveHistory saves chat history to JSON file
func (m Model) saveHistory() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := path.Join(homeDir, ".config", "gptui", "chat")

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return err
		}
	}
	filepath := path.Join(dir, fmt.Sprintf("%s.json", m.sessionId))
	data, err := json.Marshal(m.client.history)
	if err != nil {
		return err
	}

	err = os.WriteFile(filepath, data, 0644)
	if err != nil {
		return err
	}
	return nil
}
