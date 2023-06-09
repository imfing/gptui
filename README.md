# gptui

[![Release](https://github.com/imfing/gptui/actions/workflows/release.yml/badge.svg)](https://github.com/imfing/gptui/actions/workflows/release.yml)

Terminal UI for OpenAI ChatGPT

<img src="https://user-images.githubusercontent.com/5097752/235353364-bf3b6b62-a29e-458c-b509-a48c52407fad.png" alt="gptui" width="500" />

## Features

- Elegant and easy-to-use command line interface
- Automatic syntax highlighting
- Save and restore chat history locally

## Installation

Using [Homebrew](https://brew.sh/):

```bash
$ brew install imfing/tap/gptui
```

Or go to [Release](https://github.com/imfing/gptui/releases) page and manually install it.

## Usage

[OpenAI API key](https://platform.openai.com/account/api-keys) is required. 
Pass it as a command line argument or set an environment variable:
```bash
export OPENAI_API_KEY=<your-openai-api-key>
```

To start chat:
```bash
❯ gptui chat
```

Available flags:

```text
❯ gptui chat --help
Given a chat conversation, the model will return a chat completion response.

Usage:
  gptui chat [flags]

Flags:
  -h, --help                     help for chat
      --history string           path to conversation history file to restore from
      --max-context-length int   maximum number of tokens for GPT context (default 1024)
  -m, --message string           message for the chat input
      --model string             model to use for chat completion (default "gpt-3.5-turbo")
      --stream                   if set, partial message deltas will be sent, like in ChatGPT (default true)
      --system string            system message that helps set the behavior of the assistant

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
