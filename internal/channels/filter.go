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

// FilterChannels filters channels based on include/exclude patterns.
// This is a convenience function that creates a Filter and applies it.
//
// Logic:
//  1. If channel matches ANY exclude pattern (name OR ID) → skip
//  2. If include list is empty → include all non-excluded
//  3. If include list is non-empty → only include if name OR ID matches
func FilterChannels(channels []slack.Channel, include, exclude []string) []slack.Channel {
	return NewFilter(include, exclude).Apply(channels)
}

// Apply filters the given channels based on include/exclude patterns.
// Returns channels that match include patterns and don't match exclude patterns.
//
// Logic:
//  1. If channel matches ANY exclude pattern (name OR ID) → skip
//  2. If include list is empty → include all non-excluded
//  3. If include list is non-empty → only include if name OR ID matches
func (f *Filter) Apply(channels []slack.Channel) []slack.Channel {
	var result []slack.Channel
	for _, ch := range channels {
		if f.matchesExclude(ch) {
			continue
		}
		if len(f.include) == 0 || f.matchesInclude(ch) {
			result = append(result, ch)
		}
	}
	return result
}

// matchesExclude returns true if the channel matches any exclude pattern.
func (f *Filter) matchesExclude(ch slack.Channel) bool {
	return MatchAny(f.exclude, ch.Name) || MatchAny(f.exclude, ch.ID)
}

// matchesInclude returns true if the channel matches any include pattern.
func (f *Filter) matchesInclude(ch slack.Channel) bool {
	return MatchAny(f.include, ch.Name) || MatchAny(f.include, ch.ID)
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
