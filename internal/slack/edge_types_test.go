package slack

import (
	"encoding/json"
	"testing"
)

func TestUserBootResponse_Unmarshal(t *testing.T) {
	jsonData := `{
		"ok": true,
		"self": {
			"id": "U12345678",
			"team_id": "T12345678",
			"name": "testuser"
		},
		"team": {
			"id": "T12345678",
			"name": "Test Team",
			"domain": "testteam"
		},
		"ims": [
			{
				"id": "D12345678",
				"user": "U87654321",
				"is_im": true,
				"is_open": true,
				"last_read": "1737676800.123456",
				"latest": "1737676900.654321"
			}
		],
		"channels": [
			{
				"id": "C12345678",
				"name": "general",
				"is_channel": true,
				"is_group": false,
				"is_im": false,
				"is_mpim": false,
				"is_private": false,
				"is_archived": false,
				"is_member": true,
				"last_read": "1737676800.000000",
				"latest": "1737676800.123456",
				"created": 1609459200,
				"creator": "U12345678"
			},
			{
				"id": "G87654321",
				"name": "private-group",
				"is_channel": false,
				"is_group": true,
				"is_im": false,
				"is_mpim": false,
				"is_private": true,
				"is_archived": true,
				"created": 1609459200,
				"updated": 1737676800000,
				"creator": "U12345678"
			}
		]
	}`

	var resp UserBootResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !resp.OK {
		t.Error("expected OK to be true")
	}

	// Verify Self
	if resp.Self.ID != "U12345678" {
		t.Errorf("expected Self.ID U12345678, got %s", resp.Self.ID)
	}
	if resp.Self.TeamID != "T12345678" {
		t.Errorf("expected Self.TeamID T12345678, got %s", resp.Self.TeamID)
	}
	if resp.Self.Name != "testuser" {
		t.Errorf("expected Self.Name testuser, got %s", resp.Self.Name)
	}

	// Verify Team
	if resp.Team.ID != "T12345678" {
		t.Errorf("expected Team.ID T12345678, got %s", resp.Team.ID)
	}
	if resp.Team.Name != "Test Team" {
		t.Errorf("expected Team.Name Test Team, got %s", resp.Team.Name)
	}

	// Verify IMs
	if len(resp.IMs) != 1 {
		t.Fatalf("expected 1 IM, got %d", len(resp.IMs))
	}
	if resp.IMs[0].ID != "D12345678" {
		t.Errorf("expected IM ID D12345678, got %s", resp.IMs[0].ID)
	}
	if !resp.IMs[0].IsIM {
		t.Error("expected IM.IsIM to be true")
	}

	// Verify Channels
	if len(resp.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(resp.Channels))
	}

	// First channel - public
	ch1 := resp.Channels[0]
	if ch1.ID != "C12345678" {
		t.Errorf("expected channel ID C12345678, got %s", ch1.ID)
	}
	if ch1.Name != "general" {
		t.Errorf("expected channel name general, got %s", ch1.Name)
	}
	if !ch1.IsChannel {
		t.Error("expected IsChannel to be true")
	}
	if !ch1.IsMember {
		t.Error("expected IsMember to be true")
	}
	if ch1.IsArchived {
		t.Error("expected IsArchived to be false")
	}
	if ch1.Latest != "1737676800.123456" {
		t.Errorf("expected Latest 1737676800.123456, got %s", ch1.Latest)
	}

	// Second channel - private archived
	ch2 := resp.Channels[1]
	if !ch2.IsGroup {
		t.Error("expected IsGroup to be true")
	}
	if !ch2.IsPrivate {
		t.Error("expected IsPrivate to be true")
	}
	if !ch2.IsArchived {
		t.Error("expected IsArchived to be true")
	}
	if ch2.Updated != 1737676800000 {
		t.Errorf("expected Updated 1737676800000, got %d", ch2.Updated)
	}
}

func TestUserBootResponse_Error(t *testing.T) {
	jsonData := `{
		"ok": false,
		"error": "invalid_auth"
	}`

	var resp UserBootResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.OK {
		t.Error("expected OK to be false")
	}
	if resp.Error != "invalid_auth" {
		t.Errorf("expected error invalid_auth, got %s", resp.Error)
	}
}

func TestCountsResponse_Unmarshal(t *testing.T) {
	jsonData := `{
		"ok": true,
		"channels": [
			{
				"id": "C12345678",
				"last_read": "1737676800.000000",
				"latest": "1737676900.123456",
				"mention_count": 5,
				"has_unreads": true
			},
			{
				"id": "C87654321",
				"last_read": "1737676500.000000",
				"latest": "1737676500.000000",
				"mention_count": 0,
				"has_unreads": false
			}
		],
		"mpims": [
			{
				"id": "G11111111",
				"last_read": "1737676000.000000",
				"latest": "1737676100.000000",
				"mention_count": 1,
				"has_unreads": true
			}
		],
		"ims": [
			{
				"id": "D22222222",
				"last_read": "1737675000.000000",
				"latest": "1737675500.000000",
				"mention_count": 0,
				"has_unreads": false
			}
		]
	}`

	var resp CountsResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !resp.OK {
		t.Error("expected OK to be true")
	}

	// Verify Channels
	if len(resp.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(resp.Channels))
	}

	ch1 := resp.Channels[0]
	if ch1.ID != "C12345678" {
		t.Errorf("expected channel ID C12345678, got %s", ch1.ID)
	}
	if ch1.Latest != "1737676900.123456" {
		t.Errorf("expected Latest 1737676900.123456, got %s", ch1.Latest)
	}
	if ch1.MentionCount != 5 {
		t.Errorf("expected MentionCount 5, got %d", ch1.MentionCount)
	}
	if !ch1.HasUnreads {
		t.Error("expected HasUnreads to be true")
	}

	// Verify MPIMs
	if len(resp.MPIMs) != 1 {
		t.Fatalf("expected 1 MPIM, got %d", len(resp.MPIMs))
	}
	if resp.MPIMs[0].ID != "G11111111" {
		t.Errorf("expected MPIM ID G11111111, got %s", resp.MPIMs[0].ID)
	}

	// Verify IMs
	if len(resp.IMs) != 1 {
		t.Fatalf("expected 1 IM, got %d", len(resp.IMs))
	}
	if resp.IMs[0].ID != "D22222222" {
		t.Errorf("expected IM ID D22222222, got %s", resp.IMs[0].ID)
	}
}

func TestCountsResponse_Error(t *testing.T) {
	jsonData := `{
		"ok": false,
		"error": "not_authed"
	}`

	var resp CountsResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.OK {
		t.Error("expected OK to be false")
	}
	if resp.Error != "not_authed" {
		t.Errorf("expected error not_authed, got %s", resp.Error)
	}
}

func TestCountsResponse_EmptyLists(t *testing.T) {
	jsonData := `{
		"ok": true
	}`

	var resp CountsResponse
	err := json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !resp.OK {
		t.Error("expected OK to be true")
	}
	if resp.Channels != nil {
		t.Error("expected Channels to be nil")
	}
	if resp.MPIMs != nil {
		t.Error("expected MPIMs to be nil")
	}
	if resp.IMs != nil {
		t.Error("expected IMs to be nil")
	}
}

func TestChannelSnapshot_LatestFieldFormat(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected string
	}{
		{
			name:     "standard format",
			json:     `{"id": "C1", "last_read": "0", "latest": "1737676800.123456", "mention_count": 0, "has_unreads": false}`,
			expected: "1737676800.123456",
		},
		{
			name:     "zero timestamp",
			json:     `{"id": "C1", "last_read": "0", "latest": "0", "mention_count": 0, "has_unreads": false}`,
			expected: "0",
		},
		{
			name:     "empty latest",
			json:     `{"id": "C1", "last_read": "0", "latest": "", "mention_count": 0, "has_unreads": false}`,
			expected: "",
		},
		{
			name:     "high precision",
			json:     `{"id": "C1", "last_read": "0", "latest": "1737676800.999999", "mention_count": 0, "has_unreads": false}`,
			expected: "1737676800.999999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var snapshot ChannelSnapshot
			err := json.Unmarshal([]byte(tt.json), &snapshot)
			if err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if snapshot.Latest != tt.expected {
				t.Errorf("expected Latest %q, got %q", tt.expected, snapshot.Latest)
			}
		})
	}
}
