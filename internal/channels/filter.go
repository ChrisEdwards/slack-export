// Package channels provides filtering logic for Slack channel selection
// based on glob patterns.
package channels

import (
	"path/filepath"
	"strings"

	"github.com/chrisedwards/slack-export/internal/slack"
)

// Filter applies include/exclude patterns to a list of channels.
type Filter struct {
	include []string
	exclude []string
}

// NewFilter creates a Filter with the given include and exclude patterns.
func NewFilter(include, exclude []string) *Filter {
	return &Filter{
		include: include,
		exclude: exclude,
	}
}

// Apply filters the given channels based on include/exclude patterns.
// Returns channels that match include patterns and don't match exclude patterns.
func (f *Filter) Apply(channels []slack.Channel) []slack.Channel {
	// TODO: Implement glob-based filtering
	return channels
}

// MatchAny checks if a value matches any pattern in a list.
// Returns true if any pattern matches, false for empty pattern list.
// Short-circuits on first match.
func MatchAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if MatchPattern(pattern, value) {
			return true
		}
	}
	return false
}

// MatchPattern matches a value against a glob pattern.
// Supports glob patterns (* matches any sequence, ? matches single character).
// Matching is case-insensitive. Returns false for invalid patterns.
func MatchPattern(pattern, value string) bool {
	matched, err := filepath.Match(pattern, value)
	if err != nil {
		return false
	}
	if matched {
		return true
	}
	lowerPattern := strings.ToLower(pattern)
	lowerValue := strings.ToLower(value)
	matched, _ = filepath.Match(lowerPattern, lowerValue)
	return matched
}
