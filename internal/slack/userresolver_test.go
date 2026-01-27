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

func TestUserResolver_EmptyID(t *testing.T) {
	resolver := NewUserResolver(nil, nil, nil)

	name, err := resolver.Username(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "unknown" {
		t.Errorf("expected unknown for empty ID, got %s", name)
	}
}
