package chat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
	Model            string         `json:"model"`
	Messages         []Message      `json:"messages"`
	Temperature      float32        `json:"temperature,omitempty"`
	TopP             float32        `json:"top_p,omitempty"`
	N                int            `json:"n,omitempty"`
	Stream           bool           `json:"stream,omitempty"`
	Stop             []string       `json:"stop,omitempty"`
	MaxTokens        int            `json:"max_tokens,omitempty"`
	PresencePenalty  float32        `json:"presence_penalty,omitempty"`
	FrequencyPenalty float32        `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]int `json:"logit_bias,omitempty"`
	User             string         `json:"user,omitempty"`
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
	// system optional message that helps set the behavior of the assistant
	system string
	// stream if set to `true`, partial message deltas will be sent
	stream bool
	// token sets the Bearer token in the header for authentication
	token string
	// events is the channel for streaming the data-only server-sent events
	events chan CompletionStreamResponse
	// history stores list of previous messages
	history []Message
}

func NewChatClient(baseURL string, token string, model string, system string, stream bool) *Client {
	c := rest.NewClient(
		rest.WithBaseURL(baseURL),
		rest.WithTimeout(time.Minute),
	)
	client := &Client{
		httpClient: c,
		model:      model,
		system:     system,
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
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}

	return nil, nil
}
