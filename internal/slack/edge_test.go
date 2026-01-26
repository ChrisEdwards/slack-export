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

func TestEdgeClient_ClientUserBoot_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ok": true,
			"self": {"id": "U123", "team_id": "T123", "name": "testuser"},
			"team": {"id": "T123", "name": "Test Team", "domain": "test"},
			"ims": [{"id": "D123", "user": "U456", "is_im": true, "is_open": true}],
			"channels": [
				{"id": "C123", "name": "general", "is_channel": true, "is_member": true, "created": 1609459200},
				{"id": "G456", "name": "private", "is_group": true, "is_private": true, "is_archived": true, "created": 1609459200}
			]
		}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	resp, err := client.ClientUserBoot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.OK {
		t.Error("expected OK to be true")
	}

	if resp.Self.ID != "U123" {
		t.Errorf("expected Self.ID U123, got %s", resp.Self.ID)
	}

	if resp.Team.Name != "Test Team" {
		t.Errorf("expected Team.Name Test Team, got %s", resp.Team.Name)
	}

	if len(resp.IMs) != 1 {
		t.Fatalf("expected 1 IM, got %d", len(resp.IMs))
	}

	if len(resp.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(resp.Channels))
	}

	if resp.Channels[0].Name != "general" {
		t.Errorf("expected first channel name general, got %s", resp.Channels[0].Name)
	}

	if !resp.Channels[1].IsArchived {
		t.Error("expected second channel to be archived")
	}
}

func TestEdgeClient_ClientUserBoot_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": false, "error": "invalid_auth"}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	_, err := client.ClientUserBoot(context.Background())
	if err == nil {
		t.Fatal("expected error for API error response")
	}

	if !strings.Contains(err.Error(), "invalid_auth") {
		t.Errorf("expected error to contain 'invalid_auth': %v", err)
	}
}

func TestEdgeClient_ClientUserBoot_ParseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	_, err := client.ClientUserBoot(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "parsing userBoot response") {
		t.Errorf("expected parsing error message: %v", err)
	}
}

func TestEdgeClient_ClientCounts_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ok": true,
			"channels": [
				{"id": "C123", "last_read": "1737676800.000000", "latest": "1737676900.123456", "mention_count": 5, "has_unreads": true},
				{"id": "C456", "last_read": "1737676500.000000", "latest": "1737676500.000000", "mention_count": 0, "has_unreads": false}
			],
			"mpims": [
				{"id": "G789", "last_read": "1737676000.000000", "latest": "1737676100.000000", "mention_count": 1, "has_unreads": true}
			],
			"ims": [
				{"id": "D111", "last_read": "1737675000.000000", "latest": "1737675500.000000", "mention_count": 0, "has_unreads": false}
			]
		}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	resp, err := client.ClientCounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.OK {
		t.Error("expected OK to be true")
	}

	if len(resp.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(resp.Channels))
	}

	if resp.Channels[0].Latest != "1737676900.123456" {
		t.Errorf("expected Latest 1737676900.123456, got %s", resp.Channels[0].Latest)
	}

	if resp.Channels[0].MentionCount != 5 {
		t.Errorf("expected MentionCount 5, got %d", resp.Channels[0].MentionCount)
	}

	if !resp.Channels[0].HasUnreads {
		t.Error("expected HasUnreads to be true")
	}

	if len(resp.MPIMs) != 1 {
		t.Fatalf("expected 1 MPIM, got %d", len(resp.MPIMs))
	}

	if len(resp.IMs) != 1 {
		t.Fatalf("expected 1 IM, got %d", len(resp.IMs))
	}
}

func TestEdgeClient_ClientCounts_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": false, "error": "not_authed"}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	_, err := client.ClientCounts(context.Background())
	if err == nil {
		t.Fatal("expected error for API error response")
	}

	if !strings.Contains(err.Error(), "not_authed") {
		t.Errorf("expected error to contain 'not_authed': %v", err)
	}
}

func TestEdgeClient_ClientCounts_ParseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	_, err := client.ClientCounts(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "parsing counts response") {
		t.Errorf("expected parsing error message: %v", err)
	}
}

func TestEdgeClient_ClientCounts_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	resp, err := client.ClientCounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.OK {
		t.Error("expected OK to be true")
	}

	if resp.Channels != nil {
		t.Error("expected Channels to be nil")
	}

	if resp.MPIMs != nil {
		t.Error("expected MPIMs to be nil")
	}

	if resp.IMs != nil {
		t.Error("expected IMs to be nil")
	}
}

func TestEdgeClient_ClientUserBoot_RequestFormat(t *testing.T) {
	var capturedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		capturedBody = string(bodyBytes)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true, "self": {}, "team": {}, "ims": [], "channels": []}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	_, err := client.ClientUserBoot(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	formValues, err := url.ParseQuery(capturedBody)
	if err != nil {
		t.Fatalf("failed to parse form body: %v", err)
	}

	if formValues.Get("include_permissions") != "1" {
		t.Errorf("expected include_permissions=1, got %s", formValues.Get("include_permissions"))
	}

	if formValues.Get("only_self_subteams") != "1" {
		t.Errorf("expected only_self_subteams=1, got %s", formValues.Get("only_self_subteams"))
	}
}

func TestEdgeClient_ClientCounts_RequestFormat(t *testing.T) {
	var capturedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		capturedBody = string(bodyBytes)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithBaseURL(server.URL)

	_, err := client.ClientCounts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	formValues, err := url.ParseQuery(capturedBody)
	if err != nil {
		t.Fatalf("failed to parse form body: %v", err)
	}

	if formValues.Get("thread_counts_by_channel") != "1" {
		t.Errorf("expected thread_counts_by_channel=1, got %s", formValues.Get("thread_counts_by_channel"))
	}

	if formValues.Get("org_wide_aware") != "1" {
		t.Errorf("expected org_wide_aware=1, got %s", formValues.Get("org_wide_aware"))
	}

	if formValues.Get("include_file_channels") != "1" {
		t.Errorf("expected include_file_channels=1, got %s", formValues.Get("include_file_channels"))
	}
}
