package slack

import (
	"net/http"
	"testing"
	"time"
)

func TestNewEdgeClient(t *testing.T) {
	creds := &Credentials{
		Token:     "xoxc-123-456-789",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds)

	if client.creds != creds {
		t.Error("expected credentials to be set")
	}

	if client.httpClient == nil {
		t.Error("expected HTTP client to be initialized")
	}

	if client.httpClient.Timeout != DefaultHTTPTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultHTTPTimeout, client.httpClient.Timeout)
	}

	if client.baseURL != DefaultEdgeBaseURL {
		t.Errorf("expected baseURL %q, got %q", DefaultEdgeBaseURL, client.baseURL)
	}
}

func TestEdgeClient_WithBaseURL(t *testing.T) {
	creds := &Credentials{
		Token:  "xoxc-123-456-789",
		TeamID: "T12345",
	}

	original := NewEdgeClient(creds)
	customURL := "http://localhost:8080"

	modified := original.WithBaseURL(customURL)

	// Verify new client has custom URL
	if modified.baseURL != customURL {
		t.Errorf("expected baseURL %q, got %q", customURL, modified.baseURL)
	}

	// Verify original is unchanged (immutability)
	if original.baseURL != DefaultEdgeBaseURL {
		t.Errorf("original baseURL was modified: got %q", original.baseURL)
	}

	// Verify credentials are shared
	if modified.creds != original.creds {
		t.Error("expected credentials to be shared")
	}

	// Verify HTTP client is shared
	if modified.httpClient != original.httpClient {
		t.Error("expected HTTP client to be shared")
	}
}

func TestEdgeClient_WithHTTPClient(t *testing.T) {
	creds := &Credentials{
		Token:  "xoxc-123-456-789",
		TeamID: "T12345",
	}

	original := NewEdgeClient(creds)
	customClient := &http.Client{Timeout: 60 * time.Second}

	modified := original.WithHTTPClient(customClient)

	// Verify new client has custom HTTP client
	if modified.httpClient != customClient {
		t.Error("expected custom HTTP client to be set")
	}

	if modified.httpClient.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s, got %v", modified.httpClient.Timeout)
	}

	// Verify original is unchanged
	if original.httpClient.Timeout != DefaultHTTPTimeout {
		t.Errorf("original HTTP client was modified: got %v", original.httpClient.Timeout)
	}

	// Verify baseURL is preserved
	if modified.baseURL != original.baseURL {
		t.Errorf("expected baseURL to be preserved: got %q", modified.baseURL)
	}
}

func TestEdgeClient_Chaining(t *testing.T) {
	creds := &Credentials{
		Token:  "xoxc-123-456-789",
		TeamID: "T12345",
	}

	customURL := "http://test-server:9000"
	customClient := &http.Client{Timeout: 5 * time.Second}

	client := NewEdgeClient(creds).
		WithBaseURL(customURL).
		WithHTTPClient(customClient)

	if client.baseURL != customURL {
		t.Errorf("expected baseURL %q, got %q", customURL, client.baseURL)
	}

	if client.httpClient != customClient {
		t.Error("expected custom HTTP client")
	}

	if client.creds != creds {
		t.Error("expected credentials to be preserved")
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultEdgeBaseURL != "https://edgeapi.slack.com" {
		t.Errorf("unexpected DefaultEdgeBaseURL: %s", DefaultEdgeBaseURL)
	}

	if DefaultHTTPTimeout != 30*time.Second {
		t.Errorf("unexpected DefaultHTTPTimeout: %v", DefaultHTTPTimeout)
	}
}
