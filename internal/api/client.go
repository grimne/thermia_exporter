// Package api provides a client for the Thermia API.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"thermia_exporter/internal/types"
)

const thermiaConfigURL = "https://online.thermia.se/api/configuration"

// APIClient handles HTTP requests to the Thermia API.
type APIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewAPIClient creates a new Thermia API client.
// It automatically discovers the API base URL from the configuration endpoint.
func NewAPIClient(ctx context.Context, token string, logger *slog.Logger) (*APIClient, error) {
	client := &APIClient{
		token:  token,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}

	// Discover API base URL
	cfg, err := client.getConfiguration(ctx)
	if err != nil {
		return nil, fmt.Errorf("get configuration: %w", err)
	}

	client.baseURL = strings.TrimRight(cfg.APIBaseURL, "/")
	logger.Debug("API client initialized", "base_url", client.baseURL)

	return client, nil
}

// doRequest performs an HTTP request with authentication and error handling.
func (c *APIClient) doRequest(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.logger.Debug("API request", "method", method, "path", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Request failed", "method", method, "path", path, "error", err)
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("Non-200 status", "method", method, "path", path, "status", resp.StatusCode)
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
	}

	c.logger.Debug("API response", "method", method, "path", path, "bytes", len(data))

	return data, nil
}

// getConfiguration retrieves the API configuration (base URL discovery).
func (c *APIClient) getConfiguration(ctx context.Context) (*types.Config, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", thermiaConfigURL, nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
	}

	var cfg types.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	return &cfg, nil
}
