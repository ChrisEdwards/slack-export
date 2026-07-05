package export

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	rslack "github.com/rusq/slack"
)

type memoryArchiveSource struct {
	channels []rslack.Channel
	users    []rslack.User
	messages map[string][]rslack.Message
	threads  map[string][]rslack.Message
}

func TestRenderSourceRange_WritesOnlyWhenContentChanges(t *testing.T) {
	src := memoryArchiveSource{
		channels: []rslack.Channel{{
			GroupConversation: rslack.GroupConversation{
				Conversation: rslack.Conversation{ID: "C123"},
				Name:         "engineering",
			},
		}},
		users: []rslack.User{{ID: "U1", Name: "alice", RealName: "Alice"}},
		messages: map[string][]rslack.Message{
			"C123": {
				{Msg: rslack.Msg{
					Type:      "message",
					User:      "U1",
					Text:      "Stable content",
					Timestamp: "1783094460.000000",
				}},
			},
		},
	}
	outputDir := t.TempDir()

	writes, err := RenderSourceRange(context.Background(), src, outputDir, "2026-07-03", "2026-07-03", "America/Chicago")
	if err != nil {
		t.Fatalf("RenderSourceRange() first run error = %v", err)
	}
	if writes != 1 {
		t.Fatalf("first run writes = %d, want 1", writes)
	}
	outPath := filepath.Join(outputDir, "2026-07-03", "2026-07-03-engineering.md")
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat rendered file: %v", err)
	}
	firstMod := info.ModTime()

	time.Sleep(10 * time.Millisecond)
	writes, err = RenderSourceRange(context.Background(), src, outputDir, "2026-07-03", "2026-07-03", "America/Chicago")
	if err != nil {
		t.Fatalf("RenderSourceRange() second run error = %v", err)
	}
	if writes != 0 {
		t.Fatalf("second run writes = %d, want 0", writes)
	}
	info, err = os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat rendered file after second run: %v", err)
	}
	if !info.ModTime().Equal(firstMod) {
		t.Errorf("mtime changed on identical render: got %s want %s", info.ModTime(), firstMod)
	}
}

func TestRenderSourceRange_UsesChannelNameOverrideWhenArchiveNameMissing(t *testing.T) {
	src := memoryArchiveSource{
		channels: []rslack.Channel{{
			GroupConversation: rslack.GroupConversation{
				Conversation: rslack.Conversation{ID: "C123"},
			},
		}},
		users: []rslack.User{{ID: "U1", Name: "alice", RealName: "Alice"}},
		messages: map[string][]rslack.Message{
			"C123": {
				{Msg: rslack.Msg{
					Type:      "message",
					User:      "U1",
					Text:      "Readable filename",
					Timestamp: "1783094460.000000",
				}},
			},
		},
	}
	outputDir := t.TempDir()

	writes, err := RenderSourceRangeWithChannelNames(
		context.Background(),
		src,
		outputDir,
		"2026-07-03",
		"2026-07-03",
		"America/Chicago",
		map[string]string{"C123": "engineering"},
	)
	if err != nil {
		t.Fatalf("RenderSourceRangeWithChannelNames() error = %v", err)
	}
	if writes != 1 {
		t.Fatalf("writes = %d, want 1", writes)
	}

	outPath := filepath.Join(outputDir, "2026-07-03", "2026-07-03-engineering.md")
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("stat rendered file with channel name: %v", err)
	}
	idPath := filepath.Join(outputDir, "2026-07-03", "2026-07-03-C123.md")
	if _, err := os.Stat(idPath); !os.IsNotExist(err) {
		t.Fatalf("ID-named file should not be written, stat err = %v", err)
	}
}

func TestRenderSourceRange_CanRenderOnlySelectedChannels(t *testing.T) {
	src := memoryArchiveSource{
		channels: []rslack.Channel{
			{
				GroupConversation: rslack.GroupConversation{
					Conversation: rslack.Conversation{ID: "C_SKIP"},
					Name:         "operations-alarms",
				},
			},
			{
				GroupConversation: rslack.GroupConversation{
					Conversation: rslack.Conversation{ID: "C_KEEP"},
					Name:         "constellation",
				},
			},
		},
		users: []rslack.User{{ID: "U1", Name: "alice", RealName: "Alice"}},
		messages: map[string][]rslack.Message{
			"C_SKIP": {
				{Msg: rslack.Msg{
					Type:      "message",
					User:      "U1",
					Text:      "Noisy alert",
					Timestamp: "1783094460.000000",
				}},
			},
			"C_KEEP": {
				{Msg: rslack.Msg{
					Type:      "message",
					User:      "U1",
					Text:      "Useful work discussion",
					Timestamp: "1783094460.000000",
				}},
			},
		},
	}
	outputDir := t.TempDir()

	writes, err := renderSourceRange(
		context.Background(),
		src,
		outputDir,
		"2026-07-03",
		"2026-07-03",
		"America/Chicago",
		nil,
		[]string{"C_KEEP"},
	)
	if err != nil {
		t.Fatalf("renderSourceRange() error = %v", err)
	}
	if writes != 1 {
		t.Fatalf("writes = %d, want 1", writes)
	}

	keepPath := filepath.Join(outputDir, "2026-07-03", "2026-07-03-constellation.md")
	if _, err := os.Stat(keepPath); err != nil {
		t.Fatalf("stat included channel file: %v", err)
	}
	skipPath := filepath.Join(outputDir, "2026-07-03", "2026-07-03-operations-alarms.md")
	if _, err := os.Stat(skipPath); !os.IsNotExist(err) {
		t.Fatalf("excluded channel file should not be written, stat err = %v", err)
	}
}

func TestRenderSourceRange_ReusesArchiveReadsAcrossDates(t *testing.T) {
	src := &countingArchiveSource{
		memoryArchiveSource: memoryArchiveSource{
			channels: []rslack.Channel{{
				GroupConversation: rslack.GroupConversation{
					Conversation: rslack.Conversation{ID: "C123"},
					Name:         "engineering",
				},
			}},
			users: []rslack.User{{ID: "U1", Name: "alice", RealName: "Alice"}},
			messages: map[string][]rslack.Message{
				"C123": {
					{Msg: rslack.Msg{
						Type:            "message",
						User:            "U1",
						Text:            "Thread parent",
						Timestamp:       "1782922930.000000",
						ThreadTimestamp: "1782922930.000000",
						ReplyCount:      1,
					}},
					{Msg: rslack.Msg{
						Type:      "message",
						User:      "U1",
						Text:      "Later date message",
						Timestamp: "1783094460.000000",
					}},
				},
			},
			threads: map[string][]rslack.Message{
				"C123:1782922930.000000": {
					{Msg: rslack.Msg{
						Type:            "message",
						User:            "U1",
						Text:            "Thread parent",
						Timestamp:       "1782922930.000000",
						ThreadTimestamp: "1782922930.000000",
						ReplyCount:      1,
					}},
					{Msg: rslack.Msg{
						Type:            "message",
						User:            "U1",
						Text:            "Late reply",
						Timestamp:       "1783098060.000000",
						ThreadTimestamp: "1782922930.000000",
					}},
				},
			},
		},
	}

	writes, err := RenderSourceRange(context.Background(), src, t.TempDir(), "2026-07-01", "2026-07-03", "America/Chicago")
	if err != nil {
		t.Fatalf("RenderSourceRange() error = %v", err)
	}
	if writes != 2 {
		t.Fatalf("writes = %d, want 2", writes)
	}
	if src.usersCalls != 1 {
		t.Fatalf("Users() calls = %d, want 1", src.usersCalls)
	}
	if src.messageCalls["C123"] != 1 {
		t.Fatalf("AllMessages(C123) calls = %d, want 1", src.messageCalls["C123"])
	}
	if src.threadCalls["C123:1782922930.000000"] != 1 {
		t.Fatalf("AllThreadMessages(parent) calls = %d, want 1", src.threadCalls["C123:1782922930.000000"])
	}
}

func (s memoryArchiveSource) Channels(context.Context) ([]rslack.Channel, error) {
	return s.channels, nil
}

func (s memoryArchiveSource) Users(context.Context) ([]rslack.User, error) {
	return s.users, nil
}

func (s memoryArchiveSource) AllMessages(_ context.Context, channelID string) (iter.Seq2[rslack.Message, error], error) {
	return seqMessages(s.messages[channelID]), nil
}

func (s memoryArchiveSource) AllThreadMessages(
	_ context.Context,
	channelID string,
	threadID string,
) (iter.Seq2[rslack.Message, error], error) {
	return seqMessages(s.threads[channelID+":"+threadID]), nil
}

type countingArchiveSource struct {
	memoryArchiveSource
	usersCalls   int
	messageCalls map[string]int
	threadCalls  map[string]int
}

func (s *countingArchiveSource) Users(ctx context.Context) ([]rslack.User, error) {
	s.usersCalls++
	return s.memoryArchiveSource.Users(ctx)
}

func (s *countingArchiveSource) AllMessages(ctx context.Context, channelID string) (iter.Seq2[rslack.Message, error], error) {
	if s.messageCalls == nil {
		s.messageCalls = make(map[string]int)
	}
	s.messageCalls[channelID]++
	return s.memoryArchiveSource.AllMessages(ctx, channelID)
}

func (s *countingArchiveSource) AllThreadMessages(
	ctx context.Context,
	channelID string,
	threadID string,
) (iter.Seq2[rslack.Message, error], error) {
	if s.threadCalls == nil {
		s.threadCalls = make(map[string]int)
	}
	s.threadCalls[channelID+":"+threadID]++
	return s.memoryArchiveSource.AllThreadMessages(ctx, channelID, threadID)
}

func seqMessages(messages []rslack.Message) iter.Seq2[rslack.Message, error] {
	return func(yield func(rslack.Message, error) bool) {
		for _, msg := range messages {
			if !yield(msg, nil) {
				return
			}
		}
	}
}

func TestRenderChannelDate_IncludesLateReplyInContinuationSection(t *testing.T) {
	src := memoryArchiveSource{
		channels: []rslack.Channel{{
			GroupConversation: rslack.GroupConversation{
				Conversation: rslack.Conversation{ID: "C123"},
				Name:         "engineering",
			},
		}},
		users: []rslack.User{
			{ID: "U1", Name: "alice", RealName: "Alice"},
			{ID: "U2", Name: "bob", RealName: "Bob"},
		},
		messages: map[string][]rslack.Message{
			"C123": {
				{Msg: rslack.Msg{
					Type:            "message",
					User:            "U1",
					Text:            "Parent text with <@U2>",
					Timestamp:       "1782922930.000000",
					ThreadTimestamp: "1782922930.000000",
					ReplyCount:      2,
				}},
				{Msg: rslack.Msg{
					Type:      "message",
					User:      "U2",
					Text:      "A normal message on the reply day",
					Timestamp: "1783094460.000000",
				}},
			},
		},
		threads: map[string][]rslack.Message{
			"C123:1782922930.000000": {
				{Msg: rslack.Msg{
					Type:            "message",
					User:            "U1",
					Text:            "Parent text with <@U2>",
					Timestamp:       "1782922930.000000",
					ThreadTimestamp: "1782922930.000000",
					ReplyCount:      2,
				}},
				{Msg: rslack.Msg{
					Type:            "message",
					User:            "U2",
					Text:            "Same-day reply stays nested with parent",
					Timestamp:       "1782923230.000000",
					ThreadTimestamp: "1782922930.000000",
				}},
				{Msg: rslack.Msg{
					Type:            "message",
					User:            "U2",
					Text:            "Late reply belongs to reply day",
					Timestamp:       "1783098060.000000",
					ThreadTimestamp: "1782922930.000000",
				}},
				{Msg: rslack.Msg{
					Type:            "message",
					SubType:         rslack.MsgSubTypeThreadBroadcast,
					User:            "U2",
					Text:            "Broadcast reply should not be duplicated",
					Timestamp:       "1783098120.000000",
					ThreadTimestamp: "1782922930.000000",
				}},
			},
		},
	}

	got, err := RenderChannelDate(context.Background(), src, RenderRequest{
		Date:        "2026-07-03",
		Timezone:    "America/Chicago",
		ChannelID:   "C123",
		ChannelName: "engineering",
	})
	if err != nil {
		t.Fatalf("RenderChannelDate() error = %v", err)
	}

	for _, want := range []string{
		"> Bob [U2] @ 03/07/2026 16:01:00 Z:",
		"A normal message on the reply day",
		"## Thread continuations",
		"### Thread started 2026-07-01 (see 2026-07-01/2026-07-01-engineering.md)",
		"[context] > Alice [U1] @ 01/07/2026 16:22:10 Z:",
		"[context] Parent text with Bob",
		"|   > Bob [U2] @ 03/07/2026 17:01:00 Z:",
		"|   Late reply belongs to reply day",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered output missing %q:\n%s", want, got)
		}
	}

	for _, notWant := range []string{
		"Same-day reply stays nested with parent",
		"Broadcast reply should not be duplicated",
	} {
		if strings.Contains(got, notWant) {
			t.Errorf("rendered output should not contain %q:\n%s", notWant, got)
		}
	}
}

func TestRenderChannelDate_UsesWorkdayBoundaryForContinuations(t *testing.T) {
	src := memoryArchiveSource{
		channels: []rslack.Channel{{
			GroupConversation: rslack.GroupConversation{
				Conversation: rslack.Conversation{ID: "C123"},
				Name:         "engineering",
			},
		}},
		users: []rslack.User{
			{ID: "U1", Name: "alice", RealName: "Alice"},
			{ID: "U2", Name: "bob", RealName: "Bob"},
		},
		messages: map[string][]rslack.Message{
			"C123": {
				{Msg: rslack.Msg{
					Type:            "message",
					User:            "U1",
					Text:            "Parent before boundary",
					Timestamp:       "1782922930.000000",
					ThreadTimestamp: "1782922930.000000",
					ReplyCount:      1,
				}},
			},
		},
		threads: map[string][]rslack.Message{
			"C123:1782922930.000000": {
				{Msg: rslack.Msg{
					Type:            "message",
					User:            "U1",
					Text:            "Parent before boundary",
					Timestamp:       "1782922930.000000",
					ThreadTimestamp: "1782922930.000000",
					ReplyCount:      1,
				}},
				{Msg: rslack.Msg{
					Type:            "message",
					User:            "U2",
					Text:            "Reply at 2:30am local is previous work day",
					Timestamp:       "1783063800.000000",
					ThreadTimestamp: "1782922930.000000",
				}},
			},
		},
	}

	got, err := RenderChannelDate(context.Background(), src, RenderRequest{
		Date:        "2026-07-02",
		Timezone:    "America/Chicago",
		ChannelID:   "C123",
		ChannelName: "engineering",
	})
	if err != nil {
		t.Fatalf("RenderChannelDate() error = %v", err)
	}
	if !strings.Contains(got, "Reply at 2:30am local is previous work day") {
		t.Errorf("rendered output should include 2:30am local reply in previous work day:\n%s", got)
	}
}
