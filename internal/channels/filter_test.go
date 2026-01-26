package channels

import (
	"testing"

	"github.com/chrisedwards/slack-export/internal/slack"
)

func TestMatchAny(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		value    string
		want     bool
	}{
		{
			name:     "empty patterns returns false",
			patterns: []string{},
			value:    "anything",
			want:     false,
		},
		{
			name:     "nil patterns returns false",
			patterns: nil,
			value:    "anything",
			want:     false,
		},
		{
			name:     "single pattern match",
			patterns: []string{"eng-*"},
			value:    "eng-backend",
			want:     true,
		},
		{
			name:     "single pattern no match",
			patterns: []string{"eng-*"},
			value:    "marketing",
			want:     false,
		},
		{
			name:     "multiple patterns first matches",
			patterns: []string{"eng-*", "ai-*", "marketing"},
			value:    "eng-frontend",
			want:     true,
		},
		{
			name:     "multiple patterns middle matches",
			patterns: []string{"eng-*", "ai-*", "marketing"},
			value:    "ai-team",
			want:     true,
		},
		{
			name:     "multiple patterns last matches",
			patterns: []string{"eng-*", "ai-*", "marketing"},
			value:    "marketing",
			want:     true,
		},
		{
			name:     "multiple patterns none match",
			patterns: []string{"eng-*", "ai-*", "marketing"},
			value:    "random",
			want:     false,
		},
		{
			name:     "case-insensitive match",
			patterns: []string{"ENG-*"},
			value:    "eng-backend",
			want:     true,
		},
		{
			name:     "channel ID match",
			patterns: []string{"C03*", "C04*"},
			value:    "C03TSU00NK1",
			want:     true,
		},
		{
			name:     "exact channel ID in list",
			patterns: []string{"eng-*", "C12345", "ai-*"},
			value:    "C12345",
			want:     true,
		},
		{
			name:     "empty value with wildcard pattern",
			patterns: []string{"*"},
			value:    "",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchAny(tt.patterns, tt.value)
			if got != tt.want {
				t.Errorf("MatchAny(%v, %q) = %v, want %v",
					tt.patterns, tt.value, got, tt.want)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		value   string
		want    bool
	}{
		{
			name:    "exact match",
			pattern: "engineering",
			value:   "engineering",
			want:    true,
		},
		{
			name:    "exact match case-insensitive",
			pattern: "Engineering",
			value:   "engineering",
			want:    true,
		},
		{
			name:    "exact match case-insensitive reversed",
			pattern: "engineering",
			value:   "ENGINEERING",
			want:    true,
		},
		{
			name:    "no match",
			pattern: "engineering",
			value:   "marketing",
			want:    false,
		},
		{
			name:    "wildcard suffix",
			pattern: "eng-*",
			value:   "eng-backend",
			want:    true,
		},
		{
			name:    "wildcard suffix case-insensitive",
			pattern: "ENG-*",
			value:   "eng-backend",
			want:    true,
		},
		{
			name:    "wildcard prefix",
			pattern: "*-deploys",
			value:   "staging-deploys",
			want:    true,
		},
		{
			name:    "wildcard middle",
			pattern: "eng-*-team",
			value:   "eng-backend-team",
			want:    true,
		},
		{
			name:    "single char wildcard",
			pattern: "eng-?",
			value:   "eng-a",
			want:    true,
		},
		{
			name:    "single char wildcard no match",
			pattern: "eng-?",
			value:   "eng-ab",
			want:    false,
		},
		{
			name:    "channel ID match",
			pattern: "C03TSU00NK1",
			value:   "C03TSU00NK1",
			want:    true,
		},
		{
			name:    "channel ID case-insensitive",
			pattern: "c03tsu00nk1",
			value:   "C03TSU00NK1",
			want:    true,
		},
		{
			name:    "channel ID wildcard",
			pattern: "C03*",
			value:   "C03TSU00NK1",
			want:    true,
		},
		{
			name:    "empty pattern",
			pattern: "",
			value:   "anything",
			want:    false,
		},
		{
			name:    "empty value",
			pattern: "*",
			value:   "",
			want:    true,
		},
		{
			name:    "empty pattern empty value",
			pattern: "",
			value:   "",
			want:    true,
		},
		{
			name:    "invalid pattern - returns false",
			pattern: "[",
			value:   "anything",
			want:    false,
		},
		{
			name:    "character class",
			pattern: "eng-[abc]",
			value:   "eng-b",
			want:    true,
		},
		{
			name:    "character class no match",
			pattern: "eng-[abc]",
			value:   "eng-d",
			want:    false,
		},
		{
			name:    "mixed case with wildcard",
			pattern: "AI-*",
			value:   "ai-team",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchPattern(tt.pattern, tt.value)
			if got != tt.want {
				t.Errorf("MatchPattern(%q, %q) = %v, want %v",
					tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}

func TestFilterChannels(t *testing.T) {
	channels := []slack.Channel{
		{ID: "C1", Name: "eng-backend"},
		{ID: "C2", Name: "eng-frontend"},
		{ID: "C3", Name: "random"},
		{ID: "C4", Name: "_app_bot"},
		{ID: "C5", Name: "ai-team"},
	}

	tests := []struct {
		name     string
		include  []string
		exclude  []string
		expected []string // channel IDs
	}{
		{
			name:     "no filters returns all",
			include:  nil,
			exclude:  nil,
			expected: []string{"C1", "C2", "C3", "C4", "C5"},
		},
		{
			name:     "empty filters returns all",
			include:  []string{},
			exclude:  []string{},
			expected: []string{"C1", "C2", "C3", "C4", "C5"},
		},
		{
			name:     "include eng-* only",
			include:  []string{"eng-*"},
			exclude:  nil,
			expected: []string{"C1", "C2"},
		},
		{
			name:     "exclude _app_*",
			include:  nil,
			exclude:  []string{"_app_*"},
			expected: []string{"C1", "C2", "C3", "C5"},
		},
		{
			name:     "include eng-* exclude *backend*",
			include:  []string{"eng-*"},
			exclude:  []string{"*backend*"},
			expected: []string{"C2"},
		},
		{
			name:     "match by channel ID",
			include:  []string{"C3"},
			exclude:  nil,
			expected: []string{"C3"},
		},
		{
			name:     "case insensitivity - ENG-* matches eng-*",
			include:  []string{"ENG-*"},
			exclude:  nil,
			expected: []string{"C1", "C2"},
		},
		{
			name:     "exclude takes priority over include",
			include:  []string{"eng-*"},
			exclude:  []string{"eng-backend"},
			expected: []string{"C2"},
		},
		{
			name:     "include multiple patterns",
			include:  []string{"eng-*", "ai-*"},
			exclude:  nil,
			expected: []string{"C1", "C2", "C5"},
		},
		{
			name:     "exclude all with *",
			include:  nil,
			exclude:  []string{"*"},
			expected: []string{},
		},
		{
			name:     "include by ID overrides name pattern",
			include:  []string{"eng-*", "C3"},
			exclude:  nil,
			expected: []string{"C1", "C2", "C3"},
		},
		{
			name:     "exclude by ID",
			include:  nil,
			exclude:  []string{"C1", "C2"},
			expected: []string{"C3", "C4", "C5"},
		},
		{
			name:     "order preserved",
			include:  []string{"*"},
			exclude:  []string{"C3"},
			expected: []string{"C1", "C2", "C4", "C5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterChannels(channels, tt.include, tt.exclude)
			got := make([]string, len(result))
			for i, ch := range result {
				got[i] = ch.ID
			}
			if len(got) != len(tt.expected) {
				t.Errorf("FilterChannels() returned %d channels, want %d\ngot: %v\nwant: %v",
					len(got), len(tt.expected), got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("FilterChannels()[%d] = %q, want %q",
						i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestFilterApply(t *testing.T) {
	channels := []slack.Channel{
		{ID: "C1", Name: "eng-backend"},
		{ID: "C2", Name: "marketing"},
	}

	t.Run("Filter.Apply matches FilterChannels", func(t *testing.T) {
		include := []string{"eng-*"}
		exclude := []string{}
		filter := NewFilter(include, exclude)
		applyResult := filter.Apply(channels)
		filterResult := FilterChannels(channels, include, exclude)

		if len(applyResult) != len(filterResult) {
			t.Errorf("Apply returned %d, FilterChannels returned %d",
				len(applyResult), len(filterResult))
			return
		}
		for i := range applyResult {
			if applyResult[i].ID != filterResult[i].ID {
				t.Errorf("Result[%d]: Apply got %s, FilterChannels got %s",
					i, applyResult[i].ID, filterResult[i].ID)
			}
		}
	})

	t.Run("empty channels returns empty", func(t *testing.T) {
		filter := NewFilter([]string{"*"}, nil)
		result := filter.Apply(nil)
		if len(result) != 0 {
			t.Errorf("Expected empty result for nil input, got %d", len(result))
		}
	})
}
