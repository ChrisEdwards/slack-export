# External User Resolution Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Resolve external Slack Connect user IDs to usernames for DM channel naming via `users.info` API with persistent disk caching.

**Architecture:** Three-tier lookup (workspace users → disk cache → API fetch). New `UserResolver` combines `UserIndex` + `UserCache` + `UserFetcher` to provide unified username resolution. Cache persists to `~/.cache/slack-export/users.json`.

**Tech Stack:** Go, standard library only (encoding/json, os, path/filepath)

---

## Task 1: Add UserInfoResponse Type

**Files:**
- Modify: `internal/slack/edge_types.go:113` (after UsersListResponse)

**Step 1: Add the type definition**

Add after `UsersListResponse` (line 113):

```go
// UserInfoResponse is the response from the Slack users.info API.
// Used to fetch individual user details, especially for external Slack Connect users.
type UserInfoResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	User  User   `json:"user"`
}
```

**Step 2: Run tests to verify no breakage**

Run: `make check-test`
Expected: All tests pass

**Step 3: Commit**

```bash
git add internal/slack/edge_types.go
git commit -m "feat(slack): add UserInfoResponse type for users.info API"
```

---

## Task 2: Add FetchUserInfo Method to EdgeClient

**Files:**
- Modify: `internal/slack/edge.go` (after FetchUsers method, around line 347)
- Test: `internal/slack/edge_test.go`

**Step 1: Write the failing test**

Add to `internal/slack/edge_test.go`:

```go
func TestEdgeClient_FetchUserInfo_Success(t *testing.T) {
	var capturedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		capturedBody = string(bodyBytes)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ok": true,
			"user": {
				"id": "U03A0EQBAS3",
				"name": "external.user",
				"real_name": "External User",
				"deleted": false,
				"profile": {
					"display_name": "External",
					"real_name": "External User"
				}
			}
		}`))
	}))
	defer server.Close()

	creds := &Credentials{Token: "xoxc-test-token"}
	client := NewEdgeClient(creds).WithSlackAPIURL(server.URL)

	user, err := client.FetchUserInfo(context.Background(), "U03A0EQBAS3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.ID != "U03A0EQBAS3" {
		t.Errorf("expected ID U03A0EQBAS3, got %s", user.ID)
	}
	if user.Name != "external.user" {
		t.Errorf("expected name external.user, got %s", user.Name)
	}

	// Verify request format
	if !strings.Contains(capturedBody, "user=U03A0EQBAS3") {
		t.Errorf("expected user ID in request body, got: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "token=xoxc-test-token") {
		t.Errorf("expected token in request body, got: %s", capturedBody)
	}
}

func TestEdgeClient_FetchUserInfo_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": false, "error": "user_not_found"}`))
	}))
	defer server.Close()

	creds := &Credentials{Token: "xoxc-test-token"}
	client := NewEdgeClient(creds).WithSlackAPIURL(server.URL)

	_, err := client.FetchUserInfo(context.Background(), "U_INVALID")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
	if !strings.Contains(err.Error(), "user_not_found") {
		t.Errorf("expected user_not_found in error, got: %v", err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestEdgeClient_FetchUserInfo ./internal/slack/`
Expected: FAIL - method does not exist

**Step 3: Write minimal implementation**

Add to `internal/slack/edge.go` after `fetchUsersPage` function (around line 347):

```go
// FetchUserInfo fetches a single user's info via the Slack users.info API.
// This is used for external Slack Connect users not in the workspace user list.
func (c *EdgeClient) FetchUserInfo(ctx context.Context, userID string) (*User, error) {
	form := url.Values{}
	form.Set("token", c.creds.Token)
	form.Set("user", userID)

	apiURL := c.slackAPIURL + "/users.info"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
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
	defer resp.Body.Close()

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
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestEdgeClient_FetchUserInfo ./internal/slack/`
Expected: PASS

**Step 5: Run full test suite**

Run: `make check-test`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/slack/edge.go internal/slack/edge_test.go
git commit -m "feat(slack): add FetchUserInfo method for users.info API"
```

---

## Task 3: Create UserCache Type

**Files:**
- Create: `internal/slack/usercache.go`
- Test: `internal/slack/usercache_test.go`

**Step 1: Write the failing test for cache structure**

Create `internal/slack/usercache_test.go`:

```go
package slack

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUserCache_GetSet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")

	cache := NewUserCache(path)

	// Initially empty
	if user := cache.Get("U123"); user != nil {
		t.Error("expected nil for missing user")
	}

	// Set a user
	testUser := &User{
		ID:       "U123",
		Name:     "testuser",
		RealName: "Test User",
	}
	cache.Set(testUser)

	// Get it back
	got := cache.Get("U123")
	if got == nil {
		t.Fatal("expected user after Set")
	}
	if got.Name != "testuser" {
		t.Errorf("expected name testuser, got %s", got.Name)
	}
}

func TestUserCache_Persistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.json")

	// Create and populate cache
	cache1 := NewUserCache(path)
	cache1.Set(&User{ID: "U123", Name: "testuser"})
	if err := cache1.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Load in new instance
	cache2 := NewUserCache(path)
	if err := cache2.Load(); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	got := cache2.Get("U123")
	if got == nil {
		t.Fatal("expected user after Load")
	}
	if got.Name != "testuser" {
		t.Errorf("expected name testuser, got %s", got.Name)
	}
}

func TestUserCache_LoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	cache := NewUserCache(path)
	err := cache.Load()
	if err != nil {
		t.Errorf("Load of nonexistent file should not error, got: %v", err)
	}
}

func TestUserCache_SaveCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "users.json")

	cache := NewUserCache(path)
	cache.Set(&User{ID: "U123", Name: "testuser"})

	if err := cache.Save(); err != nil {
		t.Fatalf("Save should create parent dir: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected file to exist after Save")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestUserCache ./internal/slack/`
Expected: FAIL - type does not exist

**Step 3: Write minimal implementation**

Create `internal/slack/usercache.go`:

```go
package slack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CachedUser wraps a User with fetch metadata.
type CachedUser struct {
	User      User  `json:"user"`
	FetchedAt int64 `json:"fetched_at"`
}

// CacheData is the top-level structure for the cache file.
type CacheData struct {
	Version int                   `json:"version"`
	Users   map[string]CachedUser `json:"users"`
}

// UserCache provides persistent caching for external Slack users.
// Thread-safe for concurrent access.
type UserCache struct {
	path  string
	mu    sync.RWMutex
	users map[string]*User
}

// NewUserCache creates a new UserCache that persists to the given path.
func NewUserCache(path string) *UserCache {
	return &UserCache{
		path:  path,
		users: make(map[string]*User),
	}
}

// Get returns a cached user by ID, or nil if not found.
func (c *UserCache) Get(id string) *User {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.users[id]
}

// Set adds or updates a user in the cache.
func (c *UserCache) Set(user *User) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.users[user.ID] = user
}

// Load reads the cache from disk. Returns nil if file doesn't exist.
func (c *UserCache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var cacheData CacheData
	if err := json.Unmarshal(data, &cacheData); err != nil {
		return err
	}

	c.users = make(map[string]*User, len(cacheData.Users))
	for id, cached := range cacheData.Users {
		user := cached.User
		c.users[id] = &user
	}
	return nil
}

// Save writes the cache to disk.
func (c *UserCache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		return err
	}

	now := time.Now().Unix()
	cacheData := CacheData{
		Version: 1,
		Users:   make(map[string]CachedUser, len(c.users)),
	}

	for id, user := range c.users {
		cacheData.Users[id] = CachedUser{
			User:      *user,
			FetchedAt: now,
		}
	}

	data, err := json.MarshalIndent(cacheData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.path, data, 0644)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestUserCache ./internal/slack/`
Expected: PASS

**Step 5: Run full test suite**

Run: `make check-test`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/slack/usercache.go internal/slack/usercache_test.go
git commit -m "feat(slack): add UserCache for persistent external user storage"
```

---

## Task 4: Add UserFetcher Interface and UserResolver

**Files:**
- Modify: `internal/slack/edge_types.go` (after UserIndex methods)
- Test: `internal/slack/edge_types_test.go` (create if needed)

**Step 1: Write the failing test**

Create `internal/slack/userresolver_test.go`:

```go
package slack

import (
	"context"
	"errors"
	"testing"
)

// mockFetcher implements UserFetcher for testing.
type mockFetcher struct {
	users map[string]*User
	err   error
	calls []string
}

func (m *mockFetcher) FetchUserInfo(_ context.Context, id string) (*User, error) {
	m.calls = append(m.calls, id)
	if m.err != nil {
		return nil, m.err
	}
	if user, ok := m.users[id]; ok {
		return user, nil
	}
	return nil, errors.New("user_not_found")
}

func TestUserResolver_UsernameFromIndex(t *testing.T) {
	idx := NewUserIndex([]User{{ID: "U123", Name: "localuser"}})
	cache := NewUserCache("")
	fetcher := &mockFetcher{}

	resolver := NewUserResolver(idx, cache, fetcher)

	name, err := resolver.Username(context.Background(), "U123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "localuser" {
		t.Errorf("expected localuser, got %s", name)
	}
	if len(fetcher.calls) > 0 {
		t.Error("should not call fetcher for indexed user")
	}
}

func TestUserResolver_UsernameFromCache(t *testing.T) {
	idx := NewUserIndex(nil)
	cache := NewUserCache("")
	cache.Set(&User{ID: "U456", Name: "cacheduser"})
	fetcher := &mockFetcher{}

	resolver := NewUserResolver(idx, cache, fetcher)

	name, err := resolver.Username(context.Background(), "U456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "cacheduser" {
		t.Errorf("expected cacheduser, got %s", name)
	}
	if len(fetcher.calls) > 0 {
		t.Error("should not call fetcher for cached user")
	}
}

func TestUserResolver_UsernameFromFetcher(t *testing.T) {
	idx := NewUserIndex(nil)
	cache := NewUserCache("")
	fetcher := &mockFetcher{
		users: map[string]*User{"U789": {ID: "U789", Name: "externaluser"}},
	}

	resolver := NewUserResolver(idx, cache, fetcher)

	name, err := resolver.Username(context.Background(), "U789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "externaluser" {
		t.Errorf("expected externaluser, got %s", name)
	}
	if len(fetcher.calls) != 1 || fetcher.calls[0] != "U789" {
		t.Errorf("expected single fetch call for U789, got %v", fetcher.calls)
	}

	// Verify user is now in cache
	if cached := cache.Get("U789"); cached == nil {
		t.Error("user should be cached after fetch")
	}
}

func TestUserResolver_FetcherError(t *testing.T) {
	idx := NewUserIndex(nil)
	cache := NewUserCache("")
	fetcher := &mockFetcher{err: errors.New("api error")}

	resolver := NewUserResolver(idx, cache, fetcher)

	_, err := resolver.Username(context.Background(), "U999")
	if err == nil {
		t.Fatal("expected error from fetcher")
	}
	if !errors.Is(err, fetcher.err) && err.Error() != "api error" {
		t.Errorf("expected api error, got %v", err)
	}
}

func TestUserResolver_NilFetcher(t *testing.T) {
	idx := NewUserIndex(nil)
	cache := NewUserCache("")

	resolver := NewUserResolver(idx, cache, nil)

	name, err := resolver.Username(context.Background(), "U999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Falls back to ID when fetcher is nil
	if name != "U999" {
		t.Errorf("expected U999 fallback, got %s", name)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestUserResolver ./internal/slack/`
Expected: FAIL - types do not exist

**Step 3: Write minimal implementation**

Add to `internal/slack/edge_types.go` at the end:

```go
// UserFetcher fetches user info from an external source (e.g., Slack API).
type UserFetcher interface {
	FetchUserInfo(ctx context.Context, userID string) (*User, error)
}

// UserResolver provides unified user lookup across multiple sources.
// Lookup order: UserIndex (workspace) → UserCache (disk) → UserFetcher (API).
type UserResolver struct {
	index   UserIndex
	cache   *UserCache
	fetcher UserFetcher
}

// NewUserResolver creates a resolver with the given sources.
// Any source can be nil - lookups will skip that source.
func NewUserResolver(index UserIndex, cache *UserCache, fetcher UserFetcher) *UserResolver {
	return &UserResolver{
		index:   index,
		cache:   cache,
		fetcher: fetcher,
	}
}

// Username returns the username for a user ID, checking sources in order.
// Returns the raw ID if user cannot be found and fetcher is nil or lookup succeeds.
func (r *UserResolver) Username(ctx context.Context, id string) (string, error) {
	if id == "" {
		return "unknown", nil
	}

	// 1. Check workspace index
	if r.index != nil {
		if user, ok := r.index[id]; ok && user.Name != "" {
			return strings.ToLower(user.Name), nil
		}
	}

	// 2. Check disk cache
	if r.cache != nil {
		if user := r.cache.Get(id); user != nil && user.Name != "" {
			return strings.ToLower(user.Name), nil
		}
	}

	// 3. Fetch from API
	if r.fetcher != nil {
		user, err := r.fetcher.FetchUserInfo(ctx, id)
		if err != nil {
			return "", err
		}
		if r.cache != nil {
			r.cache.Set(user)
		}
		if user.Name != "" {
			return strings.ToLower(user.Name), nil
		}
	}

	// Fallback to raw ID
	return id, nil
}
```

Add `"context"` to the imports at line 3:

```go
import (
	"context"
	"strings"
)
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestUserResolver ./internal/slack/`
Expected: PASS

**Step 5: Run full test suite**

Run: `make check-test`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/slack/edge_types.go internal/slack/userresolver_test.go
git commit -m "feat(slack): add UserFetcher interface and UserResolver"
```

---

## Task 5: Update resolveDMName to Use UserResolver

**Files:**
- Modify: `internal/slack/edge.go` (lines 385-447)
- Test: `internal/slack/edge_test.go`

**Step 1: Write the failing test**

Add to `internal/slack/edge_test.go`:

```go
func TestEdgeClient_GetActiveChannelsWithResolver_ExternalUser(t *testing.T) {
	// Mock server for userBoot
	userBootCalled := false
	countsCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "client.userBoot") {
			userBootCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U000", "team_id": "T123", "name": "self"},
				"team": {"id": "T123", "name": "TestTeam", "domain": "test"},
				"ims": [{"id": "D001", "user": "U_EXTERNAL", "is_im": true, "is_open": true}],
				"channels": []
			}`))
			return
		}
		if strings.Contains(r.URL.Path, "client.counts") {
			countsCalled = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"channels": [],
				"mpims": [],
				"ims": [{"id": "D001", "latest": "1700000000.000000"}]
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	creds := &Credentials{Token: "xoxc-test"}
	client := NewEdgeClient(creds).WithBaseURL(server.URL)
	client.workspaceURL = server.URL

	// Empty workspace index - user not found locally
	idx := NewUserIndex(nil)

	// Cache has the external user
	cache := NewUserCache("")
	cache.Set(&User{ID: "U_EXTERNAL", Name: "external.user"})

	resolver := NewUserResolver(idx, cache, nil)

	channels, err := client.GetActiveChannelsWithResolver(context.Background(), time.Time{}, resolver)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !userBootCalled || !countsCalled {
		t.Error("expected both API calls")
	}

	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}

	// Should resolve to username, not raw ID
	if channels[0].Name != "dm_external.user" {
		t.Errorf("expected dm_external.user, got %s", channels[0].Name)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestEdgeClient_GetActiveChannelsWithResolver ./internal/slack/`
Expected: FAIL - method does not exist

**Step 3: Write minimal implementation**

Add new method to `internal/slack/edge.go` after `GetActiveChannelsWithUsers`:

```go
// GetActiveChannelsWithResolver returns active channels with DM names resolved via UserResolver.
// This supports external Slack Connect users through cache and API fallback.
func (c *EdgeClient) GetActiveChannelsWithResolver(
	ctx context.Context,
	since time.Time,
	resolver *UserResolver,
) ([]Channel, error) {
	userBoot, err := c.ClientUserBoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("userBoot: %w", err)
	}

	counts, err := c.ClientCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("counts: %w", err)
	}

	timestamps := buildTimestampLookup(counts)

	var active []Channel

	// Process regular channels
	for _, ch := range userBoot.Channels {
		latest := timestamps[ch.ID]
		if !since.IsZero() && (latest.IsZero() || latest.Before(since)) {
			continue
		}
		active = append(active, Channel{
			ID:          ch.ID,
			Name:        ch.Name,
			IsIM:        ch.IsIM,
			IsPrivate:   ch.IsPrivate,
			LastMessage: latest,
		})
	}

	// Process DMs with resolver
	for _, im := range userBoot.IMs {
		latest := timestamps[im.ID]
		if !since.IsZero() && (latest.IsZero() || latest.Before(since)) {
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
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestEdgeClient_GetActiveChannelsWithResolver ./internal/slack/`
Expected: PASS

**Step 5: Run full test suite**

Run: `make check-test`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/slack/edge.go internal/slack/edge_test.go
git commit -m "feat(slack): add GetActiveChannelsWithResolver for external user support"
```

---

## Task 6: Add DefaultCachePath Helper

**Files:**
- Modify: `internal/slack/usercache.go`
- Test: `internal/slack/usercache_test.go`

**Step 1: Write the failing test**

Add to `internal/slack/usercache_test.go`:

```go
func TestDefaultCachePath(t *testing.T) {
	path := DefaultCachePath()

	// Should contain slack-export in path
	if !strings.Contains(path, "slack-export") {
		t.Errorf("expected slack-export in path, got %s", path)
	}

	// Should end with users.json
	if !strings.HasSuffix(path, "users.json") {
		t.Errorf("expected users.json suffix, got %s", path)
	}
}
```

Add `"strings"` to imports.

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestDefaultCachePath ./internal/slack/`
Expected: FAIL - function does not exist

**Step 3: Write minimal implementation**

Add to `internal/slack/usercache.go`:

```go
// DefaultCachePath returns the default path for the user cache.
// Uses XDG Base Directory spec: ~/.cache/slack-export/users.json
func DefaultCachePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "slack-export", "users.json")
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestDefaultCachePath ./internal/slack/`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/slack/usercache.go internal/slack/usercache_test.go
git commit -m "feat(slack): add DefaultCachePath for XDG-compliant cache location"
```

---

## Task 7: Wire Up UserResolver in runChannels

**Files:**
- Modify: `cmd/slack-export/main.go` (lines 296-304)

**Step 1: Update runChannels to use UserResolver**

Replace the user fetching and channel getting code (lines 296-304) with:

```go
	userIndex, err := client.FetchUsers(ctx)
	if err != nil {
		return fmt.Errorf("fetching users: %w", err)
	}

	// Set up external user cache
	cache := slack.NewUserCache(slack.DefaultCachePath())
	if err := cache.Load(); err != nil {
		return fmt.Errorf("loading user cache: %w", err)
	}

	resolver := slack.NewUserResolver(userIndex, cache, client)

	chans, err := client.GetActiveChannelsWithResolver(ctx, since, resolver)
	if err != nil {
		return fmt.Errorf("getting channels: %w", err)
	}

	// Save cache after successful fetch (may have new external users)
	if err := cache.Save(); err != nil {
		// Non-fatal: warn but continue
		fmt.Fprintf(os.Stderr, "Warning: failed to save user cache: %v\n", err)
	}
```

**Step 2: Run checks and tests**

Run: `make check-test`
Expected: All pass

**Step 3: Commit**

```bash
git add cmd/slack-export/main.go
git commit -m "feat(channels): wire up UserResolver for external user support"
```

---

## Task 8: Wire Up UserResolver in initStepVerify

**Files:**
- Modify: `cmd/slack-export/main.go` (find initStepVerify function)

**Step 1: Find and read initStepVerify**

Look for the function and update similar to runChannels.

**Step 2: Update initStepVerify to use UserResolver**

After fetching userIndex, add cache and resolver setup:

```go
	userIndex, err := client.FetchUsers(ctx)
	if err != nil {
		return fmt.Errorf("fetching users: %w", err)
	}

	// Set up external user cache
	cache := slack.NewUserCache(slack.DefaultCachePath())
	if err := cache.Load(); err != nil {
		return fmt.Errorf("loading user cache: %w", err)
	}

	resolver := slack.NewUserResolver(userIndex, cache, client)

	channels, err := client.GetActiveChannelsWithResolver(ctx, time.Time{}, resolver)
	if err != nil {
		return fmt.Errorf("getting channels: %w", err)
	}

	// Save cache after successful fetch
	if err := cache.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save user cache: %v\n", err)
	}
```

**Step 3: Run checks and tests**

Run: `make check-test`
Expected: All pass

**Step 4: Commit**

```bash
git add cmd/slack-export/main.go
git commit -m "feat(init): wire up UserResolver in verification step"
```

---

## Task 9: Wire Up UserResolver in Exporter.ExportDate

**Files:**
- Modify: `internal/export/exporter.go` (lines 87-95)

**Step 1: Update ExportDate to use UserResolver**

Replace the user fetching and channel getting code:

```go
	userIndex, err := e.edgeClient.FetchUsers(ctx)
	if err != nil {
		return fmt.Errorf("fetching users: %w", err)
	}

	// Set up external user cache
	cache := slack.NewUserCache(slack.DefaultCachePath())
	if err := cache.Load(); err != nil {
		return fmt.Errorf("loading user cache: %w", err)
	}

	resolver := slack.NewUserResolver(userIndex, cache, e.edgeClient)

	allChannels, err := e.edgeClient.GetActiveChannelsWithResolver(ctx, start, resolver)
	if err != nil {
		return fmt.Errorf("getting active channels: %w", err)
	}

	// Save cache after successful fetch
	if err := cache.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save user cache: %v\n", err)
	}
```

**Step 2: Run checks and tests**

Run: `make check-test`
Expected: All pass

**Step 3: Commit**

```bash
git add internal/export/exporter.go
git commit -m "feat(export): wire up UserResolver for external user support"
```

---

## Task 10: Integration Test

**Files:**
- Test: Manual testing

**Step 1: Build and run**

```bash
go build -o slack-export ./cmd/slack-export
./slack-export channels
```

**Step 2: Verify output**

Expected: DM channels with Slack Connect users should show `dm_username` not `dm_U123456`

**Step 3: Check cache file**

```bash
cat ~/.cache/slack-export/users.json
```

Expected: External users should be cached with their details

**Step 4: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "chore: cleanup after integration testing"
```

---

## Task 11: Delete debug_dst.go

**Files:**
- Delete: `debug_dst.go` (only after getting explicit approval)

**Step 1: Ask for approval to delete debug file**

The `debug_dst.go` file was a debugging script used during development. It's no longer needed now that `FetchUserInfo` is implemented properly.

**Step 2: After approval, delete**

```bash
rm debug_dst.go
git add debug_dst.go
git commit -m "chore: remove debug script now that FetchUserInfo is implemented"
```

---

## Summary

This plan implements external user resolution in 11 tasks:

1. Add `UserInfoResponse` type
2. Add `FetchUserInfo` method to EdgeClient
3. Create `UserCache` for persistent storage
4. Add `UserFetcher` interface and `UserResolver`
5. Add `GetActiveChannelsWithResolver` method
6. Add `DefaultCachePath` helper
7. Wire up in `runChannels`
8. Wire up in `initStepVerify`
9. Wire up in `Exporter.ExportDate`
10. Integration test
11. Cleanup debug file

Each task follows TDD: write failing test → implement → verify → commit.
