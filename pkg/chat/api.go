package chat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/imfing/gptui/pkg/rest"
)

// OpenAI API types
// See https://platform.openai.com/docs/api-reference/chat

type CompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type CompletionChoice struct {
	Index        int `json:"index,omitempty"`
	Message      Message
	FinishReason string `json:"finish_reason,omitempty"`
}

type CompletionResponse struct {
	ID      string             `json:"id,omitempty"`
	Object  string             `json:"object,omitempty"`
	Created int64              `json:"created,omitempty"`
	Choices []CompletionChoice `json:"choices,omitempty"`
	Usage   CompletionUsage    `json:"usage,omitempty"`
}

type CompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CompletionStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type CompletionStreamChoice struct {
	Index        int                   `json:"index,omitempty"`
	Delta        CompletionStreamDelta `json:"delta,omitempty"`
	FinishReason string                `json:"finish_reason,omitempty"`
}

type CompletionStreamResponse struct {
	ID      string                   `json:"id,omitempty"`
	Object  string                   `json:"object,omitempty"`
	Created int64                    `json:"created,omitempty"`
	Choices []CompletionStreamChoice `json:"choices,omitempty"`
}

// Client implements a REST client for OpenAI API
type Client struct {
	httpClient *rest.Client
	// model ID of the model to use
	model string
	// stream if set to `true`, partial message deltas will be sent
	stream bool
	// token sets the Bearer token in the header for authentication
	token string
	// events is the channel for streaming the data-only server-sent events
	events chan CompletionStreamResponse
	// history stores list of previous messages
	history []Message
}

func NewChatClient(baseURL string, token string, model string, stream bool) *Client {
	c := rest.NewClient(
		rest.WithBaseURL(baseURL),
		rest.WithTimeout(time.Minute),
	)
	client := &Client{
		httpClient: c,
		model:      model,
		stream:     stream,
		token:      token,
		events:     make(chan CompletionStreamResponse),
		history:    []Message{},
	}
	return client
}

// NewRequest creates a http request for the chat completion API
func (c *Client) NewRequest(body *CompletionRequest) (*http.Request, error) {
	header := http.Header{
		"Authorization": []string{fmt.Sprintf("Bearer %s", c.token)},
		"Content-Type":  []string{"application/json"},
	}
	if c.stream {
		header.Set("Accept", "text/event-stream")
		header.Set("Cache-Control", "no-cache")
		header.Set("Connection", "keep-alive")
		body.Stream = true
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := c.httpClient.NewRequest(
		"/chat/completions",
		rest.WithMethod(http.MethodPost),
		rest.WithHeader(header),
		rest.WithBody(bytes.NewReader(payload)),
	)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// CreateCompletion sends the CompletionRequest
// If stream is enabled, server-sent events will be sent into the events channel
// Otherwise, it returns CompletionResponse
func (c *Client) CreateCompletion(request *CompletionRequest) (*CompletionResponse, error) {
	req, err := c.NewRequest(request)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("status code: %d, body: %s", resp.StatusCode, string(body))
	}

	if !c.stream {
		body, err := io.ReadAll(resp.Body)
		var ret CompletionResponse
		if err = json.Unmarshal(body, &ret); err != nil {
			return nil, err
		}
		return &ret, nil
	}

	// process stream response
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

			if data == "[DONE]" {
				break
			} else {
				var streamResp CompletionStreamResponse
				if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
					return nil, err
				}
				c.events <- streamResp
			}
		}
	}
	return nil, nil
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

func sendChatCompletionRequest(endpoint string, token string, model string, message string) tea.Cmd {
	return func() tea.Msg {
		if len(token) == 0 {
			return fmt.Errorf("OpenAI API key is not set. Please set it with the --openai-api-key flag")
		}

		// TODO: prepend previous messages
		chatCompletionRequest := CompletionRequest{
			Model: model,
			Messages: []Message{
				{Role: "user", Content: message},
			},
		}

		payload, err := json.Marshal(chatCompletionRequest)
		if err != nil {
			return error(err)
		}

		req, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(payload))

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		client := &http.Client{
			Timeout: 30 * time.Second,
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
			return fmt.Errorf("status code: %d, body: %s", resp.StatusCode, string(body))
		}

		var chatCompletion CompletionResponse
		if err := json.Unmarshal(body, &chatCompletion); err != nil {
			return error(err)
		}

		return chatCompletion
	}
}

func sendChatCompletionStreamRequest(endpoint string, token string, model string, message string, sub chan CompletionStreamResponse) tea.Cmd {
	return func() tea.Msg {
		if len(token) == 0 {
			return fmt.Errorf("API token not set")
		}

		chatCompletionRequest := CompletionRequest{
			Model: model,
			Messages: []Message{
				//{Role: "system", Content: "You are an AI language model trained to provide information and answer questions."},
				{Role: "user", Content: message},
			},
			Stream: true,
		}

		payload, err := json.Marshal(chatCompletionRequest)
		if err != nil {
			return err
		}

		req, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(payload))

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// process SSE
		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data:") {
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

				if data == "[DONE]" {
					// end of stream
					break
				} else {
					var streamResp CompletionStreamResponse
					if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
						return err
					}
					sub <- streamResp
				}
			}
		}
		return nil
	}
}

func waitForStreamResponse(sub chan CompletionStreamResponse) tea.Cmd {
	return func() tea.Msg {
		return CompletionStreamResponse(<-sub)
	}
}
