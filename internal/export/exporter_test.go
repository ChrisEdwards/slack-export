package export

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/chrisedwards/slack-export/internal/config"
	"github.com/chrisedwards/slack-export/internal/slack"
	"github.com/rusq/slackdump/v4/source"
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
		OutputDir: tmpDir,
		Timezone:  "America/New_York",
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
		OutputDir: filepath.Join(tmpDir, "output"),
		Timezone:  "America/New_York",
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
		OutputDir: filepath.Join(tmpDir, "output"),
		Timezone:  "America/New_York",
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

func TestMovedResumeChannelIDs_SkipsMissingCheckpointBeforeCoverage(t *testing.T) {
	coverageStart := time.Date(2026, 7, 3, 7, 0, 0, 0, time.UTC)
	checkpoint := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)

	tracked := []slack.Channel{
		{ID: "C_OLD_MISSING"},
		{ID: "C_NEW_MISSING"},
		{ID: "C_UNKNOWN_MISSING"},
		{ID: "C_UNCHANGED"},
		{ID: "C_MOVED"},
	}
	checkpoints := map[string]time.Time{
		"C_UNCHANGED": checkpoint,
		"C_MOVED":     checkpoint,
	}
	countLatest := map[string]time.Time{
		"C_OLD_MISSING": coverageStart.Add(-time.Minute),
		"C_NEW_MISSING": coverageStart.Add(time.Minute),
		"C_UNCHANGED":   checkpoint,
		"C_MOVED":       checkpoint.Add(time.Minute),
	}

	got := movedResumeChannelIDs(tracked, checkpoints, countLatest, coverageStart)
	want := []string{"C_NEW_MISSING", "C_UNKNOWN_MISSING", "C_MOVED"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("movedResumeChannelIDs() = %v, want %v", got, want)
	}
}

type testSlackLink string

func (l testSlackLink) String() string {
	return string(l)
}

func TestScopedResumeArgsFromLatest_ExcludesUnmovedAndPreservesMovedCheckpoints(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	coverageStart := time.Date(2026, 7, 3, 7, 0, 0, 0, time.UTC)
	tracked := []slack.Channel{
		{ID: "C_MOVED"},
		{ID: "C_UNMOVED"},
		{ID: "C_NEW"},
	}
	latest := map[testSlackLink]time.Time{
		"C_MOVED":           now,
		"C_MOVED:111.111":   now,
		"C_UNMOVED":         now,
		"C_UNMOVED:222.222": now,
		"C_UNTRACKED":       now,
		"C_UNTRACKED:333":   now,
	}
	checkpoints := map[string]time.Time{
		"C_MOVED":     now,
		"C_UNMOVED":   now,
		"C_UNTRACKED": now,
	}
	movedIDs := []string{"C_MOVED", "C_NEW"}

	got := scopedResumeArgsFromLatest(tracked, latest, checkpoints, movedIDs, coverageStart)
	sort.Strings(got)
	want := []string{
		"C_NEW,2026-07-03T07:00:00",
		"^C_UNMOVED",
		"^C_UNMOVED:222.222",
		"^C_UNTRACKED",
		"^C_UNTRACKED:333",
	}
	sort.Strings(want)

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("scopedResumeArgsFromLatest() = %v, want %v", got, want)
	}
}

func TestScopedResumeArgsFromLatest_BoundsMissingCheckpointsAtCoverageStartUTC(t *testing.T) {
	eastern, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("loading timezone: %v", err)
	}
	coverageStart := time.Date(2026, 7, 3, 3, 0, 0, 0, eastern)

	got := scopedResumeArgsFromLatest(
		[]slack.Channel{{ID: "C_NEW"}},
		map[testSlackLink]time.Time{},
		map[string]time.Time{},
		[]string{"C_NEW"},
		coverageStart,
	)
	want := []string{"C_NEW,2026-07-03T07:00:00"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("scopedResumeArgsFromLatest() = %v, want %v", got, want)
	}
}

func TestScopedResumeArgsFromLatest_MovedOnlyUsesExistingCheckpoints(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	got := scopedResumeArgsFromLatest(
		[]slack.Channel{{ID: "C_MOVED"}},
		map[testSlackLink]time.Time{
			"C_MOVED":         now,
			"C_MOVED:111.111": now,
		},
		map[string]time.Time{"C_MOVED": now},
		[]string{"C_MOVED"},
		time.Date(2026, 7, 3, 7, 0, 0, 0, time.UTC),
	)
	if len(got) != 0 {
		t.Fatalf("scopedResumeArgsFromLatest() = %v, want empty args", got)
	}
}

func TestScopedResumeArgsFromLatest_NoMovedChannels(t *testing.T) {
	got := scopedResumeArgsFromLatest(
		[]slack.Channel{{ID: "C_UNMOVED"}},
		map[testSlackLink]time.Time{"C_UNMOVED": time.Now()},
		map[string]time.Time{"C_UNMOVED": time.Now()},
		nil,
		time.Date(2026, 7, 3, 7, 0, 0, 0, time.UTC),
	)
	if got != nil {
		t.Fatalf("scopedResumeArgsFromLatest() = %v, want nil", got)
	}
}

func TestFullSweepResumeArgsFromLatest_ExcludesUntrackedAndBoundsMissingTracked(t *testing.T) {
	now := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	coverageStart := time.Date(2026, 7, 3, 7, 0, 0, 0, time.UTC)
	tracked := []slack.Channel{
		{ID: "C_TRACKED"},
		{ID: "C_NEW"},
	}
	latest := map[testSlackLink]time.Time{
		"C_TRACKED":           now,
		"C_TRACKED:111.111":   now,
		"C_UNTRACKED":         now,
		"C_UNTRACKED:222.222": now,
	}

	got := fullSweepResumeArgsFromLatest(tracked, latest, coverageStart)
	sort.Strings(got)
	want := []string{
		"C_NEW,2026-07-03T07:00:00",
		"^C_UNTRACKED",
		"^C_UNTRACKED:222.222",
	}
	sort.Strings(want)

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("fullSweepResumeArgsFromLatest() = %v, want %v", got, want)
	}
}

func TestResumeOptions_ModeSplitIgnoresOldFullSweepMarker(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name              string
		opts              SyncOptions
		wantLookback      string
		wantDedupe        bool
		wantSkipStale     string
		wantSkipComplete  bool
		wantAPIConfigPath bool
	}{
		{
			name:             "daily run keeps configured recent options and skips dedupe",
			wantLookback:     "7d",
			wantSkipStale:    "21d",
			wantSkipComplete: true,
		},
		{
			name:              "full run always uses bounded sweep options",
			opts:              SyncOptions{Full: true},
			wantLookback:      "90d",
			wantDedupe:        true,
			wantSkipStale:     "90d",
			wantSkipComplete:  true,
			wantAPIConfigPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiveDir := t.TempDir()
			oldMarkerPath := filepath.Join(archiveDir, ".slack-export-last-full-sweep")
			if err := os.WriteFile(oldMarkerPath, []byte(now.Add(-30*24*time.Hour).Format(time.RFC3339)), 0600); err != nil {
				t.Fatalf("writing old marker: %v", err)
			}
			e := &Exporter{cfg: &config.Config{
				Lookback:            "7d",
				SkipStaleThreads:    "21d",
				SkipCompleteThreads: true,
			}}

			got, err := e.resumeOptions(archiveDir, tt.opts)
			if err != nil {
				t.Fatalf("resumeOptions() error = %v", err)
			}
			if got.Lookback != tt.wantLookback {
				t.Fatalf("resumeOptions().Lookback = %q, want %q", got.Lookback, tt.wantLookback)
			}
			if got.Dedupe != tt.wantDedupe {
				t.Fatalf("resumeOptions().Dedupe = %v, want %v", got.Dedupe, tt.wantDedupe)
			}
			if got.SkipStaleThreads != tt.wantSkipStale {
				t.Fatalf("resumeOptions().SkipStaleThreads = %q, want %q", got.SkipStaleThreads, tt.wantSkipStale)
			}
			if got.SkipCompleteThreads != tt.wantSkipComplete {
				t.Fatalf("resumeOptions().SkipCompleteThreads = %v, want %v", got.SkipCompleteThreads, tt.wantSkipComplete)
			}
			if (got.APIConfigPath != "") != tt.wantAPIConfigPath {
				t.Fatalf("resumeOptions().APIConfigPath = %q, want path? %v", got.APIConfigPath, tt.wantAPIConfigPath)
			}
		})
	}
}

func TestAcquireArchiveLock_UsesSiblingPathAndIsNonBlocking(t *testing.T) {
	archiveDir := filepath.Join(t.TempDir(), "workspace-archive")
	first, acquired, err := acquireArchiveLock(archiveDir)
	if err != nil {
		t.Fatalf("acquireArchiveLock() first error = %v", err)
	}
	if !acquired {
		t.Fatal("first acquire should succeed")
	}
	defer func() { _ = first.Release() }()

	if _, err := os.Stat(archiveDir + ".lock"); err != nil {
		t.Fatalf("lock file should be sibling path: %v", err)
	}

	second, acquired, err := acquireArchiveLock(archiveDir)
	if err != nil {
		t.Fatalf("acquireArchiveLock() second error = %v", err)
	}
	if acquired {
		_ = second.Release()
		t.Fatal("second acquire should report contention without blocking")
	}

	if err := first.Release(); err != nil {
		t.Fatalf("releasing first lock: %v", err)
	}
	third, acquired, err := acquireArchiveLock(archiveDir)
	if err != nil {
		t.Fatalf("acquireArchiveLock() third error = %v", err)
	}
	if !acquired {
		t.Fatal("third acquire should succeed after release")
	}
	if err := third.Release(); err != nil {
		t.Fatalf("releasing third lock: %v", err)
	}
}

func TestSync_DailyContentionSkipsBeforeFetchingSlack(t *testing.T) {
	baseDir := t.TempDir()
	cfg := &config.Config{
		ArchiveDir: baseDir,
		OutputDir:  filepath.Join(t.TempDir(), "out"),
		Timezone:   "America/New_York",
	}
	e := &Exporter{
		cfg:   cfg,
		creds: &slack.Credentials{Workspace: "locked-team"},
	}
	archiveDir, err := e.ArchiveDir()
	if err != nil {
		t.Fatalf("ArchiveDir() error = %v", err)
	}
	lock, acquired, err := acquireArchiveLock(archiveDir)
	if err != nil {
		t.Fatalf("acquireArchiveLock() error = %v", err)
	}
	if !acquired {
		t.Fatal("setup lock should acquire")
	}
	defer func() { _ = lock.Release() }()

	if err := e.Sync(context.Background(), time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC), SyncOptions{}); err != nil {
		t.Fatalf("daily Sync() under contention error = %v", err)
	}
}

func TestMarkSweepSuccess_WritesLastSuccessHealthMarker(t *testing.T) {
	archiveDir := t.TempDir()
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

	if err := markSweepSuccess(archiveDir, now); err != nil {
		t.Fatalf("markSweepSuccess() error = %v", err)
	}
	got, ok, err := lastSweepSuccess(archiveDir)
	if err != nil {
		t.Fatalf("lastSweepSuccess() error = %v", err)
	}
	if !ok {
		t.Fatal("lastSweepSuccess() ok = false, want true")
	}
	if !got.Equal(now) {
		t.Fatalf("lastSweepSuccess() = %s, want %s", got, now)
	}
	if _, err := os.Stat(filepath.Join(archiveDir, ".slack-export-last-full-sweep")); !os.IsNotExist(err) {
		t.Fatalf("old re-escalation marker should not be written, stat err = %v", err)
	}
}

func TestWrittenResumeRenderTargets_UsesLatestFinishedResumeMessageDates(t *testing.T) {
	archiveDir := t.TempDir()
	db := openTestArchiveDB(t, archiveDir)
	defer func() { _ = db.Close() }()

	execTestSQL(t, db, `CREATE TABLE SESSION (ID INTEGER PRIMARY KEY, FINISHED SMALLINT, MODE TEXT NOT NULL)`)
	execTestSQL(t, db, `
		CREATE TABLE CHUNK (
			ID INTEGER PRIMARY KEY,
			SESSION_ID INTEGER NOT NULL,
			NUM_REC INTEGER NOT NULL DEFAULT 0,
			CHANNEL_ID TEXT
		)
	`)
	execTestSQL(t, db, `
		CREATE TABLE MESSAGE (
			ID INTEGER NOT NULL,
			CHUNK_ID INTEGER NOT NULL,
			CHANNEL_ID TEXT NOT NULL,
			TS TEXT NOT NULL,
			IDX INTEGER NOT NULL,
			DATA BLOB NOT NULL,
			PRIMARY KEY (ID, CHUNK_ID)
		)
	`)
	execTestSQL(t, db, `
		INSERT INTO SESSION (ID, FINISHED, MODE) VALUES
			(1, 1, 'resume'),
			(2, 0, 'resume'),
			(3, 1, 'resume')
	`)
	execTestSQL(t, db, `
		INSERT INTO CHUNK (ID, SESSION_ID, NUM_REC, CHANNEL_ID) VALUES
			(10, 1, 1, 'C_OLD'),
			(20, 2, 1, 'C_UNFINISHED'),
			(30, 3, 2, 'C_ALPHA'),
			(31, 3, 1, 'C_BETA')
	`)
	execTestSQL(t, db, `
		INSERT INTO MESSAGE (ID, CHUNK_ID, CHANNEL_ID, TS, IDX, DATA) VALUES
			(1, 10, 'C_OLD', '1782922930.000000', 0, '{}'),
			(2, 20, 'C_UNFINISHED', '1783094460.000000', 0, '{}'),
			(3, 30, 'C_ALPHA', '1782922930.000000', 0, '{}'),
			(4, 30, 'C_ALPHA', '1783063800.000000', 1, '{}'),
			(5, 31, 'C_BETA', '1783094460.000000', 0, '{}')
	`)

	got, err := writtenResumeRenderTargets(archiveDir, "America/Chicago")
	if err != nil {
		t.Fatalf("writtenResumeRenderTargets() error = %v", err)
	}
	want := []renderTarget{
		{channelID: "C_ALPHA", date: "2026-07-01"},
		{channelID: "C_ALPHA", date: "2026-07-02"},
		{channelID: "C_BETA", date: "2026-07-03"},
	}
	if strings.Join(renderTargetsForTest(got), "|") != strings.Join(renderTargetsForTest(want), "|") {
		t.Fatalf("writtenResumeRenderTargets() = %v, want %v", got, want)
	}
}

func renderTargetsForTest(targets []renderTarget) []string {
	formatted := make([]string, 0, len(targets))
	for _, target := range targets {
		formatted = append(formatted, target.channelID+":"+target.date)
	}
	return formatted
}

func openTestArchiveDB(t *testing.T, archiveDir string) *sql.DB {
	t.Helper()
	if err := os.MkdirAll(archiveDir, 0750); err != nil {
		t.Fatalf("creating archive dir: %v", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(archiveDir, source.DefaultDBFile))
	if err != nil {
		t.Fatalf("opening sqlite db: %v", err)
	}
	return db
}

func execTestSQL(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	if _, err := db.Exec(stmt); err != nil {
		t.Fatalf("executing SQL %q: %v", stmt, err)
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
	if !strings.Contains(err.Error(), "parsing from date") {
		t.Errorf("error should mention parsing from date: %v", err)
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
	if !strings.Contains(err.Error(), "loading timezone") {
		t.Errorf("error should mention loading timezone: %v", err)
	}
}

func TestExportDate_NoActiveChannels(t *testing.T) {
	e := &Exporter{
		cfg: &config.Config{
			ArchiveDir: t.TempDir(),
			SeedDate:   "2026-01-01",
			Timezone:   "America/New_York",
		},
		creds: &slack.Credentials{Workspace: "test"},
	}

	err := e.ExportDate(context.Background(), "2026-01-22")
	if err == nil {
		t.Fatal("ExportDate() should fail when archive is missing")
	}
	if !strings.Contains(err.Error(), "run slack-export sync first") {
		t.Errorf("error should tell user to run sync: %v", err)
	}
}

func TestExportDate_AllChannelsFilteredOut(t *testing.T) {
	archiveBase := t.TempDir()
	e := &Exporter{
		cfg: &config.Config{
			ArchiveDir: archiveBase,
			SeedDate:   "2026-01-23",
			Timezone:   "America/New_York",
		},
		creds: &slack.Credentials{Workspace: "test"},
	}

	err := e.ExportDate(context.Background(), "2026-01-22")
	if err == nil {
		t.Fatal("ExportDate() should fail before seed date")
	}
	if !strings.Contains(err.Error(), "predates archive seed_date") {
		t.Errorf("error should mention reseeding: %v", err)
	}
}

func TestExportDate_EdgeAPIError(t *testing.T) {
	e := &Exporter{
		cfg: &config.Config{
			ArchiveDir: t.TempDir(),
			SeedDate:   "2026-01-01",
			Timezone:   "America/New_York",
		},
		creds: &slack.Credentials{Workspace: "Test Workspace"},
	}

	archiveDir, err := e.ArchiveDir()
	if err != nil {
		t.Fatalf("ArchiveDir() error = %v", err)
	}
	if !strings.HasSuffix(archiveDir, "test_workspace") {
		t.Errorf("ArchiveDir() = %q, want sanitized workspace suffix", archiveDir)
	}
	err = e.ExportDate(context.Background(), "2026-01-22")
	if err == nil {
		t.Error("ExportDate() should fail without archive")
	}
	if !strings.Contains(err.Error(), "archive does not exist") {
		t.Errorf("error should mention archive missing: %v", err)
	}
}

func TestChannelIDs(t *testing.T) {
	chans := []slack.Channel{
		{ID: "C001", Name: "general"},
		{ID: "C002", Name: "random"},
		{ID: "D001", Name: "dm_bob"},
	}

	ids := channelIDs(chans)

	if len(ids) != 3 {
		t.Errorf("expected 3 ids, got %d", len(ids))
	}

	expectedIDs := []string{"C001", "C002", "D001"}
	for i, id := range ids {
		if id != expectedIDs[i] {
			t.Errorf("ids[%d] = %q, want %q", i, id, expectedIDs[i])
		}
	}
}

func TestChannelIDs_Empty(t *testing.T) {
	ids := channelIDs(nil)

	if len(ids) != 0 {
		t.Errorf("expected empty ids, got %d", len(ids))
	}
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
	e := &Exporter{
		cfg: &config.Config{
			ArchiveDir: t.TempDir(),
			SeedDate:   "2026-01-01",
			Timezone:   "America/New_York",
		},
		creds: &slack.Credentials{Workspace: "test"},
	}

	err := e.ExportRange(context.Background(), "2026-01-22", "2026-01-22")
	if err == nil {
		t.Fatal("ExportRange() should fail when archive is missing")
	}
	if !strings.Contains(err.Error(), "run slack-export sync first") {
		t.Errorf("error should tell user to run sync: %v", err)
	}
}

func TestExportRange_MultiDay(t *testing.T) {
	e := &Exporter{
		cfg: &config.Config{
			ArchiveDir: t.TempDir(),
			SeedDate:   "2026-01-01",
			Timezone:   "America/New_York",
		},
		creds: &slack.Credentials{Workspace: "test"},
	}

	err := e.ExportRange(context.Background(), "2026-01-22", "2026-01-24")
	if err == nil {
		t.Fatal("ExportRange() should fail when archive is missing")
	}
}

func TestExportRange_ContinuesOnErrorAndReturnsFailure(t *testing.T) {
	e := &Exporter{
		cfg: &config.Config{
			ArchiveDir: t.TempDir(),
			SeedDate:   "2026-01-23",
			Timezone:   "America/New_York",
		},
		creds: &slack.Credentials{Workspace: "test"},
	}

	err := e.ExportRange(context.Background(), "2026-01-22", "2026-01-24")
	if err == nil {
		t.Fatal("ExportRange() should fail when range starts before seed")
	}
	for _, want := range []string{"predates archive seed_date", "2026-01-23"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ExportRange() error = %q, want it to contain %q", err.Error(), want)
		}
	}
}
