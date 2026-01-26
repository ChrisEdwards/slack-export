package export

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chrisedwards/slack-export/internal/config"
	"github.com/chrisedwards/slack-export/internal/slack"
)

func TestExporter_Config(t *testing.T) {
	cfg := &config.Config{
		OutputDir: "/test/output",
		Timezone:  "America/New_York",
	}

	e := &Exporter{cfg: cfg}

	if e.Config() != cfg {
		t.Error("Config() should return the configuration")
	}
}

func TestExporter_EdgeClient(t *testing.T) {
	creds := &slack.Credentials{
		Token:     "xoxc-test",
		TeamID:    "T123",
		Workspace: "test",
	}
	client := slack.NewEdgeClient(creds)

	e := &Exporter{edgeClient: client}

	if e.EdgeClient() != client {
		t.Error("EdgeClient() should return the edge client")
	}
}

func TestExporter_SlackdumpPath(t *testing.T) {
	e := &Exporter{slackdump: "/usr/local/bin/slackdump"}

	if e.SlackdumpPath() != "/usr/local/bin/slackdump" {
		t.Errorf("SlackdumpPath() = %q, want /usr/local/bin/slackdump", e.SlackdumpPath())
	}
}

func TestExporter_Credentials(t *testing.T) {
	creds := &slack.Credentials{
		Token:     "xoxc-test",
		TeamID:    "T123",
		Workspace: "test",
	}

	e := &Exporter{creds: creds}

	if e.Credentials() != creds {
		t.Error("Credentials() should return the credentials")
	}
}

func TestNewExporter_SlackdumpNotFound(t *testing.T) {
	// Set PATH to empty dir so slackdump won't be found
	tmpDir := t.TempDir()
	t.Setenv("PATH", tmpDir)

	cfg := &config.Config{
		OutputDir:     tmpDir,
		Timezone:      "America/New_York",
		SlackdumpPath: "", // Use PATH lookup
	}

	// This will fail at slackdump lookup before ever hitting credentials
	// But credentials check happens first, so we need to mock that path
	// For now, just verify that a missing slackdump is properly reported

	// Since LoadCredentials() will fail first (no slackdump cache),
	// we need to verify the error chain

	_, err := NewExporter(cfg)
	if err == nil {
		t.Fatal("NewExporter() should fail when credentials/slackdump unavailable")
	}
}

func TestNewExporter_InvalidSlackdumpPath(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		OutputDir:     tmpDir,
		Timezone:      "America/New_York",
		SlackdumpPath: "/nonexistent/slackdump", // Explicit bad path
	}

	_, err := NewExporter(cfg)
	if err == nil {
		t.Fatal("NewExporter() should fail with invalid slackdump path")
	}

	// The error will be about credentials first (since that check happens before slackdump)
	// unless credentials exist. Without mocking, we can't test the slackdump path error directly.
}

// TestNewExporterWithOptions tests the Exporter using direct construction
// to simulate how it would work with valid dependencies.
func TestNewExporterWithOptions(t *testing.T) {
	// Create a mock Edge API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T999", "name": "testuser"},
				"team": {"id": "T999", "name": "Test Team", "domain": "test"},
				"ims": [],
				"channels": []
			}`))
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()

	// Create a fake slackdump binary
	fakeBin := filepath.Join(tmpDir, "slackdump")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho test"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		OutputDir:     filepath.Join(tmpDir, "output"),
		Timezone:      "America/New_York",
		SlackdumpPath: fakeBin,
	}

	creds := &slack.Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	edgeClient := slack.NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

	// Manually construct an Exporter to test the struct
	e := &Exporter{
		cfg:        cfg,
		edgeClient: edgeClient,
		slackdump:  fakeBin,
		creds:      creds,
	}

	// Verify all getters work correctly
	if e.Config() != cfg {
		t.Error("Config() mismatch")
	}
	if e.EdgeClient() != edgeClient {
		t.Error("EdgeClient() mismatch")
	}
	if e.SlackdumpPath() != fakeBin {
		t.Error("SlackdumpPath() mismatch")
	}
	if e.Credentials() != creds {
		t.Error("Credentials() mismatch")
	}
}

// TestExporterIntegration_WithMockDependencies demonstrates how to construct
// an Exporter for testing purposes without actually calling NewExporter.
func TestExporterIntegration_WithMockDependencies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T999"},
				"team": {"id": "T999", "name": "Test"},
				"channels": [{"id": "C001", "name": "general", "is_channel": true}],
				"ims": []
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"channels": [{"id": "C001", "latest": "1737676900.123456"}]
			}`))
		}
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "slackdump")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\necho test"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		OutputDir:     filepath.Join(tmpDir, "output"),
		Timezone:      "America/New_York",
		SlackdumpPath: fakeBin,
	}

	creds := &slack.Credentials{
		Token:     "xoxc-test-token",
		TeamID:    "T12345",
		Workspace: "test-workspace",
	}

	edgeClient := slack.NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/")

	e := &Exporter{
		cfg:        cfg,
		edgeClient: edgeClient,
		slackdump:  fakeBin,
		creds:      creds,
	}

	// Test that EdgeClient works through the Exporter
	channels, err := e.EdgeClient().GetActiveChannels(context.Background(), time.Time{})
	if err != nil {
		t.Fatalf("GetActiveChannels() error: %v", err)
	}

	if len(channels) != 1 {
		t.Errorf("expected 1 channel, got %d", len(channels))
	}
}

func TestExportDate_InvalidDate(t *testing.T) {
	e := &Exporter{
		cfg: &config.Config{Timezone: "America/New_York"},
	}

	err := e.ExportDate(context.Background(), "not-a-date")
	if err == nil {
		t.Error("ExportDate() should fail with invalid date")
	}
	if !strings.Contains(err.Error(), "calculating date bounds") {
		t.Errorf("error should mention date bounds: %v", err)
	}
}

func TestExportDate_InvalidTimezone(t *testing.T) {
	e := &Exporter{
		cfg: &config.Config{Timezone: "Invalid/Timezone"},
	}

	err := e.ExportDate(context.Background(), "2026-01-22")
	if err == nil {
		t.Error("ExportDate() should fail with invalid timezone")
	}
	if !strings.Contains(err.Error(), "calculating date bounds") {
		t.Errorf("error should mention date bounds: %v", err)
	}
}

func TestExportDate_NoActiveChannels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/users.list" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"members": [],
				"response_metadata": {"next_cursor": ""}
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T999"},
				"team": {"id": "T999", "name": "Test"},
				"channels": [],
				"ims": []
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			_, _ = w.Write([]byte(`{"ok": true, "channels": []}`))
		}
	}))
	defer server.Close()

	creds := &slack.Credentials{Token: "xoxc-test", TeamID: "T999"}
	e := &Exporter{
		cfg:        &config.Config{Timezone: "America/New_York"},
		edgeClient: slack.NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/").WithSlackAPIURL(server.URL),
	}

	err := e.ExportDate(context.Background(), "2026-01-22")
	if err != nil {
		t.Errorf("ExportDate() should succeed with no active channels: %v", err)
	}
}

func TestExportDate_AllChannelsFilteredOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/users.list" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"members": [],
				"response_metadata": {"next_cursor": ""}
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T999"},
				"team": {"id": "T999", "name": "Test"},
				"channels": [{"id": "C001", "name": "general", "is_channel": true}],
				"ims": []
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"channels": [{"id": "C001", "latest": "1737676900.123456"}]
			}`))
		}
	}))
	defer server.Close()

	creds := &slack.Credentials{Token: "xoxc-test", TeamID: "T999"}
	e := &Exporter{
		cfg: &config.Config{
			Timezone: "America/New_York",
			Exclude:  []string{"general"}, // Exclude all channels
		},
		edgeClient: slack.NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/").WithSlackAPIURL(server.URL),
	}

	err := e.ExportDate(context.Background(), "2026-01-22")
	if err != nil {
		t.Errorf("ExportDate() should succeed when all filtered out: %v", err)
	}
}

func TestExportDate_EdgeAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users.list" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"members": [],
				"response_metadata": {"next_cursor": ""}
			}`))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"ok": false, "error": "server_error"}`))
		}
	}))
	defer server.Close()

	creds := &slack.Credentials{Token: "xoxc-test", TeamID: "T999"}
	e := &Exporter{
		cfg:        &config.Config{Timezone: "America/New_York"},
		edgeClient: slack.NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/").WithSlackAPIURL(server.URL),
	}

	err := e.ExportDate(context.Background(), "2026-01-22")
	if err == nil {
		t.Error("ExportDate() should fail on Edge API error")
	}
	if !strings.Contains(err.Error(), "getting active channels") {
		t.Errorf("error should mention getting channels: %v", err)
	}
}

func TestBuildChannelMaps(t *testing.T) {
	chans := []slack.Channel{
		{ID: "C001", Name: "general"},
		{ID: "C002", Name: "random"},
		{ID: "D001", Name: "dm_bob"},
	}

	ids, names := buildChannelMaps(chans)

	if len(ids) != 3 {
		t.Errorf("expected 3 ids, got %d", len(ids))
	}

	expectedIDs := []string{"C001", "C002", "D001"}
	for i, id := range ids {
		if id != expectedIDs[i] {
			t.Errorf("ids[%d] = %q, want %q", i, id, expectedIDs[i])
		}
	}

	if names["C001"] != "general" {
		t.Errorf("names[C001] = %q, want general", names["C001"])
	}
	if names["C002"] != "random" {
		t.Errorf("names[C002] = %q, want random", names["C002"])
	}
	if names["D001"] != "dm_bob" {
		t.Errorf("names[D001] = %q, want dm_bob", names["D001"])
	}
}

func TestBuildChannelMaps_Empty(t *testing.T) {
	ids, names := buildChannelMaps(nil)

	if len(ids) != 0 {
		t.Errorf("expected empty ids, got %d", len(ids))
	}
	if len(names) != 0 {
		t.Errorf("expected empty names, got %d", len(names))
	}
}

func TestCleanupTempDir(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "slackdump_20260122")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatal(err)
	}

	cleanupTempDir(subDir)

	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		t.Error("cleanupTempDir should remove parent directory")
	}
}

func TestCleanupTempDir_EmptyPath(t *testing.T) {
	cleanupTempDir("")
}

func TestExportRange_InvalidTimezone(t *testing.T) {
	e := &Exporter{
		cfg: &config.Config{Timezone: "Invalid/Timezone"},
	}

	err := e.ExportRange(context.Background(), "2026-01-22", "2026-01-24")
	if err == nil {
		t.Error("ExportRange() should fail with invalid timezone")
	}
	if !strings.Contains(err.Error(), "loading timezone") {
		t.Errorf("error should mention loading timezone: %v", err)
	}
}

func TestExportRange_InvalidFromDate(t *testing.T) {
	e := &Exporter{
		cfg: &config.Config{Timezone: "America/New_York"},
	}

	err := e.ExportRange(context.Background(), "not-a-date", "2026-01-24")
	if err == nil {
		t.Error("ExportRange() should fail with invalid from date")
	}
	if !strings.Contains(err.Error(), "parsing from date") {
		t.Errorf("error should mention parsing from date: %v", err)
	}
}

func TestExportRange_InvalidToDate(t *testing.T) {
	e := &Exporter{
		cfg: &config.Config{Timezone: "America/New_York"},
	}

	err := e.ExportRange(context.Background(), "2026-01-22", "not-a-date")
	if err == nil {
		t.Error("ExportRange() should fail with invalid to date")
	}
	if !strings.Contains(err.Error(), "parsing to date") {
		t.Errorf("error should mention parsing to date: %v", err)
	}
}

func TestExportRange_FromAfterTo(t *testing.T) {
	e := &Exporter{
		cfg: &config.Config{Timezone: "America/New_York"},
	}

	err := e.ExportRange(context.Background(), "2026-01-24", "2026-01-22")
	if err == nil {
		t.Error("ExportRange() should fail when from is after to")
	}
	if !strings.Contains(err.Error(), "cannot be after") {
		t.Errorf("error should mention date ordering: %v", err)
	}
}

func TestExportRange_SingleDay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/users.list" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"members": [],
				"response_metadata": {"next_cursor": ""}
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T999"},
				"team": {"id": "T999", "name": "Test"},
				"channels": [],
				"ims": []
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			_, _ = w.Write([]byte(`{"ok": true, "channels": []}`))
		}
	}))
	defer server.Close()

	creds := &slack.Credentials{Token: "xoxc-test", TeamID: "T999"}
	e := &Exporter{
		cfg:        &config.Config{Timezone: "America/New_York"},
		edgeClient: slack.NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/").WithSlackAPIURL(server.URL),
	}

	err := e.ExportRange(context.Background(), "2026-01-22", "2026-01-22")
	if err != nil {
		t.Errorf("ExportRange() should succeed with single day: %v", err)
	}
}

func TestExportRange_MultiDay(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/users.list" {
			_, _ = w.Write([]byte(`{
				"ok": true,
				"members": [],
				"response_metadata": {"next_cursor": ""}
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			callCount++
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T999"},
				"team": {"id": "T999", "name": "Test"},
				"channels": [],
				"ims": []
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			_, _ = w.Write([]byte(`{"ok": true, "channels": []}`))
		}
	}))
	defer server.Close()

	creds := &slack.Credentials{Token: "xoxc-test", TeamID: "T999"}
	e := &Exporter{
		cfg:        &config.Config{Timezone: "America/New_York"},
		edgeClient: slack.NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/").WithSlackAPIURL(server.URL),
	}

	err := e.ExportRange(context.Background(), "2026-01-22", "2026-01-24")
	if err != nil {
		t.Errorf("ExportRange() should succeed: %v", err)
	}

	// userBoot is called once per ExportDate, so 3 times for 3 days
	if callCount != 3 {
		t.Errorf("expected 3 userBoot calls (one per day), got %d", callCount)
	}
}

func TestExportRange_ContinuesOnError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users.list" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"members": [],
				"response_metadata": {"next_cursor": ""}
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.userBoot") {
			callCount++
			// Fail on second day (2026-01-23), succeed on others
			if callCount == 2 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"ok": false, "error": "server_error"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"ok": true,
				"self": {"id": "U123", "team_id": "T999"},
				"team": {"id": "T999", "name": "Test"},
				"channels": [],
				"ims": []
			}`))
		} else if strings.HasSuffix(r.URL.Path, "/client.counts") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok": true, "channels": []}`))
		}
	}))
	defer server.Close()

	creds := &slack.Credentials{Token: "xoxc-test", TeamID: "T999"}
	e := &Exporter{
		cfg:        &config.Config{Timezone: "America/New_York"},
		edgeClient: slack.NewEdgeClient(creds).WithWorkspaceURL(server.URL + "/").WithSlackAPIURL(server.URL),
	}

	err := e.ExportRange(context.Background(), "2026-01-22", "2026-01-24")
	if err != nil {
		t.Errorf("ExportRange() should continue on single-day errors: %v", err)
	}

	// Should have processed all 3 days despite error on day 2
	if callCount != 3 {
		t.Errorf("expected 3 userBoot calls (continuing past error), got %d", callCount)
	}
}
