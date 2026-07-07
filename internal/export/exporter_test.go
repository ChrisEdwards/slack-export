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

func TestResumeOptions_DedupeOnlyOnFullSweep(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name             string
		markAt           *time.Time
		wantDedupe       bool
		wantSkipStale    string
		wantSkipChannels string
	}{
		{
			name:             "normal run keeps skip-stale filters and skips dedupe",
			markAt:           ptrTime(now.Add(-24 * time.Hour)),
			wantSkipStale:    "21d",
			wantSkipChannels: "21d",
		},
		{
			name:       "missing marker triggers full sweep and dedupe",
			wantDedupe: true,
		},
		{
			name:       "expired marker triggers full sweep and dedupe",
			markAt:     ptrTime(now.Add(-8 * 24 * time.Hour)),
			wantDedupe: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			archiveDir := t.TempDir()
			if tt.markAt != nil {
				if err := markFullSweep(archiveDir, *tt.markAt); err != nil {
					t.Fatalf("markFullSweep() error = %v", err)
				}
			}
			e := &Exporter{cfg: &config.Config{
				Lookback:            "7d",
				SkipStaleThreads:    "21d",
				SkipStaleChannels:   "21d",
				SkipCompleteThreads: true,
				FullSweepInterval:   "7d",
			}}

			got := e.resumeOptions(archiveDir, now)
			if got.Dedupe != tt.wantDedupe {
				t.Fatalf("resumeOptions().Dedupe = %v, want %v", got.Dedupe, tt.wantDedupe)
			}
			if got.SkipStaleThreads != tt.wantSkipStale {
				t.Fatalf("resumeOptions().SkipStaleThreads = %q, want %q", got.SkipStaleThreads, tt.wantSkipStale)
			}
			if got.SkipStaleChannels != tt.wantSkipChannels {
				t.Fatalf("resumeOptions().SkipStaleChannels = %q, want %q", got.SkipStaleChannels, tt.wantSkipChannels)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func TestChangedResumeChannelIDs_UsesNewestFinishedResumeSession(t *testing.T) {
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
		INSERT INTO SESSION (ID, FINISHED, MODE) VALUES
			(1, 1, 'archive'),
			(2, 1, 'resume'),
			(3, 0, 'resume'),
			(4, 1, 'resume')
	`)
	execTestSQL(t, db, `
		INSERT INTO CHUNK (ID, SESSION_ID, NUM_REC, CHANNEL_ID) VALUES
			(1, 2, 5, 'C_OLD'),
			(2, 3, 7, 'C_UNFINISHED'),
			(3, 4, 1, 'C_ALPHA'),
			(4, 4, 3, 'C_BETA'),
			(5, 4, 0, 'C_EMPTY'),
			(6, 4, 2, NULL),
			(7, 4, 4, 'C_ALPHA')
	`)

	got, err := changedResumeChannelIDs(archiveDir)
	if err != nil {
		t.Fatalf("changedResumeChannelIDs() error = %v", err)
	}
	want := []string{"C_ALPHA", "C_BETA"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("changedResumeChannelIDs() = %v, want %v", got, want)
	}
}

func TestRenderChannelIDsForSync(t *testing.T) {
	tracked := []string{"C_ALPHA", "C_BETA", "C_GAMMA"}
	tests := []struct {
		name      string
		changed   []string
		renderAll bool
		want      []string
	}{
		{
			name:      "bootstrap or full sweep renders all tracked channels",
			changed:   []string{"C_ALPHA"},
			renderAll: true,
			want:      tracked,
		},
		{
			name:    "normal run renders changed tracked channels in tracked order",
			changed: []string{"C_GAMMA", "C_UNTRACKED", "C_ALPHA"},
			want:    []string{"C_ALPHA", "C_GAMMA"},
		},
		{
			name: "normal run with no changed chunks renders no channels",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderChannelIDsForSync(tracked, tt.changed, tt.renderAll)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("renderChannelIDsForSync() = %v, want %v", got, tt.want)
			}
		})
	}
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
