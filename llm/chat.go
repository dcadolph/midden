package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Message is one chat turn supplied to Chatter.Reply.
type Message struct {
	// Role is "user" or "assistant".
	Role string
	// Content is the message text.
	Content string
}

// Chatter produces an assistant reply given a system prompt and conversation history.
type Chatter interface {
	// Reply returns the assistant text for the given history.
	Reply(ctx context.Context, system string, messages []Message) (string, error)
	// Name identifies the provider for logging.
	Name() string
}

// ChatterFromEnv selects a chat provider using documented environment precedence.
// ANTHROPIC_API_KEY → Claude, OPENAI_API_KEY → OpenAI, OLLAMA_HOST → local Ollama.
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
	if isOllamaReachable() {
		return newOllamaChat()
	}
	return nil, errors.New("no chat provider configured: set ANTHROPIC_API_KEY, OPENAI_API_KEY, or run ollama locally")
}

// anthropicChat implements Chatter against the Anthropic messages API.
type anthropicChat struct {
	apiKey string
	model  string
}

func newAnthropicChat() (*anthropicChat, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, errors.New("ANTHROPIC_API_KEY is not set")
	}
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	return &anthropicChat{apiKey: key, model: model}, nil
}

func (c *anthropicChat) Name() string { return "anthropic:" + c.model }

func (c *anthropicChat) Reply(ctx context.Context, system string, history []Message) (string, error) {
	msgs := make([]map[string]string, len(history))
	for i, m := range history {
		msgs[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	body, err := json.Marshal(map[string]any{
		"model":      c.model,
		"max_tokens": 1024,
		"system":     system,
		"messages":   msgs,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := httpDo(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic messages: %s: %s", resp.Status, string(data))
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	var buf bytes.Buffer
	for _, p := range parsed.Content {
		if p.Type == "text" {
			buf.WriteString(p.Text)
		}
	}
	return buf.String(), nil
}

// openAIChat implements Chatter against the OpenAI Chat Completions endpoint.
type openAIChat struct {
	apiKey string
	model  string
}

func newOpenAIChat() (*openAIChat, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, errors.New("OPENAI_API_KEY is not set")
	}
	model := os.Getenv("OPENAI_CHAT_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &openAIChat{apiKey: key, model: model}, nil
}

func (c *openAIChat) Name() string { return "openai:" + c.model }

func (c *openAIChat) Reply(ctx context.Context, system string, history []Message) (string, error) {
	msgs := make([]map[string]string, 0, len(history)+1)
	if system != "" {
		msgs = append(msgs, map[string]string{"role": "system", "content": system})
	}
	for _, m := range history {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body, err := json.Marshal(map[string]any{"model": c.model, "messages": msgs})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := httpDo(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai chat: %s: %s", resp.Status, string(data))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
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
	return parsed.Choices[0].Message.Content, nil
}

// ollamaChat implements Chatter against a local Ollama daemon.
type ollamaChat struct {
	host  string
	model string
}

func newOllamaChat() (*ollamaChat, error) {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	model := os.Getenv("OLLAMA_CHAT_MODEL")
	if model == "" {
		model = "llama3.2"
	}
	return &ollamaChat{host: host, model: model}, nil
}

func (c *ollamaChat) Name() string { return "ollama:" + c.model }

func (c *ollamaChat) Reply(ctx context.Context, system string, history []Message) (string, error) {
	msgs := make([]map[string]string, 0, len(history)+1)
	if system != "" {
		msgs = append(msgs, map[string]string{"role": "system", "content": system})
	}
	for _, m := range history {
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}
	body, err := json.Marshal(map[string]any{"model": c.model, "messages": msgs, "stream": false})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpDo(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama chat: %s: %s", resp.Status, string(data))
	}
	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return parsed.Message.Content, nil
}
