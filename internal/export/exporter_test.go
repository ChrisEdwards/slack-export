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

	edgeClient := slack.NewEdgeClient(creds).WithBaseURL(server.URL)

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

	edgeClient := slack.NewEdgeClient(creds).WithBaseURL(server.URL)

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
