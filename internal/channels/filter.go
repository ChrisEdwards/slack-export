// Package channels provides filtering logic for Slack channel selection
// based on glob patterns.
package channels

import "github.com/chrisedwards/slack-export/internal/slack"

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
