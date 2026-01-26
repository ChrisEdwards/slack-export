package slack

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestEdgeClient_Post_Success(t *testing.T) {
	var capturedRequest *http.Request
	var capturedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequest = r
		bodyBytes, _ := io.ReadAll(r.Body)
		capturedBody = string(bodyBytes)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
		Cookies: []*http.Cookie{
			{Name: "d", Value: "test-cookie-value"},
		},
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	body := map[string]any{
		"key1": "value1",
		"key2": 42,
	}

	result, err := client.post(context.Background(), "client.userBoot", body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify response
	if string(result) != `{"ok":true}` {
		t.Errorf("unexpected response: %s", result)
	}

	// Verify URL path
	expectedPath := "/cache/T12345/client.userBoot"
	if capturedRequest.URL.Path != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, capturedRequest.URL.Path)
	}

	// Verify method
	if capturedRequest.Method != http.MethodPost {
		t.Errorf("expected POST method, got %s", capturedRequest.Method)
	}

	// Verify Content-Type header
	contentType := capturedRequest.Header.Get("Content-Type")
	if contentType != "application/x-www-form-urlencoded" {
		t.Errorf("expected Content-Type application/x-www-form-urlencoded, got %s", contentType)
	}

	// Verify form body contains token
	formValues, err := url.ParseQuery(capturedBody)
	if err != nil {
		t.Fatalf("failed to parse form body: %v", err)
	}

	if formValues.Get("token") != "xoxc-test-token" {
		t.Errorf("expected token xoxc-test-token, got %s", formValues.Get("token"))
	}

	if formValues.Get("key1") != "value1" {
		t.Errorf("expected key1=value1, got %s", formValues.Get("key1"))
	}

	if formValues.Get("key2") != "42" {
		t.Errorf("expected key2=42, got %s", formValues.Get("key2"))
	}

	// Verify cookies
	cookies := capturedRequest.Cookies()
	var foundCookie bool
	for _, c := range cookies {
		if c.Name == "d" && c.Value == "test-cookie-value" {
			foundCookie = true
			break
		}
	}
	if !foundCookie {
		t.Error("expected d cookie not found in request")
	}
}

func TestEdgeClient_Post_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_auth"}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	_, err := client.post(context.Background(), "client.userBoot", nil)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}

	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to contain status code 401: %v", err)
	}

	if !strings.Contains(err.Error(), "invalid_auth") {
		t.Errorf("expected error to contain response body: %v", err)
	}

	// Verify error message follows Go conventions (lowercase)
	if !strings.Contains(err.Error(), "edge API error") {
		t.Errorf("expected lowercase error message 'edge API error': %v", err)
	}
}

func TestEdgeClient_Post_NetworkError(t *testing.T) {
	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	// Use a non-existent server URL
	client := NewEdgeClient(creds).WithBaseURL("http://localhost:0")

	_, err := client.post(context.Background(), "client.userBoot", nil)
	if err == nil {
		t.Fatal("expected network error")
	}

	if !strings.Contains(err.Error(), "sending request") {
		t.Errorf("expected 'sending request' error prefix: %v", err)
	}
}

func TestEdgeClient_Post_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.post(ctx, "client.userBoot", nil)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestEdgeClient_Post_MultipleCookies(t *testing.T) {
	var capturedCookies []*http.Cookie

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCookies = r.Cookies()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
		Cookies: []*http.Cookie{
			{Name: "d", Value: "cookie-d"},
			{Name: "d-s", Value: "cookie-d-s"},
			{Name: "lc", Value: "cookie-lc"},
		},
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	_, err := client.post(context.Background(), "test.endpoint", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all cookies were sent
	cookieMap := make(map[string]string)
	for _, c := range capturedCookies {
		cookieMap[c.Name] = c.Value
	}

	expectedCookies := map[string]string{
		"d":   "cookie-d",
		"d-s": "cookie-d-s",
		"lc":  "cookie-lc",
	}

	for name, expected := range expectedCookies {
		if actual, ok := cookieMap[name]; !ok {
			t.Errorf("cookie %q not found", name)
		} else if actual != expected {
			t.Errorf("cookie %q: expected %q, got %q", name, expected, actual)
		}
	}
}

func TestEdgeClient_Post_EmptyBody(t *testing.T) {
	var capturedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		capturedBody = string(bodyBytes)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	_, err := client.post(context.Background(), "test.endpoint", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still contain the token even with no additional body
	formValues, err := url.ParseQuery(capturedBody)
	if err != nil {
		t.Fatalf("failed to parse form body: %v", err)
	}

	if formValues.Get("token") != "xoxc-test-token" {
		t.Errorf("expected token in form body, got: %s", capturedBody)
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"int_negative", -10, "-10"},
		{"int64", int64(1234567890123), "1234567890123"},
		{"float64", 3.14, "3.14"},
		{"float64_whole", float64(10), "10"},
		{"bool_true", true, "1"},
		{"bool_false", false, "0"},
		{"uint", uint(100), "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.input)
			if result != tt.expected {
				t.Errorf("formatValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
