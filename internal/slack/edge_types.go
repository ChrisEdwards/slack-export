package slack

import "strings"

// UserBootResponse is the response from the client.userBoot Edge API endpoint.
// Contains all channels, DMs, and groups the user has access to.
type UserBootResponse struct {
	OK       bool              `json:"ok"`
	Error    string            `json:"error,omitempty"`
	Self     Self              `json:"self"`
	Team     Team              `json:"team"`
	IMs      []IM              `json:"ims"`
	Channels []UserBootChannel `json:"channels"`
}

// UserBootChannel represents a channel from the userBoot response.
type UserBootChannel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsChannel  bool   `json:"is_channel"`
	IsGroup    bool   `json:"is_group"`
	IsIM       bool   `json:"is_im"`
	IsMpim     bool   `json:"is_mpim"`
	IsPrivate  bool   `json:"is_private"`
	IsArchived bool   `json:"is_archived"`
	IsMember   bool   `json:"is_member,omitempty"`
	LastRead   string `json:"last_read,omitempty"`
	Latest     string `json:"latest,omitempty"`
	Created    int64  `json:"created"`
	Updated    int64  `json:"updated,omitempty"`
	Creator    string `json:"creator"`
}

// IM represents a direct message conversation from userBoot.
type IM struct {
	ID       string `json:"id"`
	User     string `json:"user"`
	IsIM     bool   `json:"is_im"`
	IsOpen   bool   `json:"is_open"`
	LastRead string `json:"last_read"`
	Latest   string `json:"latest"`
}

// Self represents the current authenticated user.
type Self struct {
	ID     string `json:"id"`
	TeamID string `json:"team_id"`
	Name   string `json:"name"`
}

// Team represents a Slack workspace/team.
type Team struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// CountsResponse is the response from the client.counts Edge API endpoint.
// Contains activity timestamps showing when each channel last had a message.
type CountsResponse struct {
	OK       bool              `json:"ok"`
	Error    string            `json:"error,omitempty"`
	Channels []ChannelSnapshot `json:"channels,omitempty"`
	MPIMs    []ChannelSnapshot `json:"mpims,omitempty"`
	IMs      []ChannelSnapshot `json:"ims,omitempty"`
}

// ChannelSnapshot represents activity info for a single channel from counts API.
// The Latest field is the key field - it tells us when the last message was posted.
type ChannelSnapshot struct {
	ID           string `json:"id"`
	LastRead     string `json:"last_read"`
	Latest       string `json:"latest"`
	MentionCount int    `json:"mention_count"`
	HasUnreads   bool   `json:"has_unreads"`
}

// AuthTestResponse is the response from the Slack auth.test API endpoint.
// Used to verify credentials and obtain workspace information including TeamID.
type AuthTestResponse struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	URL    string `json:"url"`
	Team   string `json:"team"`
	User   string `json:"user"`
	TeamID string `json:"team_id"`
	UserID string `json:"user_id"`
}

// User represents a Slack workspace user from the users.list API.
type User struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	RealName string      `json:"real_name"`
	Deleted  bool        `json:"deleted"`
	Profile  UserProfile `json:"profile"`
}

// UserProfile contains profile information for a Slack user.
type UserProfile struct {
	DisplayName string `json:"display_name"`
	RealName    string `json:"real_name"`
}

// UsersListResponse is the response from the Slack users.list API.
type UsersListResponse struct {
	OK               bool   `json:"ok"`
	Error            string `json:"error,omitempty"`
	Members          []User `json:"members"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// UserInfoResponse is the response from the Slack users.info API.
// Used to fetch individual user details, especially for external Slack Connect users.
type UserInfoResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	User  User   `json:"user"`
}

// UserIndex provides O(1) lookup of users by ID.
type UserIndex map[string]*User

// NewUserIndex builds a UserIndex from a slice of users.
func NewUserIndex(users []User) UserIndex {
	idx := make(UserIndex, len(users))
	for i := range users {
		idx[users[i].ID] = &users[i]
	}
	return idx
}

// DisplayName returns a human-readable name for the given user ID.
// Priority: Profile.DisplayName > RealName > Name
// Falls back to "<unknown>:ID" for unknown users.
func (idx UserIndex) DisplayName(id string) string {
	if id == "" {
		return "unknown"
	}
	user, ok := idx[id]
	if !ok {
		return "<unknown>:" + id
	}
	if user.Profile.DisplayName != "" {
		return user.Profile.DisplayName
	}
	if user.RealName != "" {
		return user.RealName
	}
	if user.Name != "" {
		return user.Name
	}
	return "<unknown>:" + id
}

// Username returns the username (login name) for the given user ID.
// This returns the Name field in lowercase, which is the email prefix format (e.g., "john.ament").
// Falls back to the user ID if the user is not found.
func (idx UserIndex) Username(id string) string {
	if id == "" {
		return "unknown"
	}
	user, ok := idx[id]
	if !ok {
		return id
	}
	if user.Name != "" {
		return strings.ToLower(user.Name)
	}
	return id
}
