package export

import (
	"testing"

	appslack "github.com/chrisedwards/slack-export/internal/slack"
	rslack "github.com/rusq/slack"
)

func TestSaveAndLoadChannelNames(t *testing.T) {
	archiveDir := t.TempDir()

	err := saveChannelNames(archiveDir, []appslack.Channel{
		{ID: "C123", Name: "engineering"},
		{ID: "C_EMPTY_NAME"},
		{Name: "missing-id"},
	})
	if err != nil {
		t.Fatalf("saveChannelNames() error = %v", err)
	}

	got, err := loadChannelNames(archiveDir)
	if err != nil {
		t.Fatalf("loadChannelNames() error = %v", err)
	}
	if got["C123"] != "engineering" {
		t.Fatalf("C123 = %q, want engineering", got["C123"])
	}
	if _, ok := got["C_EMPTY_NAME"]; ok {
		t.Fatalf("channel with empty name should not be saved")
	}
}

func TestChannelNameResolverFallsBackToArchiveName(t *testing.T) {
	ch := rslack.Channel{
		GroupConversation: rslack.GroupConversation{
			Conversation: rslack.Conversation{ID: "C123"},
			Name:         "archive-name",
		},
	}

	got := channelNameResolver(nil).fileName(ch)
	if got != "archive-name" {
		t.Fatalf("fileName() = %q, want archive-name", got)
	}
}
