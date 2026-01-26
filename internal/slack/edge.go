package slack

import (
	"context"
	"encoding/json"
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

// ClientUserBoot calls the client.userBoot Edge API endpoint.
// Returns all channels, DMs, and groups the user has access to with metadata.
func (c *EdgeClient) ClientUserBoot(ctx context.Context) (*UserBootResponse, error) {
	data, err := c.post(ctx, "client.userBoot", map[string]any{
		"include_permissions": true,
		"only_self_subteams":  true,
	})
	if err != nil {
		return nil, err
	}

	var resp UserBootResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing userBoot response: %w", err)
	}

	if !resp.OK {
		return nil, fmt.Errorf("userBoot API error: %s", resp.Error)
	}

	return &resp, nil
}

// ParseSlackTS parses a Slack timestamp string into a time.Time.
// Slack timestamps are in the format "1737676800.123456" where the integer part
// is Unix seconds and the decimal part is microseconds.
// Returns zero time for empty string.
func ParseSlackTS(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, nil
	}

	parts := strings.Split(ts, ".")
	secs, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing seconds: %w", err)
	}

	var nsecs int64
	if len(parts) > 1 && parts[1] != "" {
		// Pad to 6 digits for microseconds
		micro := parts[1]
		for len(micro) < 6 {
			micro += "0"
		}
		// Truncate if longer than 6 digits
		if len(micro) > 6 {
			micro = micro[:6]
		}
		microVal, err := strconv.ParseInt(micro, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("parsing microseconds: %w", err)
		}
		// Convert microseconds to nanoseconds
		nsecs = microVal * 1000
	}

	return time.Unix(secs, nsecs), nil
}

// ClientCounts calls the client.counts Edge API endpoint.
// Returns activity timestamps showing when each channel last had a message.
func (c *EdgeClient) ClientCounts(ctx context.Context) (*CountsResponse, error) {
	data, err := c.post(ctx, "client.counts", map[string]any{
		"thread_counts_by_channel": true,
		"org_wide_aware":           true,
		"include_file_channels":    true,
	})
	if err != nil {
		return nil, err
	}

	var resp CountsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing counts response: %w", err)
	}

	if !resp.OK {
		return nil, fmt.Errorf("counts API error: %s", resp.Error)
	}

	return &resp, nil
}

// GetActiveChannels returns channels with activity since the given time.
// Combines channel metadata from userBoot with timestamps from counts.
// If since is zero time, returns all channels.
func (c *EdgeClient) GetActiveChannels(ctx context.Context, since time.Time) ([]Channel, error) {
	boot, err := c.ClientUserBoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("userBoot: %w", err)
	}

	counts, err := c.ClientCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("counts: %w", err)
	}

	latestByID := buildTimestampLookup(counts)
	includeAll := since.IsZero()

	var active []Channel

	for _, ch := range boot.Channels {
		latest := latestByID[ch.ID]
		if !includeAll && (latest.IsZero() || latest.Before(since)) {
			continue
		}
		active = append(active, Channel{
			ID:          ch.ID,
			Name:        ch.Name,
			IsChannel:   ch.IsChannel,
			IsGroup:     ch.IsGroup,
			IsPrivate:   ch.IsPrivate,
			IsArchived:  ch.IsArchived,
			IsMember:    ch.IsMember,
			IsMPIM:      ch.IsMpim,
			LastMessage: latest,
		})
	}

	for _, im := range boot.IMs {
		latest := latestByID[im.ID]
		if !includeAll && (latest.IsZero() || latest.Before(since)) {
			continue
		}
		active = append(active, Channel{
			ID:          im.ID,
			Name:        fmt.Sprintf("dm_%s", im.User),
			IsIM:        true,
			LastMessage: latest,
		})
	}

	return active, nil
}

// buildTimestampLookup creates a map from channel ID to latest message time.
func buildTimestampLookup(counts *CountsResponse) map[string]time.Time {
	lookup := make(map[string]time.Time)

	for _, ch := range counts.Channels {
		if t, err := ParseSlackTS(ch.Latest); err == nil && !t.IsZero() {
			lookup[ch.ID] = t
		}
	}

	for _, im := range counts.IMs {
		if t, err := ParseSlackTS(im.Latest); err == nil && !t.IsZero() {
			lookup[im.ID] = t
		}
	}

	for _, mpim := range counts.MPIMs {
		if t, err := ParseSlackTS(mpim.Latest); err == nil && !t.IsZero() {
			lookup[mpim.ID] = t
		}
	}

	return lookup
}
