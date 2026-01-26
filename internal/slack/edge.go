package slack

import (
	"context"
	"net/http"
	"time"
)

const (
	// DefaultEdgeBaseURL is the base URL for Slack's Edge API.
	DefaultEdgeBaseURL = "https://edgeapi.slack.com"

	// DefaultHTTPTimeout is the default timeout for HTTP requests.
	DefaultHTTPTimeout = 30 * time.Second
)

// EdgeClient provides access to Slack's Edge API for fast channel detection.
type EdgeClient struct {
	creds      *Credentials
	httpClient *http.Client
	baseURL    string
}

// NewEdgeClient creates a new Edge API client with the given credentials.
func NewEdgeClient(creds *Credentials) *EdgeClient {
	return &EdgeClient{
		creds:      creds,
		httpClient: &http.Client{Timeout: DefaultHTTPTimeout},
		baseURL:    DefaultEdgeBaseURL,
	}
}

// WithBaseURL returns a new EdgeClient with the specified base URL.
// Useful for testing with mock servers.
func (c *EdgeClient) WithBaseURL(baseURL string) *EdgeClient {
	return &EdgeClient{
		creds:      c.creds,
		httpClient: c.httpClient,
		baseURL:    baseURL,
	}
}

// WithHTTPClient returns a new EdgeClient with the specified HTTP client.
// Useful for testing with custom transports.
func (c *EdgeClient) WithHTTPClient(client *http.Client) *EdgeClient {
	return &EdgeClient{
		creds:      c.creds,
		httpClient: client,
		baseURL:    c.baseURL,
	}
}

// ListChannels returns all channels the user has access to using the Edge API.
func (c *EdgeClient) ListChannels(ctx context.Context) ([]Channel, error) {
	// TODO: Implement Edge API channel listing
	return nil, nil
}
