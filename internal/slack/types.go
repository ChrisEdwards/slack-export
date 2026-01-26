package slack

import (
	"net/http"
	"time"
)

// Channel represents a Slack channel with activity metadata.
// Combines metadata from userBoot with timestamps from counts endpoint.
type Channel struct {
	ID          string    // Channel ID (C..., D..., G...)
	Name        string    // Human-readable name
	IsChannel   bool      // Public channel
	IsGroup     bool      // Private channel
	IsIM        bool      // Direct message
	IsMPIM      bool      // Multi-party IM
	IsPrivate   bool      // Private flag
	IsArchived  bool      // Archived flag
	IsMember    bool      // User is member
	LastRead    time.Time // Last read timestamp
	LastMessage time.Time // Most recent message timestamp
}

// Credentials holds authentication data for Slack API access.
type Credentials struct {
	Token     string         // xoxc-... token
	Cookies   []*http.Cookie // Session cookies including 'd' cookie
	TeamID    string         // Workspace ID (T...)
	Workspace string         // Workspace name (from workspace.txt)
}
