package channels

import "testing"

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
