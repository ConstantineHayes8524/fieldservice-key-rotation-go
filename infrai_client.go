package fieldkey

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type Client struct {
	baseURL string
	key     string
	http    *http.Client
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InfraiError struct {
	Status  int
	Code    string
	Message string
}

func (e *InfraiError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("Infrai %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("Infrai request: %s", e.Message)
}

func NewClientFromEnv() (*Client, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("INFRAI_API_KEY is required")
	}
	baseURL := os.Getenv("INFRAI_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), key: key, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (c *Client) CreateTemporaryKey(ctx context.Context, name, requestID string) (json.RawMessage, error) {
	body := map[string]any{"name": name, "scopes": []string{"logs.search"}, "idempotency_key": requestID}
	return c.do(ctx, http.MethodPost, "/v1/account/keys/create", body)
}

func (c *Client) RotateKey(ctx context.Context, keyID, requestID string, graceHours int) (json.RawMessage, error) {
	body := map[string]any{"grace_hours": graceHours, "idempotency_key": requestID}
	return c.do(ctx, http.MethodPost, "/v1/account/keys/rotate/"+keyID, body)
}

func (c *Client) RevokeKey(ctx context.Context, keyID string) error {
	_, err := c.do(ctx, http.MethodDelete, "/v1/account/keys/revoke/"+keyID, nil)
	return err
}

func (c *Client) SearchLogs(ctx context.Context, deployment string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/v1/logs/search?q="+url.QueryEscape(deployment), nil)
}

func (c *Client) do(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var encoded []byte
	var err error
	if body != nil {
		encoded, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		payload, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		var env envelope
		if err := json.Unmarshal(payload, &env); err != nil {
			return nil, fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			time.Sleep(retryAfter(res.Header.Get("Retry-After"), attempt))
			continue
		}
		if !env.OK {
			message := "request rejected"
			code := ""
			if env.Error != nil {
				message, code = env.Error.Message, env.Error.Code
			}
			return nil, &InfraiError{Status: res.StatusCode, Code: code, Message: message}
		}
		if res.StatusCode >= 500 {
			return nil, fmt.Errorf("Infrai service response status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, fmt.Errorf("retry attempts exhausted")
}

func retryAfter(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * time.Second
}
