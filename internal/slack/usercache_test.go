package slack

import (
	"os"
	"path/filepath"
	"testing"
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
