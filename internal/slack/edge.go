package slack

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

// post sends an authenticated POST request to the Edge API.
// The endpoint is appended to {baseURL}/cache/{TeamID}/{endpoint}.
// Token is automatically added to the form body. Cookies from credentials are set.
//
//nolint:unused // Used by ClientUserBoot() and ClientCounts() methods (se-94m, se-15p)
func (c *EdgeClient) post(ctx context.Context, endpoint string, body map[string]any) ([]byte, error) {
	requestURL := fmt.Sprintf("%s/cache/%s/%s", c.baseURL, c.creds.TeamID, endpoint)

	// Build form data with token
	form := url.Values{}
	form.Set("token", c.creds.Token)

	for key, val := range body {
		form.Set(key, formatValue(val))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formEncoded := form.Encode()
	req.Body = io.NopCloser(strings.NewReader(formEncoded))
	req.ContentLength = int64(len(formEncoded))

	// Set cookies from credentials
	for _, cookie := range c.creds.Cookies {
		req.AddCookie(cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("edge API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return io.ReadAll(resp.Body)
}

// formatValue converts a value to string for form encoding.
//
//nolint:unused // Helper for post() method
func formatValue(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "1"
		}
		return "0"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// ListChannels returns all channels the user has access to using the Edge API.
func (c *EdgeClient) ListChannels(ctx context.Context) ([]Channel, error) {
	// TODO: Implement Edge API channel listing
	return nil, nil
}
