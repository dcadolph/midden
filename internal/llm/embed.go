// Package llm wraps the external LLM providers midden talks to for embeddings and chat.
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
	"time"
)

// Embedder produces a vector embedding for each input string.
// Implementations should be safe to call from multiple goroutines.
type Embedder interface {
	// Embed returns one vector per input string.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dim returns the embedding dimension, or zero when unknown until first call.
	Dim() int
	// Name identifies the provider for index headers and logs.
	Name() string
}

// EmbedderFromEnv picks an embedder using documented environment precedence.
// VOYAGE_API_KEY → Voyage AI, OPENAI_API_KEY → OpenAI, OLLAMA_HOST → local Ollama.
// An explicit MIDDEN_EMBED_PROVIDER short-circuits autodetection.
func EmbedderFromEnv() (Embedder, error) {
	provider := os.Getenv("MIDDEN_EMBED_PROVIDER")
	switch provider {
	case "voyage":
		return newVoyage()
	case "openai":
		return newOpenAIEmbed()
	case "ollama":
		return newOllamaEmbed()
	case "":
	default:
		return nil, fmt.Errorf("unknown MIDDEN_EMBED_PROVIDER %q", provider)
	}
	if os.Getenv("VOYAGE_API_KEY") != "" {
		return newVoyage()
	}
	if os.Getenv("OPENAI_API_KEY") != "" {
		return newOpenAIEmbed()
	}
	if os.Getenv("OLLAMA_HOST") != "" || isOllamaReachable() {
		return newOllamaEmbed()
	}
	return nil, errors.New("no embedding provider configured: set VOYAGE_API_KEY, OPENAI_API_KEY, or run ollama locally")
}

// httpDo is the shared HTTP client used by every provider.
var httpDo = (&http.Client{Timeout: 60 * time.Second}).Do

// openAIEmbed implements Embedder against the OpenAI embeddings endpoint.
type openAIEmbed struct {
	apiKey string
	model  string
	dim    int
}

func newOpenAIEmbed() (*openAIEmbed, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, errors.New("OPENAI_API_KEY is not set")
	}
	model := os.Getenv("OPENAI_EMBED_MODEL")
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &openAIEmbed{apiKey: key, model: model}, nil
}

func (e *openAIEmbed) Name() string { return "openai:" + e.model }
func (e *openAIEmbed) Dim() int     { return e.dim }

func (e *openAIEmbed) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{"input": texts, "model": e.model})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	resp, err := httpDo(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai embeddings: %s: %s", resp.Status, string(data))
	}
	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	out := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		out[i] = d.Embedding
		if i == 0 {
			e.dim = len(d.Embedding)
		}
	}
	return out, nil
}

// voyageEmbed implements Embedder against the Voyage AI embeddings endpoint.
type voyageEmbed struct {
	apiKey string
	model  string
	dim    int
}

func newVoyage() (*voyageEmbed, error) {
	key := os.Getenv("VOYAGE_API_KEY")
	if key == "" {
		return nil, errors.New("VOYAGE_API_KEY is not set")
	}
	model := os.Getenv("VOYAGE_EMBED_MODEL")
	if model == "" {
		model = "voyage-3"
	}
	return &voyageEmbed{apiKey: key, model: model}, nil
}

func (e *voyageEmbed) Name() string { return "voyage:" + e.model }
func (e *voyageEmbed) Dim() int     { return e.dim }

func (e *voyageEmbed) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{"input": texts, "model": e.model, "input_type": "document"})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.voyageai.com/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	resp, err := httpDo(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("voyage embeddings: %s: %s", resp.Status, string(data))
	}
	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	out := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		out[i] = d.Embedding
		if i == 0 {
			e.dim = len(d.Embedding)
		}
	}
	return out, nil
}

// ollamaEmbed implements Embedder against a local Ollama daemon.
type ollamaEmbed struct {
	host  string
	model string
	dim   int
}

func newOllamaEmbed() (*ollamaEmbed, error) {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	model := os.Getenv("OLLAMA_EMBED_MODEL")
	if model == "" {
		model = "nomic-embed-text"
	}
	return &ollamaEmbed{host: host, model: model}, nil
}

func (e *ollamaEmbed) Name() string { return "ollama:" + e.model }
func (e *ollamaEmbed) Dim() int     { return e.dim }

func (e *ollamaEmbed) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		body, err := json.Marshal(map[string]any{"model": e.model, "prompt": t})
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.host+"/api/embeddings", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpDo(req)
		if err != nil {
			return nil, fmt.Errorf("do request: %w", err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("ollama embeddings: %s: %s", resp.Status, string(data))
		}
		var parsed struct {
			Embedding []float32 `json:"embedding"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		out[i] = parsed.Embedding
		if i == 0 {
			e.dim = len(parsed.Embedding)
		}
	}
	return out, nil
}

// isOllamaReachable reports whether the local Ollama daemon answers a health check.
func isOllamaReachable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:11434/api/tags", nil)
	if err != nil {
		return false
	}
	resp, err := httpDo(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
