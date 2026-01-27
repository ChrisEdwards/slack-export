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

	// DefaultSlackAPIURL is the base URL for the standard Slack API.
	DefaultSlackAPIURL = "https://slack.com/api"

	// DefaultHTTPTimeout is the default timeout for HTTP requests.
	DefaultHTTPTimeout = 30 * time.Second
)

// EdgeClient provides access to Slack's Edge API for fast channel detection.
type EdgeClient struct {
	creds        *Credentials
	httpClient   *http.Client
	baseURL      string
	slackAPIURL  string
	workspaceURL string // Set by AuthTest, e.g., "https://myteam.slack.com/"
}

// NewEdgeClient creates a new Edge API client with the given credentials.
func NewEdgeClient(creds *Credentials) *EdgeClient {
	return &EdgeClient{
		creds:       creds,
		httpClient:  &http.Client{Timeout: DefaultHTTPTimeout},
		baseURL:     DefaultEdgeBaseURL,
		slackAPIURL: DefaultSlackAPIURL,
	}
}

// WithBaseURL returns a new EdgeClient with the specified base URL.
// Useful for testing with mock servers.
func (c *EdgeClient) WithBaseURL(baseURL string) *EdgeClient {
	return &EdgeClient{
		creds:        c.creds,
		httpClient:   c.httpClient,
		baseURL:      baseURL,
		slackAPIURL:  c.slackAPIURL,
		workspaceURL: c.workspaceURL,
	}
}

// WithSlackAPIURL returns a new EdgeClient with the specified Slack API URL.
// Useful for testing with mock servers.
func (c *EdgeClient) WithSlackAPIURL(slackAPIURL string) *EdgeClient {
	return &EdgeClient{
		creds:        c.creds,
		httpClient:   c.httpClient,
		baseURL:      c.baseURL,
		slackAPIURL:  slackAPIURL,
		workspaceURL: c.workspaceURL,
	}
}

// WithWorkspaceURL returns a new EdgeClient with the specified workspace URL.
// Useful for testing with mock servers. The URL should end with a trailing slash.
func (c *EdgeClient) WithWorkspaceURL(workspaceURL string) *EdgeClient {
	return &EdgeClient{
		creds:        c.creds,
		httpClient:   c.httpClient,
		baseURL:      c.baseURL,
		slackAPIURL:  c.slackAPIURL,
		workspaceURL: workspaceURL,
	}
}

// WithHTTPClient returns a new EdgeClient with the specified HTTP client.
// Useful for testing with custom transports.
func (c *EdgeClient) WithHTTPClient(client *http.Client) *EdgeClient {
	return &EdgeClient{
		creds:        c.creds,
		httpClient:   client,
		baseURL:      c.baseURL,
		slackAPIURL:  c.slackAPIURL,
		workspaceURL: c.workspaceURL,
	}
}

// post sends an authenticated POST request to the Slack webclient API.
// The endpoint is appended to {workspaceURL}api/{endpoint}.
// Token is automatically added to the form body. Cookies from credentials are set.
// Note: AuthTest must be called first to set workspaceURL.
func (c *EdgeClient) post(ctx context.Context, endpoint string, body map[string]any) ([]byte, error) {
	if c.workspaceURL == "" {
		return nil, fmt.Errorf("workspaceURL not set - call AuthTest first")
	}
	requestURL := fmt.Sprintf("%sapi/%s", c.workspaceURL, endpoint)

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

// AuthTest calls the Slack auth.test API to verify credentials and get workspace info.
// This must be called before using Edge API methods to obtain the TeamID.
// On success, it sets creds.TeamID to the workspace's team ID.
func (c *EdgeClient) AuthTest(ctx context.Context) (*AuthTestResponse, error) {
	requestURL := fmt.Sprintf("%s/auth.test", c.slackAPIURL)

	form := url.Values{}
	form.Set("token", c.creds.Token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formEncoded := form.Encode()
	req.Body = io.NopCloser(strings.NewReader(formEncoded))
	req.ContentLength = int64(len(formEncoded))

	for _, cookie := range c.creds.Cookies {
		req.AddCookie(cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth.test API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var authResp AuthTestResponse
	if err := json.Unmarshal(bodyBytes, &authResp); err != nil {
		return nil, fmt.Errorf("parsing auth.test response: %w", err)
	}

	if !authResp.OK {
		return nil, fmt.Errorf("auth.test failed: %s", authResp.Error)
	}

	c.creds.TeamID = authResp.TeamID
	c.workspaceURL = authResp.URL
	return &authResp, nil
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

// FetchUsers retrieves all users in the workspace using the Slack users.list API.
// This uses the standard Slack API (not Edge API) with Tier 2 rate limiting.
// Returns a UserIndex for O(1) lookups by user ID.
func (c *EdgeClient) FetchUsers(ctx context.Context) (UserIndex, error) {
	var allUsers []User
	cursor := ""

	for {
		users, nextCursor, err := c.fetchUsersPage(ctx, cursor)
		if err != nil {
			return nil, err
		}
		allUsers = append(allUsers, users...)

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	return NewUserIndex(allUsers), nil
}

// fetchUsersPage fetches a single page of users from the users.list API.
func (c *EdgeClient) fetchUsersPage(ctx context.Context, cursor string) ([]User, string, error) {
	requestURL := fmt.Sprintf("%s/users.list", c.slackAPIURL)

	form := url.Values{}
	form.Set("token", c.creds.Token)
	form.Set("limit", "200")
	form.Set("include_locale", "false")
	if cursor != "" {
		form.Set("cursor", cursor)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	formEncoded := form.Encode()
	req.Body = io.NopCloser(strings.NewReader(formEncoded))
	req.ContentLength = int64(len(formEncoded))

	for _, cookie := range c.creds.Cookies {
		req.AddCookie(cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("users.list API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var usersResp UsersListResponse
	if err := json.Unmarshal(bodyBytes, &usersResp); err != nil {
		return nil, "", fmt.Errorf("parsing users.list response: %w", err)
	}

	if !usersResp.OK {
		return nil, "", fmt.Errorf("users.list failed: %s", usersResp.Error)
	}

	return usersResp.Members, usersResp.ResponseMetadata.NextCursor, nil
}

// FetchUserInfo fetches a single user's info via the Slack users.info API.
// This is used for external Slack Connect users not in the workspace user list.
func (c *EdgeClient) FetchUserInfo(ctx context.Context, userID string) (*User, error) {
	requestURL := fmt.Sprintf("%s/users.info", c.slackAPIURL)

	form := url.Values{}
	form.Set("token", c.creds.Token)
	form.Set("user", userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range c.creds.Cookies {
		req.AddCookie(cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("users.info request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("users.info: HTTP %d", resp.StatusCode)
	}

	var result UserInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding users.info response: %w", err)
	}

	if !result.OK {
		return nil, fmt.Errorf("users.info: %s", result.Error)
	}

	return &result.User, nil
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
// DM names will show user IDs (dm_U123) since no user lookup is performed.
func (c *EdgeClient) GetActiveChannels(ctx context.Context, since time.Time) ([]Channel, error) {
	return c.GetActiveChannelsWithUsers(ctx, since, nil)
}

// GetActiveChannelsWithUsers returns channels with activity since the given time.
// If userIndex is provided, DM names will show display names (dm_alice) instead of IDs.
// Combines channel metadata from userBoot with timestamps from counts.
// If since is zero time, returns all channels.
func (c *EdgeClient) GetActiveChannelsWithUsers(
	ctx context.Context,
	since time.Time,
	userIndex UserIndex,
) ([]Channel, error) {
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
			Name:        resolveDMName(im.User, userIndex),
			IsIM:        true,
			LastMessage: latest,
		})
	}

	return active, nil
}

// resolveDMName generates a DM channel name from a user ID.
// If userIndex is provided, uses the username (e.g., "john.ament"); otherwise uses the raw ID.
// The result matches the format used in MPDM channel names.
func resolveDMName(userID string, userIndex UserIndex) string {
	if userIndex == nil {
		return fmt.Sprintf("dm_%s", userID)
	}
	return fmt.Sprintf("dm_%s", userIndex.Username(userID))
}

// GetActiveChannelsWithResolver returns active channels with DM names resolved via UserResolver.
// This supports external Slack Connect users through cache and API fallback.
func (c *EdgeClient) GetActiveChannelsWithResolver(
	ctx context.Context,
	since time.Time,
	resolver *UserResolver,
) ([]Channel, error) {
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

	// Process regular channels
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

	// Process DMs with resolver
	for _, im := range boot.IMs {
		latest := latestByID[im.ID]
		if !includeAll && (latest.IsZero() || latest.Before(since)) {
			continue
		}

		name, err := resolveDMNameWithResolver(ctx, im.User, resolver)
		if err != nil {
			return nil, fmt.Errorf("resolving DM user %s: %w", im.User, err)
		}

		active = append(active, Channel{
			ID:          im.ID,
			Name:        name,
			IsIM:        true,
			LastMessage: latest,
		})
	}

	return active, nil
}

// resolveDMNameWithResolver generates a DM channel name using the UserResolver.
func resolveDMNameWithResolver(ctx context.Context, userID string, resolver *UserResolver) (string, error) {
	if resolver == nil {
		return fmt.Sprintf("dm_%s", userID), nil
	}
	username, err := resolver.Username(ctx, userID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("dm_%s", username), nil
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
