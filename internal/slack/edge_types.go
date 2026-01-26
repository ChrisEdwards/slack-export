package slack

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
