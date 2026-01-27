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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	// Verify URL path (now uses workspace API, not edge API)
	expectedPath := "/api/client.userBoot"
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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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
	client := NewEdgeClient(creds).WithWorkspaceURL("http://localhost:0/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

func TestParseSlackTS(t *testing.T) {
	tests := []struct {
		name        string
		ts          string
		wantUnix    int64
		wantNsec    int64
		wantZero    bool
		wantErr     bool
		errContains string
	}{
		{
			name:     "full timestamp with microseconds",
			ts:       "1737676800.123456",
			wantUnix: 1737676800,
			wantNsec: 123456000, // 123456 microseconds = 123456000 nanoseconds
		},
		{
			name:     "timestamp without decimal",
			ts:       "1737676800",
			wantUnix: 1737676800,
			wantNsec: 0,
		},
		{
			name:     "timestamp with empty decimal",
			ts:       "1737676800.",
			wantUnix: 1737676800,
			wantNsec: 0,
		},
		{
			name:     "timestamp with short microseconds (padded)",
			ts:       "1737676800.123",
			wantUnix: 1737676800,
			wantNsec: 123000000, // "123" padded to "123000" = 123000 microseconds = 123000000 nanoseconds
		},
		{
			name:     "timestamp with single digit microseconds",
			ts:       "1737676800.1",
			wantUnix: 1737676800,
			wantNsec: 100000000, // "1" padded to "100000" = 100000 microseconds
		},
		{
			name:     "timestamp with long microseconds (truncated)",
			ts:       "1737676800.123456789",
			wantUnix: 1737676800,
			wantNsec: 123456000, // truncated to 6 digits
		},
		{
			name:     "zero timestamp",
			ts:       "0.000000",
			wantUnix: 0,
			wantNsec: 0,
		},
		{
			name:     "empty string returns zero time",
			ts:       "",
			wantZero: true,
		},
		{
			name:        "invalid seconds",
			ts:          "not_a_number.123456",
			wantErr:     true,
			errContains: "parsing seconds",
		},
		{
			name:        "invalid microseconds",
			ts:          "1737676800.abc",
			wantErr:     true,
			errContains: "parsing microseconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSlackTS(tt.ts)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantZero {
				if !got.IsZero() {
					t.Errorf("expected zero time, got %v", got)
				}
				return
			}

			if got.Unix() != tt.wantUnix {
				t.Errorf("Unix() = %d, want %d", got.Unix(), tt.wantUnix)
			}

			// Check nanoseconds component
			gotNsec := got.UnixNano() - (got.Unix() * 1e9)
			if gotNsec != tt.wantNsec {
				t.Errorf("nanoseconds = %d, want %d", gotNsec, tt.wantNsec)
			}
		})
	}
}

func TestParseSlackTS_RoundTrip(t *testing.T) {
	// Test that typical Slack timestamps parse correctly
	ts := "1737676900.123456"
	got, err := ParseSlackTS(ts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// January 24, 2025 in UTC
	expected := time.Date(2025, 1, 24, 0, 1, 40, 123456000, time.UTC)

	if !got.Equal(expected) {
		t.Errorf("ParseSlackTS(%q) = %v, want %v", ts, got, expected)
	}
}

func TestEdgeClient_GetActiveChannels_Success(t *testing.T) {
	// Create test server that responds to both endpoints
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T123", "name": "testuser"},
				"team": {"id": "T123", "name": "Test Team", "domain": "test"},
				"ims": [
					{"id": "D123", "user": "U456", "is_im": true, "is_open": true, "latest": "1737676800.000000"},
					{"id": "D789", "user": "U999", "is_im": true, "is_open": true, "latest": "1737500000.000000"}
				],
				"channels": [
					{"id": "C001", "name": "active-channel", "is_channel": true, "is_member": true, "created": 1609459200},
					{"id": "C002", "name": "old-channel", "is_channel": true, "is_member": true, "created": 1609459200},
					{"id": "G003", "name": "private-group", "is_group": true, "is_private": true, "is_member": true, "created": 1609459200}
				]
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"channels": [
					{"id": "C001", "latest": "1737676900.123456"},
					{"id": "C002", "latest": "1737500000.000000"},
					{"id": "G003", "latest": "1737676950.000000"}
				],
				"ims": [
					{"id": "D123", "latest": "1737676800.000000"},
					{"id": "D789", "latest": "1737500000.000000"}
				]
			}`))
		}
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

	// Filter to channels active after Jan 24, 2025 00:00:00 UTC
	since := time.Date(2025, 1, 24, 0, 0, 0, 0, time.UTC)

	channels, err := client.GetActiveChannels(context.Background(), since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 channels: C001 (active-channel), G003 (private-group), D123 (dm)
	// C002 and D789 should be filtered out (too old)
	if len(channels) != 3 {
		t.Fatalf("expected 3 active channels, got %d", len(channels))
	}

	// Verify C001 is included
	foundC001 := false
	for _, ch := range channels {
		if ch.ID == "C001" {
			foundC001 = true
			if ch.Name != "active-channel" {
				t.Errorf("C001 name: expected active-channel, got %s", ch.Name)
			}
			if !ch.IsChannel {
				t.Error("C001 should be a channel")
			}
			if !ch.IsMember {
				t.Error("C001 should have IsMember true")
			}
		}
	}
	if !foundC001 {
		t.Error("C001 (active-channel) should be in results")
	}

	// Verify G003 is included
	foundG003 := false
	for _, ch := range channels {
		if ch.ID == "G003" {
			foundG003 = true
			if !ch.IsGroup {
				t.Error("G003 should be a group")
			}
			if !ch.IsPrivate {
				t.Error("G003 should be private")
			}
		}
	}
	if !foundG003 {
		t.Error("G003 (private-group) should be in results")
	}

	// Verify D123 DM is included
	foundD123 := false
	for _, ch := range channels {
		if ch.ID == "D123" {
			foundD123 = true
			if !ch.IsIM {
				t.Error("D123 should be an IM")
			}
			if ch.Name != "dm_U456" {
				t.Errorf("D123 name: expected dm_U456, got %s", ch.Name)
			}
		}
	}
	if !foundD123 {
		t.Error("D123 (dm) should be in results")
	}

	// Verify C002 is NOT included (too old)
	for _, ch := range channels {
		if ch.ID == "C002" {
			t.Error("C002 (old-channel) should NOT be in results")
		}
	}
}

func TestEdgeClient_GetActiveChannels_ZeroSince(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T123", "name": "testuser"},
				"team": {"id": "T123", "name": "Test Team", "domain": "test"},
				"ims": [{"id": "D123", "user": "U456", "is_im": true}],
				"channels": [
					{"id": "C001", "name": "channel-1", "is_channel": true, "created": 1609459200},
					{"id": "C002", "name": "channel-2", "is_channel": true, "created": 1609459200}
				]
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"channels": [
					{"id": "C001", "latest": "1737676900.123456"},
					{"id": "C002", "latest": ""}
				],
				"ims": [{"id": "D123", "latest": "1737676800.000000"}]
			}`))
		}
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

	// Zero since time should return all channels
	channels, err := client.GetActiveChannels(context.Background(), time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have all 3 channels (C001, C002, D123)
	if len(channels) != 3 {
		t.Fatalf("expected 3 channels with zero since, got %d", len(channels))
	}
}

func TestEdgeClient_GetActiveChannels_UserBootError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok": false, "error": "invalid_auth"}`))
		}
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

	_, err := client.GetActiveChannels(context.Background(), time.Now())
	if err == nil {
		t.Fatal("expected error from userBoot failure")
	}

	if !strings.Contains(err.Error(), "userBoot") {
		t.Errorf("error should mention userBoot: %v", err)
	}
}

func TestEdgeClient_GetActiveChannels_CountsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T123", "name": "testuser"},
				"team": {"id": "T123", "name": "Test Team", "domain": "test"},
				"ims": [],
				"channels": []
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok": false, "error": "rate_limited"}`))
		}
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

	_, err := client.GetActiveChannels(context.Background(), time.Now())
	if err == nil {
		t.Fatal("expected error from counts failure")
	}

	if !strings.Contains(err.Error(), "counts") {
		t.Errorf("error should mention counts: %v", err)
	}
}

func TestEdgeClient_GetActiveChannels_EmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T123", "name": "testuser"},
				"team": {"id": "T123", "name": "Test Team", "domain": "test"},
				"ims": [],
				"channels": []
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok": true}`))
		}
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

	channels, err := client.GetActiveChannels(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(channels) != 0 {
		t.Errorf("expected empty results, got %d channels", len(channels))
	}
}

func TestEdgeClient_GetActiveChannels_MPIMs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T123", "name": "testuser"},
				"team": {"id": "T123", "name": "Test Team", "domain": "test"},
				"ims": [],
				"channels": [
					{"id": "G001", "name": "mpim-group", "is_mpim": true, "is_group": true, "created": 1609459200}
				]
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"mpims": [{"id": "G001", "latest": "1737676900.000000"}]
			}`))
		}
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

	// Filter to recent activity
	since := time.Date(2025, 1, 24, 0, 0, 0, 0, time.UTC)

	channels, err := client.GetActiveChannels(context.Background(), since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(channels) != 1 {
		t.Fatalf("expected 1 MPIM channel, got %d", len(channels))
	}

	if !channels[0].IsMPIM {
		t.Error("expected channel to have IsMPIM=true")
	}
}

func TestBuildTimestampLookup(t *testing.T) {
	counts := &CountsResponse{
		OK: true,
		Channels: []ChannelSnapshot{
			{ID: "C001", Latest: "1737676900.123456"},
			{ID: "C002", Latest: "1737676800.000000"},
			{ID: "C003", Latest: ""}, // Empty latest
		},
		IMs: []ChannelSnapshot{
			{ID: "D001", Latest: "1737676700.000000"},
		},
		MPIMs: []ChannelSnapshot{
			{ID: "G001", Latest: "1737676600.000000"},
		},
	}

	lookup := buildTimestampLookup(counts)

	// Should have 4 entries (C003 excluded due to empty latest)
	if len(lookup) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(lookup))
	}

	// Check C001
	if _, ok := lookup["C001"]; !ok {
		t.Error("C001 should be in lookup")
	}

	// Check D001
	if _, ok := lookup["D001"]; !ok {
		t.Error("D001 should be in lookup")
	}

	// Check G001
	if _, ok := lookup["G001"]; !ok {
		t.Error("G001 should be in lookup")
	}

	// C003 should NOT be in lookup (empty latest)
	if _, ok := lookup["C003"]; ok {
		t.Error("C003 should NOT be in lookup (empty latest)")
	}
}

func TestBuildTimestampLookup_EmptyCounts(t *testing.T) {
	counts := &CountsResponse{OK: true}

	lookup := buildTimestampLookup(counts)

	if len(lookup) != 0 {
		t.Errorf("expected empty lookup, got %d entries", len(lookup))
	}
}

func TestEdgeClient_AuthTest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth.test" {
			t.Errorf("expected path /auth.test, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "token=xoxc-test-token") {
			t.Errorf("expected token in body, got %s", string(body))
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ok": true,
			"url": "https://test-team.slack.com/",
			"team": "Test Team",
			"user": "testuser",
			"team_id": "T12345678",
			"user_id": "U12345678"
		}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithSlackAPIURL(server.URL)

	resp, err := client.AuthTest(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.OK {
		t.Error("expected OK to be true")
	}

	if resp.TeamID != "T12345678" {
		t.Errorf("expected TeamID T12345678, got %s", resp.TeamID)
	}

	if resp.UserID != "U12345678" {
		t.Errorf("expected UserID U12345678, got %s", resp.UserID)
	}

	if resp.Team != "Test Team" {
		t.Errorf("expected Team 'Test Team', got %s", resp.Team)
	}

	// Verify that creds.TeamID was set
	if creds.TeamID != "T12345678" {
		t.Errorf("expected creds.TeamID to be set to T12345678, got %s", creds.TeamID)
	}
}

func TestEdgeClient_AuthTest_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`invalid_auth`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithSlackAPIURL(server.URL)

	_, err := client.AuthTest(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}

	if !strings.Contains(err.Error(), "auth.test API error 401") {
		t.Errorf("expected auth.test API error, got: %v", err)
	}
}

func TestEdgeClient_AuthTest_SlackError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"ok": false,
			"error": "invalid_auth"
		}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		Workspace: "test-workspace",
	}

	client := NewEdgeClient(creds).WithSlackAPIURL(server.URL)

	_, err := client.AuthTest(context.Background())
	if err == nil {
		t.Fatal("expected error for Slack API error")
	}

	if !strings.Contains(err.Error(), "auth.test failed: invalid_auth") {
		t.Errorf("expected auth.test failed error, got: %v", err)
	}
}

func TestEdgeClient_AuthTest_WithCookies(t *testing.T) {
	var receivedCookies []*http.Cookie

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookies = r.Cookies()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok": true, "team_id": "T123", "user_id": "U123"}`))
	}))
	defer server.Close()

	creds := &Credentials{
		Token:     "xoxc-test-token",
		Workspace: "test-workspace",
		Cookies: []*http.Cookie{
			{Name: "d", Value: "xoxd-test"},
			{Name: "ds", Value: "12345"},
		},
	}

	client := NewEdgeClient(creds).WithSlackAPIURL(server.URL)

	_, err := client.AuthTest(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(receivedCookies) != 2 {
		t.Errorf("expected 2 cookies, got %d", len(receivedCookies))
	}

	foundD := false
	for _, c := range receivedCookies {
		if c.Name == "d" && c.Value == "xoxd-test" {
			foundD = true
		}
	}
	if !foundD {
		t.Error("expected 'd' cookie to be sent")
	}
}

func TestEdgeClient_WithSlackAPIURL(t *testing.T) {
	creds := &Credentials{
		Token:  "xoxc-123-456-789",
		TeamID: "T12345",
	}

	original := NewEdgeClient(creds)
	customURL := "http://localhost:8080"

	modified := original.WithSlackAPIURL(customURL)

	if modified.slackAPIURL != customURL {
		t.Errorf("expected slackAPIURL %q, got %q", customURL, modified.slackAPIURL)
	}

	if original.slackAPIURL != DefaultSlackAPIURL {
		t.Errorf("original slackAPIURL was modified: got %q", original.slackAPIURL)
	}

	// Ensure other fields are preserved
	if modified.baseURL != original.baseURL {
		t.Errorf("expected baseURL to be preserved, got %q", modified.baseURL)
	}
}

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

func TestUserIndex_Username(t *testing.T) {
	users := []User{
		{ID: "U001", Name: "alice.smith", RealName: "Alice Smith", Profile: UserProfile{DisplayName: "Alice"}},
		{ID: "U002", Name: "Bob.Jones", RealName: "Bob Jones", Profile: UserProfile{}},
		{ID: "U003", Name: "", RealName: "Carol", Profile: UserProfile{}},
	}
	idx := NewUserIndex(users)

	tests := []struct {
		name     string
		userID   string
		expected string
	}{
		{"returns lowercase username", "U001", "alice.smith"},
		{"converts uppercase to lowercase", "U002", "bob.jones"},
		{"falls back to ID when name empty", "U003", "U003"},
		{"falls back to ID for unknown user", "U999", "U999"},
		{"empty user ID returns unknown", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := idx.Username(tt.userID)
			if got != tt.expected {
				t.Errorf("Username(%q) = %q, want %q", tt.userID, got, tt.expected)
			}
		})
	}
}

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

	if nameByID["D001"] != "dm_alice" {
		t.Errorf("D001: expected dm_alice, got %s", nameByID["D001"])
	}

	if nameByID["D002"] != "dm_bob" {
		t.Errorf("D002: expected dm_bob, got %s", nameByID["D002"])
	}

	if nameByID["D003"] != "dm_U999" {
		t.Errorf("D003: expected dm_U999, got %s", nameByID["D003"])
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
		name     string
		userID   string
		index    UserIndex
		expected string
	}{
		{"with index and known user", "U001", userIndex, "dm_alice"},
		{"with index and unknown user", "U999", userIndex, "dm_U999"},
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

func TestEdgeClient_GetActiveChannelsWithResolver_ExternalUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
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
		if strings.HasSuffix(r.URL.Path, "/client.counts") {
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
	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

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

	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}

	// Should resolve to username, not raw ID
	if channels[0].Name != "dm_external.user" {
		t.Errorf("expected dm_external.user, got %s", channels[0].Name)
	}
}

func TestEdgeClient_GetActiveChannelsWithResolver_NilResolver(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {}, "team": {},
				"ims": [{"id": "D001", "user": "U456", "is_im": true}],
				"channels": []
			}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/client.counts") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"ims": [{"id": "D001", "latest": "1700000000.000000"}]
			}`))
			return
		}
	}))
	defer server.Close()

	creds := &Credentials{Token: "xoxc-test"}
	client := NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

	channels, err := client.GetActiveChannelsWithResolver(context.Background(), time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}

	// Falls back to user ID when resolver is nil
	if channels[0].Name != "dm_U456" {
		t.Errorf("expected dm_U456, got %s", channels[0].Name)
	}
}
