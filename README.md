# gptui

Terminal UI for OpenAI ChatGPT

<img src="https://user-images.githubusercontent.com/5097752/235353364-bf3b6b62-a29e-458c-b509-a48c52407fad.png" alt="gptui" width="500" />

## Usage

```text
❯ gptui chat --help
Given a chat conversation, the model will return a chat completion response.

Usage:
  gptui chat [flags]

Flags:
  -h, --help                     help for chat
      --history string           Path to conversation history file to restore from.
      --max-context-length int   Maximum number of tokens for GPT context. (default 1024)
  -m, --message string           Message for the chat input.
      --model string             Model to use. (default "gpt-3.5-turbo")
      --stream                   If set, partial message deltas will be sent, like in ChatGPT. (default true)
      --system string            System message that helps set the behavior of the assistant.

Global Flags:
      --openai-api-base string   OpenAI API endpoint (default "https://api.openai.com/v1")
      --openai-api-key string    OpenAI API key
```

## Roadmap

- [ ] [Completion](https://platform.openai.com/docs/api-reference/completions)
- [ ] [Edits](https://platform.openai.com/docs/api-reference/edits) 
- [ ] [Azure OpenAI](https://learn.microsoft.com/en-us/azure/cognitive-services/openai/)

## License

See [`LICENSE`](./LICENSE)
