package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dcadolph/midden/internal/util"
)

// EnvChatMaxTokens overrides the chat reply token budget.
const EnvChatMaxTokens = "MIDDEN_CHAT_MAX_TOKENS" //nolint:gosec // Environment variable name, not a credential.

// Default provider models and endpoints. They live in one block so provider
// drift is a one-line fix; each has an environment override. The reply budget
// is generous because current Claude models reason before answering and that
// reasoning is charged against the same cap as the reply, so a tight budget
// truncates the answer rather than the thinking.
const (
	defaultAnthropicModel   = "claude-opus-5"
	defaultOpenAIChatModel  = "gpt-4o-mini"
	defaultOllamaChatModel  = "llama3.2"
	defaultOpenAIEmbedModel = "text-embedding-3-small"
	defaultVoyageEmbedModel = "voyage-3"
	defaultOllamaEmbedModel = "nomic-embed-text"
	defaultOllamaHost       = "http://localhost:11434"
	defaultChatMaxTokens    = 16000
)

// retryAttempts is the total try count for retryable provider failures.
const retryAttempts = 3

// maxErrBodyRunes caps how much of a provider error body lands in returned errors.
const maxErrBodyRunes = 300

// httpDo is the shared HTTP transport used by every provider. Tests replace it.
// The generous timeout is a safety net; commands bound calls with context deadlines.
var httpDo = (&http.Client{Timeout: 5 * time.Minute}).Do

// postJSON posts a JSON payload and returns the response body. Network errors,
// 429, and 5xx responses retry with exponential backoff, honoring a
// Retry-After seconds header when present. The label prefixes every error.
func postJSON(ctx context.Context, label, url string, headers map[string]string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("%s: marshal request: %w", label, err)
	}
	var lastErr error
	var delay time.Duration
	for attempt := range retryAttempts {
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("%s: %w", label, ctx.Err())
			}
		}
		delay = (1 << attempt) * time.Second
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("%s: build request: %w", label, err)
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := httpDo(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("%s: %w", label, ctx.Err())
			}
			lastErr = fmt.Errorf("%s: do request: %w", label, err)
			continue
		}
		data, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("%s: read response: %w", label, readErr)
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return data, nil
		}
		lastErr = fmt.Errorf("%s: %s: %s", label, resp.Status, util.TruncateRunes(string(data), maxErrBodyRunes))
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			return nil, lastErr
		}
		if ra := retryAfter(resp.Header.Get("Retry-After")); ra > 0 {
			delay = ra
		}
	}
	return nil, lastErr
}

// retryAfter parses a Retry-After seconds value, returning zero when absent or invalid.
func retryAfter(v string) time.Duration {
	secs, err := strconv.Atoi(v)
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

// ollamaHost returns the Ollama base URL from OLLAMA_HOST or the local default.
func ollamaHost() string {
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		return host
	}
	return defaultOllamaHost
}

// isOllamaReachable reports whether the configured Ollama daemon answers a health check.
func isOllamaReachable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ollamaHost()+"/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := httpDo(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
