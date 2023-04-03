package chat

import (
	"bytes"
	"encoding/json"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"io"
	"log"
	"net/http"
	"time"
)

type CompletionResponse struct {
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

type CompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func sendChatCompletionRequest(endpoint string, token string, model string, message string) tea.Cmd {
	return func() tea.Msg {
		if len(token) == 0 {
			return fmt.Errorf("OpenAI API key is not set. Please set it with the --openai-api-key flag")
		}

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
