package llm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// GigaChatClient talks to the Sber GigaChat REST API.
// It handles OAuth token acquisition and caching (tokens live ~30 minutes).
type GigaChatClient struct {
	authURL  string
	apiURL   string
	model    string
	clientID string
	secret   string
	http     *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
	maxToks int
}

func NewGigaChatClient(authURL, apiURL, model, clientID, secret string, insecure bool) *GigaChatClient {
	tr := &http.Transport{}
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // explicit opt-in for local dev
	}
	return &GigaChatClient{
		authURL:  authURL,
		apiURL:   apiURL,
		model:    model,
		clientID: clientID,
		secret:   secret,
		http:     &http.Client{Transport: tr, Timeout: 60 * time.Second},
		maxToks:  512,
	}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"` // unix milliseconds
}

func (c *GigaChatClient) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Before(c.expires.Add(-30*time.Second)) {
		return c.token, nil
	}

	creds := base64.StdEncoding.EncodeToString([]byte(c.clientID + ":" + c.secret))
	body := "scope=GIGACHAT_API_PERS"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.authURL, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("RqUID", uuid.NewString())
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gigachat auth: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gigachat auth: status %d: %s", resp.StatusCode, truncate(string(data), 300))
	}

	var tr tokenResponse
	if err := json.Unmarshal(data, &tr); err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("gigachat auth: empty access token")
	}

	c.token = tr.AccessToken
	if tr.ExpiresAt > 0 {
		c.expires = time.UnixMilli(tr.ExpiresAt)
	} else {
		c.expires = time.Now().Add(29 * time.Minute)
	}
	return c.token, nil
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *GigaChatClient) Chat(ctx context.Context, messages []Message) (string, error) {
	return c.complete(ctx, chatRequest{Model: c.model, Messages: messages, Temperature: 0.7, MaxTokens: c.maxToks})
}

func (c *GigaChatClient) ScoreDialogue(ctx context.Context, input ScoringInput) (ScoreResult, error) {
	raw, err := c.complete(ctx, chatRequest{
		Model:       c.model,
		Messages:    []Message{{Role: "system", Content: ScoringPrompt(input)}},
		Temperature: 0,
		MaxTokens:   256,
	})
	if err != nil {
		return ScoreResult{}, err
	}
	return ParseScoreResult(raw)
}

func (c *GigaChatClient) complete(ctx context.Context, payload chatRequest) (string, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gigachat chat: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gigachat chat: status %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", err
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("gigachat chat: no choices returned")
	}
	return cr.Choices[0].Message.Content, nil
}

// Classify asks the model to pick exactly one category for the text.
// Context is intentionally minimal to save tokens: only the text and the
// list of categories are sent.
func (c *GigaChatClient) Classify(ctx context.Context, text string, categories []string) (string, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return "", err
	}

	sys := "Ты классификатор реплик проводника поезда. Отнеси реплику ровно к одной категории из списка. " +
		"Верни ТОЛЬКО название категории без пояснений, кавычек и лишних символов."
	user := fmt.Sprintf("Категории: %s\nРеплика проводника: %s", strings.Join(categories, ", "), text)

	payload := chatRequest{
		Model: c.model,
		Messages: []Message{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
		Temperature: 0,
		MaxTokens:   16,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gigachat classify: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gigachat classify: status %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", err
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("gigachat classify: no choices")
	}

	raw := strings.TrimSpace(cr.Choices[0].Message.Content)
	for _, cat := range categories {
		if strings.EqualFold(strings.TrimSpace(raw), cat) {
			return cat, nil
		}
	}
	// Fallback: contains match.
	for _, cat := range categories {
		if strings.Contains(strings.ToLower(raw), strings.ToLower(cat)) {
			return cat, nil
		}
	}
	return raw, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

var _ LLMClient = (*GigaChatClient)(nil)
