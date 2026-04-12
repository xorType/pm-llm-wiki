package ollama

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "http://localhost:11434"
	DefaultModel   = "gemma4:31b-cloud"
	defaultTimeout = 600 * time.Second
)

// Client is a minimal HTTP client for the Ollama REST API.
type Client struct {
	BaseURL string
	Model   string
	http    *http.Client
}

type generateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Options map[string]any `json:"options,omitempty"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}

// New returns a configured Ollama client. Empty strings fall back to defaults.
func New(baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if model == "" {
		model = DefaultModel
	}
	return &Client{
		BaseURL: baseURL,
		Model:   model,
		http:    &http.Client{Timeout: defaultTimeout},
	}
}

// Ping checks that Ollama is reachable and that the configured model exists.
func (c *Client) Ping() error {
	resp, err := c.http.Get(c.BaseURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("ollama not reachable at %s: %w\n  → run: ollama serve", c.BaseURL, err)
	}
	defer resp.Body.Close()

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return fmt.Errorf("parsing Ollama /api/tags: %w", err)
	}

	// Match on base model name (before the colon) to be flexible with tags.
	wantBase := strings.SplitN(c.Model, ":", 2)[0]
	for _, m := range tags.Models {
		if strings.HasPrefix(m.Name, wantBase) {
			return nil
		}
	}
	return fmt.Errorf("model %q not found in Ollama.\n  → run: ollama pull %s", c.Model, c.Model)
}

// Generate sends prompt to Ollama and streams the full response back as a string.
func (c *Client) Generate(prompt string) (string, error) {
	body, err := json.Marshal(generateRequest{
		Model:  c.Model,
		Prompt: prompt,
		Stream: true,
	})
	if err != nil {
		return "", err
	}

	resp, err := c.http.Post(c.BaseURL+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ollama generate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned HTTP %d", resp.StatusCode)
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	// Ollama streams one JSON object per line.
	for scanner.Scan() {
		var chunk generateResponse
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			return "", fmt.Errorf("ollama error: %s", chunk.Error)
		}
		sb.WriteString(chunk.Response)
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading ollama stream: %w", err)
	}
	return sb.String(), nil
}
