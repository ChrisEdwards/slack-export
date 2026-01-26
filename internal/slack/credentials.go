// Package slack provides Slack API integration including credential management
// and Edge API client for channel detection.
package slack

// LoadCredentials reads slackdump's cached credentials from the filesystem.
// Returns credentials needed for Slack Edge API calls.
func LoadCredentials() (*Credentials, error) {
	// TODO: Implement credential loading from slackdump cache
	return nil, nil
}
