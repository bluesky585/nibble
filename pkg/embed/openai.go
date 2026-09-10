package embed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	defaultOpenAIBase  = "https://api.openai.com/v1"
	defaultOpenAIModel = "text-embedding-3-small"
)

// OpenAI calls an OpenAI-compatible /v1/embeddings endpoint.
// Client is reused for every Embed call; do not construct OpenAI in a loop.
type OpenAI struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
}

// NewOpenAI builds a client. nil httpClient uses http.DefaultClient.
// Empty baseURL or model use the OpenAI defaults.
func NewOpenAI(httpClient *http.Client, baseURL, apiKey, model string) (*OpenAI, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if baseURL == "" {
		baseURL = defaultOpenAIBase
	}
	if model == "" {
		model = defaultOpenAIModel
	}
	return &OpenAI{
		client:  httpClient,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
	}, nil
}

// NewOpenAIFromEnv reads OPENAI_API_KEY, optional OPENAI_BASE_URL and
// OPENAI_EMBED_MODEL.
func NewOpenAIFromEnv() (*OpenAI, error) {
	return NewOpenAI(nil, os.Getenv("OPENAI_BASE_URL"), os.Getenv("OPENAI_API_KEY"), os.Getenv("OPENAI_EMBED_MODEL"))
}

type openaiRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openaiResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed sends the whole batch in one HTTP request.
func (o *OpenAI) Embed(texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(openaiRequest{Model: o.model, Input: texts}); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, o.embeddingsURL(), &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed openaiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode embeddings response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("embeddings API: %s", parsed.Error.Message)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings API HTTP %d: %s", resp.StatusCode, truncate(raw, 256))
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embeddings API returned %d vectors for %d inputs", len(parsed.Data), len(texts))
	}

	out := make([][]float64, len(texts))
	for _, item := range parsed.Data {
		if item.Index < 0 || item.Index >= len(out) {
			return nil, fmt.Errorf("embeddings API index %d out of range", item.Index)
		}
		if out[item.Index] != nil {
			return nil, fmt.Errorf("embeddings API duplicate index %d", item.Index)
		}
		out[item.Index] = item.Embedding
	}
	for i, v := range out {
		if v == nil {
			return nil, fmt.Errorf("embeddings API missing index %d", i)
		}
	}
	return out, nil
}

func (o *OpenAI) embeddingsURL() string {
	if strings.HasSuffix(o.baseURL, "/v1") {
		return o.baseURL + "/embeddings"
	}
	return o.baseURL + "/v1/embeddings"
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
