package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client is an HTTP client with mTLS support
type Client struct {
	baseURL    string
	httpClient *http.Client
	useMTLS    bool
}

// NewClient creates a new transport client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		useMTLS: false,
	}
}

// EnableMTLS configures the client to use mutual TLS
func (c *Client) EnableMTLS(certPath, keyPath, caPath string) error {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("failed to load client cert: %w", err)
	}

	caCert, err := os.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("failed to load CA cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	c.httpClient.Transport = &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	c.useMTLS = true

	return nil
}

// Post sends a POST request
func (c *Client) Post(ctx context.Context, path string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

// Get sends a GET request
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	return c.httpClient.Do(req)
}

// DecodeJSON decodes JSON response
func DecodeJSON(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
