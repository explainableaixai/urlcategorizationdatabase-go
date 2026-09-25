// Package urlcategorizationdatabase provides a client for URL Categorization Database.
package urlcategorizationdatabase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Result map[string]any
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string { return fmt.Sprintf("API request failed with status %d", e.Status) }

type Client struct {
	APIKey, BaseURL string
	HTTPClient      *http.Client
}

func New(key string) *Client {
	return &Client{APIKey: key, BaseURL: "https://www.websitecategorizationapi.com/api", HTTPClient: &http.Client{Timeout: 30 * time.Second}}
}
func (c *Client) lookup(ctx context.Context, value string) (Result, error) {
	if strings.TrimSpace(c.APIKey) == "" || strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("API key and input are required")
	}
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/iab/iab_web_content_filtering.php"
	var req *http.Request
	var err error
	form := url.Values{"query": {value}, "data_type": {"url"}, "api_key": {c.APIKey}}
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err == nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 400 {
		return nil, &APIError{Status: resp.StatusCode, Body: string(raw)}
	}
	var result Result
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}
func (c *Client) Classify(ctx context.Context, value string) (Result, error) {
	return c.lookup(ctx, value)
}
