package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// stubResponse describes one canned HTTP response for the stubbed transport.
type stubResponse struct {
	// Status is the HTTP status code to return.
	Status int
	// Body is the response body.
	Body string
	// Header holds optional response headers.
	Header http.Header
}

// stubHTTP replaces httpDo with a scripted sequence of responses and restores
// it when the test finishes. It returns a counter of requests served.
// Tests using it must not run in parallel because httpDo is package state.
func stubHTTP(t *testing.T, responses ...stubResponse) *int {
	t.Helper()
	prev := httpDo
	calls := 0
	httpDo = func(req *http.Request) (*http.Response, error) {
		if calls >= len(responses) {
			t.Fatalf("unexpected extra request %d to %s", calls+1, req.URL)
		}
		r := responses[calls]
		calls++
		h := r.Header
		if h == nil {
			h = http.Header{}
		}
		return &http.Response{
			StatusCode: r.Status,
			Status:     fmt.Sprintf("%d stub", r.Status),
			Header:     h,
			Body:       io.NopCloser(strings.NewReader(r.Body)),
		}, nil
	}
	t.Cleanup(func() { httpDo = prev })
	return &calls
}

func TestPostJSONRetriesOn429ThenSucceeds(t *testing.T) {
	calls := stubHTTP(t,
		stubResponse{Status: 429, Body: `{"error":"slow down"}`, Header: http.Header{"Retry-After": []string{"0"}}},
		stubResponse{Status: 200, Body: `{"ok":true}`},
	)
	got, err := postJSON(context.Background(), "test", "http://stub", nil, map[string]int{"a": 1})
	if err != nil {
		t.Fatalf("postJSON: %v", err)
	}
	if *calls != 2 {
		t.Errorf("want 2 requests, got %d", *calls)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("unexpected body %q", got)
	}
}

func TestPostJSONDoesNotRetryClientErrors(t *testing.T) {
	calls := stubHTTP(t, stubResponse{Status: 400, Body: `{"error":"bad request"}`})
	_, err := postJSON(context.Background(), "test", "http://stub", nil, nil)
	if err == nil {
		t.Fatal("want error for 400, got nil")
	}
	if *calls != 1 {
		t.Errorf("400 must not retry: got %d requests", *calls)
	}
}

func TestPostJSONTruncatesErrorBody(t *testing.T) {
	stubHTTP(t, stubResponse{Status: 400, Body: strings.Repeat("x", 2000)})
	_, err := postJSON(context.Background(), "test", "http://stub", nil, nil)
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if len(err.Error()) > 400 {
		t.Errorf("error body not truncated: %d chars", len(err.Error()))
	}
}

func TestAnthropicReplyConcatenatesTextAndFlagsTruncation(t *testing.T) {
	stubHTTP(t, stubResponse{Status: 200, Body: `{
		"stop_reason": "max_tokens",
		"content": [
			{"type":"text","text":"part one "},
			{"type":"tool_use","text":"ignored"},
			{"type":"text","text":"part two"}
		]}`})
	c := &anthropicChat{apiKey: "k", model: "m", maxTokens: 16}
	got, err := c.Reply(context.Background(), "sys", []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	want := "part one part two" + truncationNotice
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestOpenAIReplyErrorsOnNoChoices(t *testing.T) {
	stubHTTP(t, stubResponse{Status: 200, Body: `{"choices":[]}`})
	c := &openAIChat{apiKey: "k", model: "m"}
	if _, err := c.Reply(context.Background(), "", []Message{{Role: "user", Content: "hi"}}); err == nil {
		t.Fatal("want error for empty choices, got nil")
	}
}

func TestOllamaEmbedBatchesAndRecordsDim(t *testing.T) {
	stubHTTP(t, stubResponse{Status: 200, Body: `{"embeddings":[[1,2,3],[4,5,6]]}`})
	e := &ollamaEmbed{host: "http://stub", model: "m"}
	got, err := e.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(got) != 2 || e.Dim() != 3 {
		t.Errorf("want 2 vectors dim 3, got %d vectors dim %d", len(got), e.Dim())
	}
}

func TestOllamaEmbedCountMismatchErrors(t *testing.T) {
	stubHTTP(t, stubResponse{Status: 200, Body: `{"embeddings":[[1,2,3]]}`})
	e := &ollamaEmbed{host: "http://stub", model: "m"}
	if _, err := e.Embed(context.Background(), []string{"a", "b"}); err == nil {
		t.Fatal("want count mismatch error, got nil")
	}
}

func TestEmbedEmptyInputSkipsNetwork(t *testing.T) {
	calls := stubHTTP(t)
	e := &openAIEmbed{apiKey: "k", model: "m"}
	got, err := e.Embed(context.Background(), nil)
	if err != nil || got != nil {
		t.Fatalf("want nil,nil for empty input, got %v %v", got, err)
	}
	if *calls != 0 {
		t.Errorf("empty input must not hit the network: %d requests", *calls)
	}
}

func TestChatterFromEnvPrecedence(t *testing.T) {
	for _, env := range []string{"MIDDEN_CHAT_PROVIDER", "ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OLLAMA_HOST", "ANTHROPIC_MODEL", "OPENAI_CHAT_MODEL"} {
		t.Setenv(env, "")
	}
	t.Setenv("ANTHROPIC_API_KEY", "a")
	t.Setenv("OPENAI_API_KEY", "b")
	c, err := ChatterFromEnv()
	if err != nil {
		t.Fatalf("ChatterFromEnv: %v", err)
	}
	if !strings.HasPrefix(c.Name(), "anthropic:") {
		t.Errorf("anthropic key must win, got %s", c.Name())
	}
	t.Setenv("MIDDEN_CHAT_PROVIDER", "openai")
	c, err = ChatterFromEnv()
	if err != nil {
		t.Fatalf("ChatterFromEnv: %v", err)
	}
	if !strings.HasPrefix(c.Name(), "openai:") {
		t.Errorf("explicit provider must win, got %s", c.Name())
	}
	t.Setenv("MIDDEN_CHAT_PROVIDER", "bogus")
	if _, err := ChatterFromEnv(); err == nil {
		t.Fatal("want error for unknown provider, got nil")
	}
}

func TestChatMaxTokensOverride(t *testing.T) {
	t.Setenv(EnvChatMaxTokens, "9000")
	if got := chatMaxTokens(); got != 9000 {
		t.Errorf("want 9000, got %d", got)
	}
	t.Setenv(EnvChatMaxTokens, "not-a-number")
	if got := chatMaxTokens(); got != defaultChatMaxTokens {
		t.Errorf("invalid value must fall back to default, got %d", got)
	}
}

func TestAnthropicChatSurfacesRefusal(t *testing.T) {
	stubHTTP(t, stubResponse{
		Status: 200,
		Body:   `{"stop_reason":"refusal","stop_details":{"type":"refusal","category":"cyber"},"content":[]}`,
	})
	c := &anthropicChat{apiKey: "test", model: "claude-opus-5", maxTokens: 1024}
	// A refusal is an HTTP 200 with no content blocks, so reading the blocks
	// without checking would report an empty answer as a real one.
	reply, err := c.Reply(context.Background(), "system", []Message{{Role: "user", Content: "hi"}})
	if err == nil {
		t.Fatalf("want an error for a refused request, got reply %q", reply)
	}
	if !strings.Contains(err.Error(), "cyber") {
		t.Errorf("want the refusal category surfaced, got %v", err)
	}
}

func TestAnthropicChatReturnsText(t *testing.T) {
	stubHTTP(t, stubResponse{
		Status: 200,
		Body:   `{"stop_reason":"end_turn","content":[{"type":"thinking","text":""},{"type":"text","text":"answer"}]}`,
	})
	c := &anthropicChat{apiKey: "test", model: "claude-opus-5", maxTokens: 1024}
	reply, err := c.Reply(context.Background(), "system", []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if diff := cmp.Diff("answer", reply); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}
