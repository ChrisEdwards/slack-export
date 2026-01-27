# Resolve User IDs to Display Names for DM Channels

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace cryptic DM channel names like `dm_U015ANT8LLD` with human-readable names like `dm_chris.edwards`.

**Architecture:** Fetch all workspace users via Slack's standard `users.list` API, build an O(1) lookup map (UserIndex), and use it to resolve user IDs to display names when generating DM channel names. The name resolution follows a priority order: DisplayName > RealName > Username, with graceful fallback for external/unknown users.

**Tech Stack:** Go standard library, existing EdgeClient HTTP infrastructure

---

**Issue:** se-1d8 - Resolve user IDs to display names for DM channels
**Date:** 2026-01-26

## Current State

DM names are generated at `internal/slack/edge.go:341`:

```go
Name: fmt.Sprintf("dm_%s", im.User),  // im.User is the user ID like "U015ANT8LLD"
```

The `IM` struct only provides `User` (ID), not display name, resulting in output like:
- `dm_U015ANT8LLD` (current, hard to read)

## Target State

- `dm_chris.edwards` (human-readable display name)
- `dm_<unknown>:U015ANT8LLD` (graceful fallback for external users)

---

## Task 1: Add User Types to edge_types.go

**Files:**
- Modify: `internal/slack/edge_types.go`

**Step 1: Read the existing types file**

Already read. We need to add User, UserProfile, UserIndex types, and helper functions.

**Step 2: Add the User and UserProfile types**

Add at end of `internal/slack/edge_types.go`:

```go
// User represents a Slack workspace user from the users.list API.
type User struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	RealName string      `json:"real_name"`
	Deleted  bool        `json:"deleted"`
	Profile  UserProfile `json:"profile"`
}

// UserProfile contains profile information for a Slack user.
type UserProfile struct {
	DisplayName string `json:"display_name"`
	RealName    string `json:"real_name"`
}

// UsersListResponse is the response from the Slack users.list API.
type UsersListResponse struct {
	OK               bool   `json:"ok"`
	Error            string `json:"error,omitempty"`
	Members          []User `json:"members"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// UserIndex provides O(1) lookup of users by ID.
type UserIndex map[string]*User

// NewUserIndex builds a UserIndex from a slice of users.
func NewUserIndex(users []User) UserIndex {
	idx := make(UserIndex, len(users))
	for i := range users {
		idx[users[i].ID] = &users[i]
	}
	return idx
}

// DisplayName returns a human-readable name for the given user ID.
// Priority: Profile.DisplayName > RealName > Name
// Falls back to "<unknown>:ID" for unknown users.
func (idx UserIndex) DisplayName(id string) string {
	if id == "" {
		return "unknown"
	}
	user, ok := idx[id]
	if !ok {
		return "<unknown>:" + id
	}
	if user.Profile.DisplayName != "" {
		return user.Profile.DisplayName
	}
	if user.RealName != "" {
		return user.RealName
	}
	if user.Name != "" {
		return user.Name
	}
	return "<unknown>:" + id
}
```

**Step 3: Run tests to verify no regressions**

Run: `make check-test`
Expected: All tests pass

**Step 4: Commit**

```bash
git add internal/slack/edge_types.go
git commit -m "$(cat <<'EOF'
feat(slack): add User types and UserIndex for name resolution

Add User, UserProfile, UsersListResponse types and UserIndex map
with DisplayName method for resolving user IDs to human-readable
names. Supports fallback for unknown/external users.

Part of se-1d8: Resolve user IDs to display names for DM channels
EOF
)"
```

---

## Task 2: Add Tests for UserIndex

**Files:**
- Modify: `internal/slack/edge_test.go`

**Step 1: Write tests for NewUserIndex and DisplayName**

Add to end of `internal/slack/edge_test.go`:

```go
func TestNewUserIndex(t *testing.T) {
	users := []User{
		{ID: "U001", Name: "alice", RealName: "Alice Smith", Profile: UserProfile{DisplayName: "Alice"}},
		{ID: "U002", Name: "bob", RealName: "Bob Jones", Profile: UserProfile{}},
		{ID: "U003", Name: "carol", Profile: UserProfile{}},
	}

	idx := NewUserIndex(users)

	if len(idx) != 3 {
		t.Errorf("expected 3 users in index, got %d", len(idx))
	}

	if idx["U001"] == nil {
		t.Error("U001 should be in index")
	}

	if idx["U001"].Name != "alice" {
		t.Errorf("expected U001.Name=alice, got %s", idx["U001"].Name)
	}
}

func TestUserIndex_DisplayName(t *testing.T) {
	users := []User{
		{ID: "U001", Name: "alice", RealName: "Alice Smith", Profile: UserProfile{DisplayName: "Alice"}},
		{ID: "U002", Name: "bob", RealName: "Bob Jones", Profile: UserProfile{}},
		{ID: "U003", Name: "carol", Profile: UserProfile{}},
		{ID: "U004", Name: "", RealName: "", Profile: UserProfile{}},
	}

	idx := NewUserIndex(users)

	tests := []struct {
		name     string
		userID   string
		expected string
	}{
		{"prefers display name", "U001", "Alice"},
		{"falls back to real name", "U002", "Bob Jones"},
		{"falls back to username", "U003", "carol"},
		{"unknown user in index", "U004", "<unknown>:U004"},
		{"user not in index", "U999", "<unknown>:U999"},
		{"empty user ID", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := idx.DisplayName(tt.userID)
			if got != tt.expected {
				t.Errorf("DisplayName(%q) = %q, want %q", tt.userID, got, tt.expected)
			}
		})
	}
}

func TestUserIndex_Empty(t *testing.T) {
	idx := NewUserIndex(nil)

	if len(idx) != 0 {
		t.Errorf("expected empty index, got %d entries", len(idx))
	}

	got := idx.DisplayName("U123")
	if got != "<unknown>:U123" {
		t.Errorf("expected <unknown>:U123, got %s", got)
	}
}
```

**Step 2: Run the new tests**

Run: `go test -v ./internal/slack/... -run TestUserIndex`
Expected: All 3 new tests pass

**Step 3: Commit**

```bash
git add internal/slack/edge_test.go
git commit -m "$(cat <<'EOF'
test(slack): add tests for UserIndex and DisplayName

Cover NewUserIndex construction, DisplayName priority order
(display name > real name > username), and edge cases for
empty/unknown users.

Part of se-1d8: Resolve user IDs to display names for DM channels
EOF
)"
```

---

## Task 3: Add FetchUsers Method to EdgeClient

**Files:**
- Modify: `internal/slack/edge.go`

**Step 1: Add the FetchUsers method**

Add after the `ClientCounts` method (after line ~295) in `internal/slack/edge.go`:

```go
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
```

**Step 2: Run tests to verify no regressions**

Run: `make check-test`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/slack/edge.go
git commit -m "$(cat <<'EOF'
feat(slack): add FetchUsers method to EdgeClient

Add FetchUsers method that fetches all workspace users via Slack's
users.list API with pagination support. Returns a UserIndex for
O(1) lookups by user ID.

Part of se-1d8: Resolve user IDs to display names for DM channels
EOF
)"
```

---

## Task 4: Add Tests for FetchUsers

**Files:**
- Modify: `internal/slack/edge_test.go`

**Step 1: Write tests for FetchUsers**

Add to `internal/slack/edge_test.go`:

```go
func TestEdgeClient_FetchUsers_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users.list" {
			t.Errorf("expected path /users.list, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ok": true,
			"members": [
				{"id": "U001", "name": "alice", "real_name": "Alice Smith", "profile": {"display_name": "Alice"}},
				{"id": "U002", "name": "bob", "real_name": "Bob Jones", "profile": {}}
			],
			"response_metadata": {"next_cursor": ""}
		}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithSlackAPIURL(server.URL)

	idx, err := client.FetchUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(idx) != 2 {
		t.Fatalf("expected 2 users, got %d", len(idx))
	}

	if idx.DisplayName("U001") != "Alice" {
		t.Errorf("expected Alice, got %s", idx.DisplayName("U001"))
	}

	if idx.DisplayName("U002") != "Bob Jones" {
		t.Errorf("expected Bob Jones, got %s", idx.DisplayName("U002"))
	}
}

func TestEdgeClient_FetchUsers_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, _ := io.ReadAll(r.Body)
		params, _ := url.ParseQuery(string(body))

		if callCount == 1 {
			if params.Get("cursor") != "" {
				t.Error("first call should not have cursor")
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"members": [{"id": "U001", "name": "alice"}],
				"response_metadata": {"next_cursor": "cursor123"}
			}`))
		} else {
			if params.Get("cursor") != "cursor123" {
				t.Errorf("expected cursor123, got %s", params.Get("cursor"))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"members": [{"id": "U002", "name": "bob"}],
				"response_metadata": {"next_cursor": ""}
			}`))
		}
	}))
	defer server.Close()

	creds := &Credentials{Token: "xoxc-test-token"}
	client := NewEdgeClient(creds).WithSlackAPIURL(server.URL)

	idx, err := client.FetchUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}

	if len(idx) != 2 {
		t.Fatalf("expected 2 users, got %d", len(idx))
	}
}

func TestEdgeClient_FetchUsers_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": false, "error": "invalid_auth"}`))
	}))
	defer server.Close()

	creds := &Credentials{Token: "xoxc-test-token"}
	client := NewEdgeClient(creds).WithSlackAPIURL(server.URL)

	_, err := client.FetchUsers(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(err.Error(), "invalid_auth") {
		t.Errorf("expected invalid_auth error, got: %v", err)
	}
}

func TestEdgeClient_FetchUsers_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ok": true,
			"members": [],
			"response_metadata": {"next_cursor": ""}
		}`))
	}))
	defer server.Close()

	creds := &Credentials{Token: "xoxc-test-token"}
	client := NewEdgeClient(creds).WithSlackAPIURL(server.URL)

	idx, err := client.FetchUsers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(idx) != 0 {
		t.Errorf("expected empty index, got %d users", len(idx))
	}
}
```

**Step 2: Run the new tests**

Run: `go test -v ./internal/slack/... -run TestEdgeClient_FetchUsers`
Expected: All 4 new tests pass

**Step 3: Commit**

```bash
git add internal/slack/edge_test.go
git commit -m "$(cat <<'EOF'
test(slack): add tests for FetchUsers method

Cover successful fetch, pagination handling, API errors,
and empty workspace edge case.

Part of se-1d8: Resolve user IDs to display names for DM channels
EOF
)"
```

---

## Task 5: Update GetActiveChannels to Use UserIndex

**Files:**
- Modify: `internal/slack/edge.go`

**Step 1: Update GetActiveChannels to accept an optional UserIndex**

The current signature is:
```go
func (c *EdgeClient) GetActiveChannels(ctx context.Context, since time.Time) ([]Channel, error)
```

Add a new method that accepts UserIndex, and update the original to call it:

Replace the existing `GetActiveChannels` function with:

```go
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
func (c *EdgeClient) GetActiveChannelsWithUsers(ctx context.Context, since time.Time, userIndex UserIndex) ([]Channel, error) {
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
		dmName := resolveDMName(im.User, userIndex)
		active = append(active, Channel{
			ID:          im.ID,
			Name:        dmName,
			IsIM:        true,
			LastMessage: latest,
		})
	}

	return active, nil
}

// resolveDMName generates a DM channel name from a user ID.
// If userIndex is provided, uses the display name; otherwise uses the raw ID.
func resolveDMName(userID string, userIndex UserIndex) string {
	if userIndex == nil {
		return fmt.Sprintf("dm_%s", userID)
	}
	return fmt.Sprintf("dm_%s", userIndex.DisplayName(userID))
}
```

**Step 2: Run tests to verify backwards compatibility**

Run: `make check-test`
Expected: All existing tests still pass (GetActiveChannels behavior unchanged)

**Step 3: Commit**

```bash
git add internal/slack/edge.go
git commit -m "$(cat <<'EOF'
feat(slack): add GetActiveChannelsWithUsers for DM name resolution

Add GetActiveChannelsWithUsers method that accepts an optional
UserIndex to resolve DM user IDs to display names. Original
GetActiveChannels delegates to it with nil for backwards compat.

Part of se-1d8: Resolve user IDs to display names for DM channels
EOF
)"
```

---

## Task 6: Add Tests for GetActiveChannelsWithUsers

**Files:**
- Modify: `internal/slack/edge_test.go`

**Step 1: Write tests for the new method**

Add to `internal/slack/edge_test.go`:

```go
func TestEdgeClient_GetActiveChannelsWithUsers_ResolvesNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T123", "name": "testuser"},
				"team": {"id": "T123", "name": "Test Team", "domain": "test"},
				"ims": [
					{"id": "D001", "user": "U001", "is_im": true},
					{"id": "D002", "user": "U002", "is_im": true},
					{"id": "D003", "user": "U999", "is_im": true}
				],
				"channels": []
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"ims": [
					{"id": "D001", "latest": "1737676900.000000"},
					{"id": "D002", "latest": "1737676900.000000"},
					{"id": "D003", "latest": "1737676900.000000"}
				]
			}`))
		}
	}))
	defer server.Close()

	creds := &Credentials{Token: "xoxc-test-token", Workspace: "test"}
	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

	userIndex := NewUserIndex([]User{
		{ID: "U001", Name: "alice", Profile: UserProfile{DisplayName: "Alice"}},
		{ID: "U002", Name: "bob", RealName: "Bob Jones", Profile: UserProfile{}},
	})

	channels, err := client.GetActiveChannelsWithUsers(context.Background(), time.Time{}, userIndex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(channels) != 3 {
		t.Fatalf("expected 3 DMs, got %d", len(channels))
	}

	nameByID := make(map[string]string)
	for _, ch := range channels {
		nameByID[ch.ID] = ch.Name
	}

	if nameByID["D001"] != "dm_Alice" {
		t.Errorf("D001: expected dm_Alice, got %s", nameByID["D001"])
	}

	if nameByID["D002"] != "dm_Bob Jones" {
		t.Errorf("D002: expected dm_Bob Jones, got %s", nameByID["D002"])
	}

	if nameByID["D003"] != "dm_<unknown>:U999" {
		t.Errorf("D003: expected dm_<unknown>:U999, got %s", nameByID["D003"])
	}
}

func TestEdgeClient_GetActiveChannelsWithUsers_NilIndex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {}, "team": {},
				"ims": [{"id": "D001", "user": "U456", "is_im": true}],
				"channels": []
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"ims": [{"id": "D001", "latest": "1737676900.000000"}]
			}`))
		}
	}))
	defer server.Close()

	creds := &Credentials{Token: "xoxc-test-token", Workspace: "test"}
	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

	channels, err := client.GetActiveChannelsWithUsers(context.Background(), time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(channels) != 1 {
		t.Fatalf("expected 1 DM, got %d", len(channels))
	}

	if channels[0].Name != "dm_U456" {
		t.Errorf("expected dm_U456, got %s", channels[0].Name)
	}
}

func TestResolveDMName(t *testing.T) {
	userIndex := NewUserIndex([]User{
		{ID: "U001", Name: "alice", Profile: UserProfile{DisplayName: "Alice"}},
	})

	tests := []struct {
		name      string
		userID    string
		index     UserIndex
		expected  string
	}{
		{"with index and known user", "U001", userIndex, "dm_Alice"},
		{"with index and unknown user", "U999", userIndex, "dm_<unknown>:U999"},
		{"nil index", "U001", nil, "dm_U001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveDMName(tt.userID, tt.index)
			if got != tt.expected {
				t.Errorf("resolveDMName(%q, index) = %q, want %q", tt.userID, got, tt.expected)
			}
		})
	}
}
```

**Step 2: Run the new tests**

Run: `go test -v ./internal/slack/... -run "TestEdgeClient_GetActiveChannelsWithUsers|TestResolveDMName"`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/slack/edge_test.go
git commit -m "$(cat <<'EOF'
test(slack): add tests for GetActiveChannelsWithUsers

Cover DM name resolution with known users, unknown users,
and nil UserIndex fallback to raw IDs.

Part of se-1d8: Resolve user IDs to display names for DM channels
EOF
)"
```

---

## Task 7: Update Exporter to Fetch Users and Use New Method

**Files:**
- Modify: `internal/export/exporter.go`

**Step 1: Read the exporter to understand the current usage**

First, we need to find where `GetActiveChannels` is called. Let me check the exporter.

The exporter likely calls `edgeClient.GetActiveChannels(ctx, since)` and needs to:
1. First call `FetchUsers` to get the user index
2. Then call `GetActiveChannelsWithUsers` with the index

**Step 2: Find and update the call site**

Search for `GetActiveChannels` in the exporter and update it:

```go
// Before:
channels, err := client.GetActiveChannels(ctx, since)

// After:
userIndex, err := client.FetchUsers(ctx)
if err != nil {
    return fmt.Errorf("fetching users: %w", err)
}

channels, err := client.GetActiveChannelsWithUsers(ctx, since, userIndex)
```

**Step 3: Run tests to verify integration**

Run: `make check-test`
Expected: All tests pass

**Step 4: Commit**

```bash
git add internal/export/exporter.go
git commit -m "$(cat <<'EOF'
feat(export): use FetchUsers and GetActiveChannelsWithUsers

Fetch workspace users before getting active channels so that
DM names display human-readable usernames instead of IDs.

Closes se-1d8: Resolve user IDs to display names for DM channels
EOF
)"
```

---

## Task 8: Update Exporter Tests

**Files:**
- Modify: `internal/export/exporter_test.go`

**Step 1: Update mock server to handle users.list**

Find tests that mock the edge client and add responses for `/users.list` endpoint.

Add to the mock server handler:

```go
case "/users.list":
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(`{
        "ok": true,
        "members": [
            {"id": "U456", "name": "alice", "profile": {"display_name": "Alice"}}
        ],
        "response_metadata": {"next_cursor": ""}
    }`))
```

**Step 2: Update expected DM names in test assertions**

Find assertions like:
```go
"D123": "dm_U456"  // old
```

Update to:
```go
"D123": "dm_Alice"  // new - resolved from user index
```

**Step 3: Run tests**

Run: `make check-test`
Expected: All tests pass with updated expectations

**Step 4: Commit**

```bash
git add internal/export/exporter_test.go
git commit -m "$(cat <<'EOF'
test(export): update tests for user ID resolution

Add users.list mock responses and update DM name assertions
to expect resolved display names instead of raw user IDs.

Part of se-1d8: Resolve user IDs to display names for DM channels
EOF
)"
```

---

## Task 9: Final Integration Testing and Cleanup

**Files:**
- All modified files

**Step 1: Run full test suite**

Run: `make check-test`
Expected: All tests pass, no linting errors

**Step 2: Verify no file length limits exceeded**

Check that edge.go and edge_test.go don't exceed 500/800 lines respectively. If they do, consider extracting user-related code to a separate file.

**Step 3: Manual verification (if possible)**

If you have access to a real Slack workspace:
```bash
go run ./cmd/slack-export channels
```

Expected: DM channels show names like `dm_alice` instead of `dm_U015ANT8LLD`

**Step 4: Close the issue**

```bash
br close se-1d8 --reason "Implemented user ID to display name resolution for DM channels"
br sync --flush-only
git add .beads/
git commit -m "chore: close se-1d8 - user ID resolution implemented"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add User types and UserIndex | edge_types.go |
| 2 | Add tests for UserIndex | edge_test.go |
| 3 | Add FetchUsers method | edge.go |
| 4 | Add tests for FetchUsers | edge_test.go |
| 5 | Add GetActiveChannelsWithUsers | edge.go |
| 6 | Add tests for new method | edge_test.go |
| 7 | Update exporter to use new method | exporter.go |
| 8 | Update exporter tests | exporter_test.go |
| 9 | Final testing and cleanup | All |

## Edge Cases Handled

- **Unknown users:** Display as `dm_<unknown>:U123`
- **Empty user ID:** Display as `dm_unknown`
- **Deleted users:** Included in users.list, resolved normally
- **External users:** Not in workspace users.list, fall back to unknown format
- **Empty workspace:** UserIndex handles gracefully
- **Pagination:** FetchUsers handles multi-page responses
- **Backwards compatibility:** GetActiveChannels still works without users
