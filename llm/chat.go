package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Message is one chat turn supplied to Chatter.Reply.
type Message struct {
	// Role is "user" or "assistant".
	Role string
	// Content is the message text.
	Content string
}

// truncationNotice is appended to replies the provider cut off at its token limit.
const truncationNotice = "\n[Reply truncated at the provider token limit.]"

// Chatter produces an assistant reply given a system prompt and conversation history.
type Chatter interface {
	// Reply returns the assistant text for the given history.
	Reply(ctx context.Context, system string, messages []Message) (string, error)
	// Name identifies the provider for logging.
	Name() string
}

// ChatterFromEnv selects a chat provider using documented environment precedence.
// ANTHROPIC_API_KEY → Claude, OPENAI_API_KEY → OpenAI, then a configured or
// reachable Ollama daemon. MIDDEN_CHAT_PROVIDER short-circuits autodetection.
func ChatterFromEnv() (Chatter, error) {
	provider := os.Getenv("MIDDEN_CHAT_PROVIDER")
	switch provider {
	case "anthropic":
		return newAnthropicChat()
	case "openai":
		return newOpenAIChat()
	case "ollama":
		return newOllamaChat()
	case "":
	default:
		return nil, fmt.Errorf("unknown MIDDEN_CHAT_PROVIDER %q", provider)
	}
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return newAnthropicChat()
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return newOpenAIChat()
	}
	if os.Getenv("OLLAMA_HOST") != "" || isOllamaReachable() {
		return newOllamaChat()
	}
	return nil, errors.New("no chat provider configured: set ANTHROPIC_API_KEY, OPENAI_API_KEY, or run ollama locally")
}

// chatMaxTokens returns the reply token budget from MIDDEN_CHAT_MAX_TOKENS or the default.
func chatMaxTokens() int {
	if v := os.Getenv(EnvChatMaxTokens); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultChatMaxTokens
}

// roleMaps renders an optional system turn plus history as provider wire messages.
func roleMaps(system string, history []Message) []map[string]string {
	msgs := make([]map[string]string, 0, len(history)+1)
	if system != "" {
		msgs = append(msgs, map[string]string{"role": "system", "content": system})
	}
	for _, m := range history {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	return msgs
}

// anthropicChat implements Chatter against the Anthropic messages API.
type anthropicChat struct {
	// apiKey authenticates requests; never logged or serialized.
	apiKey string
	// model is the model ID sent with every request.
	model string
	// maxTokens caps the reply length.
	maxTokens int
}

func newAnthropicChat() (*anthropicChat, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, errors.New("ANTHROPIC_API_KEY is not set")
	}
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = defaultAnthropicModel
	}
	return &anthropicChat{apiKey: key, model: model, maxTokens: chatMaxTokens()}, nil
}

func (c *anthropicChat) Name() string { return "anthropic:" + c.model }

func (c *anthropicChat) Reply(ctx context.Context, system string, history []Message) (string, error) {
	data, err := postJSON(ctx, "anthropic messages", "https://api.anthropic.com/v1/messages",
		map[string]string{"x-api-key": c.apiKey, "anthropic-version": "2023-06-01"},
		map[string]any{
			"model":      c.model,
			"max_tokens": c.maxTokens,
			"system":     system,
			"messages":   roleMaps("", history),
		})
	if err != nil {
		return "", err
	}
	var parsed struct {
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	var b strings.Builder
	for _, p := range parsed.Content {
		if p.Type == "text" {
			b.WriteString(p.Text)
		}
	}
	reply := b.String()
	if parsed.StopReason == "max_tokens" {
		reply += truncationNotice
	}
	return reply, nil
}

// openAIChat implements Chatter against the OpenAI Chat Completions endpoint.
type openAIChat struct {
	// apiKey authenticates requests; never logged or serialized.
	apiKey string
	// model is the model ID sent with every request.
	model string
}

func newOpenAIChat() (*openAIChat, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, errors.New("OPENAI_API_KEY is not set")
	}
	model := os.Getenv("OPENAI_CHAT_MODEL")
	if model == "" {
		model = defaultOpenAIChatModel
	}
	return &openAIChat{apiKey: key, model: model}, nil
}

func (c *openAIChat) Name() string { return "openai:" + c.model }

func (c *openAIChat) Reply(ctx context.Context, system string, history []Message) (string, error) {
	data, err := postJSON(ctx, "openai chat", "https://api.openai.com/v1/chat/completions",
		map[string]string{"Authorization": "Bearer " + c.apiKey},
		map[string]any{"model": c.model, "messages": roleMaps(system, history)})
	if err != nil {
		return "", err
	}
	var parsed struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("openai chat returned no choices")
	}
	reply := parsed.Choices[0].Message.Content
	if parsed.Choices[0].FinishReason == "length" {
		reply += truncationNotice
	}
	return reply, nil
}

// ollamaChat implements Chatter against a local Ollama daemon.
type ollamaChat struct {
	// host is the Ollama base URL.
	host string
	// model is the model name sent with every request.
	model string
}

func newOllamaChat() (*ollamaChat, error) {
	model := os.Getenv("OLLAMA_CHAT_MODEL")
	if model == "" {
		model = defaultOllamaChatModel
	}
	return &ollamaChat{host: ollamaHost(), model: model}, nil
}

func (c *ollamaChat) Name() string { return "ollama:" + c.model }

func (c *ollamaChat) Reply(ctx context.Context, system string, history []Message) (string, error) {
	data, err := postJSON(ctx, "ollama chat", c.host+"/api/chat", nil,
		map[string]any{"model": c.model, "messages": roleMaps(system, history), "stream": false})
	if err != nil {
		return "", err
	}
	var parsed struct {
		DoneReason string `json:"done_reason"`
		Message    struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	reply := parsed.Message.Content
	if parsed.DoneReason == "length" {
		reply += truncationNotice
	}
	return reply, nil
}
