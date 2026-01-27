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
	if err := os.MkdirAll(filepath.Dir(c.path), 0700); err != nil {
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

	return os.WriteFile(c.path, data, 0600)
}
