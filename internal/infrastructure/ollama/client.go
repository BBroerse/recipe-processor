package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultTimeout   = 120 * time.Second
	defaultRateLimit = 5 // requests per second
)

// SystemPrompt is the default instruction sent to the LLM alongside the recipe text.
const SystemPrompt = `You are a recipe parser.
					Rules:
					- Translate everything to Dutch.
					- Output ONLY valid JSON.
					- If info is missing, use "" or 0.
					- Ensure 'total_time' is always in minutes.

					Important rules:
					- Return ONLY valid JSON, no markdown, no explanation
					- If information is missing, use reasonable defaults or null
					- Keep ingredients and instructions as arrays of strings
					- Times should be in minutes (integers)
					- All fields are required except CourseType (can be null if unclear)`

type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
	limiter    *rate.Limiter
}

func NewClient(baseURL, model string) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		limiter: rate.NewLimiter(rate.Limit(defaultRateLimit), 1),
	}
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system"`
	Format string `json:"format"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
}

func (c *Client) Process(ctx context.Context, input string) (string, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("rate limit wait: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	body, err := json.Marshal(generateRequest{
		Model:  c.model,
		Prompt: input,
		System: SystemPrompt,
		Format: "json",
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding ollama response: %w", err)
	}

	return result.Response, nil
}
