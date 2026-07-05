package export

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"iter"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	rslack "github.com/rusq/slack"
	"github.com/rusq/slackdump/v4/source"
)

// ArchiveMessageSource is the subset of slackdump/v4/source used by the renderer.
type ArchiveMessageSource interface {
	Channels(context.Context) ([]rslack.Channel, error)
	Users(context.Context) ([]rslack.User, error)
	AllMessages(context.Context, string) (iter.Seq2[rslack.Message, error], error)
	AllThreadMessages(context.Context, string, string) (iter.Seq2[rslack.Message, error], error)
}

type ArchiveSourceCloser interface {
	ArchiveMessageSource
	Close() error
}

type RenderRequest struct {
	Date        string
	Timezone    string
	ChannelID   string
	ChannelName string
}

// LoadArchiveSource opens a slackdump v4 archive database source.
func LoadArchiveSource(ctx context.Context, archiveDir string) (ArchiveSourceCloser, error) {
	src, err := source.Load(ctx, archiveDir)
	if err != nil {
		return nil, err
	}
	return src, nil
}

// RenderArchiveRange renders all channel files for an inclusive date range.
func RenderArchiveRange(ctx context.Context, archiveDir, outputDir, from, to, timezone string) (int, error) {
	return RenderArchiveRangeForChannels(ctx, archiveDir, outputDir, from, to, timezone, nil)
}

// RenderArchiveRangeForChannels renders selected channel files for an inclusive date range.
// A nil or empty channelIDs slice renders all channels in the archive.
func RenderArchiveRangeForChannels(
	ctx context.Context,
	archiveDir string,
	outputDir string,
	from string,
	to string,
	timezone string,
	channelIDs []string,
) (int, error) {
	src, err := LoadArchiveSource(ctx, archiveDir)
	if err != nil {
		return 0, fmt.Errorf("loading archive source: %w", err)
	}
	defer func() { _ = src.Close() }()

	channelNames, err := loadChannelNames(archiveDir)
	if err != nil {
		return 0, fmt.Errorf("loading channel names: %w", err)
	}
	return renderSourceRange(ctx, src, outputDir, from, to, timezone, channelNames, channelIDs)
}

// RenderSourceRange renders all channels from an already opened source.
func RenderSourceRange(
	ctx context.Context,
	src ArchiveMessageSource,
	outputDir string,
	from string,
	to string,
	timezone string,
) (int, error) {
	return renderSourceRange(ctx, src, outputDir, from, to, timezone, nil, nil)
}

func renderSourceRange(
	ctx context.Context,
	src ArchiveMessageSource,
	outputDir string,
	from string,
	to string,
	timezone string,
	channelNames channelNameResolver,
	channelIDs []string,
) (int, error) {
	channels, err := src.Channels(ctx)
	if err != nil {
		return 0, fmt.Errorf("loading channels: %w", err)
	}
	channels = filterRenderChannels(channels, channelIDs)

	dates, err := datesInRange(from, to, timezone)
	if err != nil {
		return 0, err
	}
	users, err := loadUsers(ctx, src)
	if err != nil {
		return 0, err
	}

	writes := 0
	for _, ch := range channels {
		messages, err := loadChannelMessages(ctx, src, ch.ID)
		if err != nil {
			return writes, fmt.Errorf("loading channel messages %s: %w", ch.ID, err)
		}
		threads := make(threadMessageCache)
		for _, date := range dates {
			name := channelNames.fileName(ch)
			content, err := renderChannelDateFromMessages(ctx, src, RenderRequest{
				Date:        date,
				Timezone:    timezone,
				ChannelID:   ch.ID,
				ChannelName: name,
			}, users, messages, threads)
			if err != nil {
				return writes, fmt.Errorf("rendering %s %s: %w", date, ch.ID, err)
			}
			if content == "" {
				continue
			}
			path := filepath.Join(outputDir, date, fmt.Sprintf("%s-%s.md", date, name))
			written, err := writeFileIfChanged(path, []byte(content))
			if err != nil {
				return writes, err
			}
			if written {
				writes++
			}
		}
	}
	return writes, nil
}

func filterRenderChannels(channels []rslack.Channel, channelIDs []string) []rslack.Channel {
	if len(channelIDs) == 0 {
		return channels
	}
	allowed := make(map[string]struct{}, len(channelIDs))
	for _, id := range channelIDs {
		allowed[id] = struct{}{}
	}
	filtered := make([]rslack.Channel, 0, len(channels))
	for _, ch := range channels {
		if _, ok := allowed[ch.ID]; ok {
			filtered = append(filtered, ch)
		}
	}
	return filtered
}

func writeFileIfChanged(path string, content []byte) (bool, error) {
	cleanPath := filepath.Clean(path)
	if existing, err := os.ReadFile(cleanPath); err == nil && bytes.Equal(existing, content) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0750); err != nil {
		return false, fmt.Errorf("creating output directory: %w", err)
	}
	if err := os.WriteFile(cleanPath, content, 0600); err != nil {
		return false, fmt.Errorf("writing %s: %w", cleanPath, err)
	}
	return true, nil
}

type userLookup map[string]rslack.User
type threadMessageCache map[string][]rslack.Message

type continuationBlock struct {
	parent     rslack.Message
	parentDate string
	replies    []rslack.Message
	firstReply time.Time
}

var mentionPattern = regexp.MustCompile(`<@([A-Z0-9]+)>`)

// RenderChannelDate renders one channel's markdown for one work day.
func RenderChannelDate(ctx context.Context, src ArchiveMessageSource, req RenderRequest) (string, error) {
	users, err := loadUsers(ctx, src)
	if err != nil {
		return "", err
	}
	return renderChannelDate(ctx, src, req, users)
}

func renderChannelDate(ctx context.Context, src ArchiveMessageSource, req RenderRequest, users userLookup) (string, error) {
	messages, err := loadChannelMessages(ctx, src, req.ChannelID)
	if err != nil {
		return "", err
	}
	return renderChannelDateFromMessages(ctx, src, req, users, messages, make(threadMessageCache))
}

func renderChannelDateFromMessages(
	ctx context.Context,
	src ArchiveMessageSource,
	req RenderRequest,
	users userLookup,
	messages []rslack.Message,
	threads threadMessageCache,
) (string, error) {
	if _, _, err := GetDateBounds(req.Date, req.Timezone); err != nil {
		return "", err
	}

	base, err := renderBaseSection(ctx, src, req, users, messages, threads)
	if err != nil {
		return "", err
	}
	continuations, err := renderContinuations(ctx, src, req, users, messages, threads)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	out.WriteString(base)
	if continuations != "" {
		if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n") {
			out.WriteByte('\n')
		}
		out.WriteString(continuations)
	}
	return out.String(), nil
}

func loadChannelMessages(ctx context.Context, src ArchiveMessageSource, channelID string) ([]rslack.Message, error) {
	messages, err := collectMessages(ctx, func() (iter.Seq2[rslack.Message, error], error) {
		return src.AllMessages(ctx, channelID)
	})
	if err != nil {
		return nil, err
	}
	sortMessages(messages)
	return messages, nil
}

func loadUsers(ctx context.Context, src ArchiveMessageSource) (userLookup, error) {
	users, err := src.Users(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading users: %w", err)
	}
	lookup := make(userLookup, len(users))
	for _, user := range users {
		lookup[user.ID] = user
	}
	return lookup, nil
}

func renderBaseSection(
	ctx context.Context,
	src ArchiveMessageSource,
	req RenderRequest,
	users userLookup,
	messages []rslack.Message,
	threads threadMessageCache,
) (string, error) {
	var out bytes.Buffer
	for _, msg := range messages {
		if !messageBelongsToDate(msg, req.Date, req.Timezone) {
			continue
		}
		writeMessage(&out, msg, "", users)
		if isThreadParent(msg) {
			if err := writeSameDayReplies(ctx, &out, src, req, users, msg, threads); err != nil {
				return "", err
			}
		}
	}
	return out.String(), nil
}

func writeSameDayReplies(
	ctx context.Context,
	out *bytes.Buffer,
	src ArchiveMessageSource,
	req RenderRequest,
	users userLookup,
	parent rslack.Message,
	threads threadMessageCache,
) error {
	thread, err := threads.get(ctx, src, req.ChannelID, parent.ThreadTimestamp)
	if err != nil {
		return err
	}
	for _, reply := range thread {
		if reply.Timestamp == parent.Timestamp || !messageBelongsToDate(reply, req.Date, req.Timezone) {
			continue
		}
		writeMessage(out, reply, "|   ", users)
	}
	return nil
}

func renderContinuations(
	ctx context.Context,
	src ArchiveMessageSource,
	req RenderRequest,
	users userLookup,
	messages []rslack.Message,
	threads threadMessageCache,
) (string, error) {
	var blocks []continuationBlock
	for _, parent := range messages {
		if !isThreadParent(parent) {
			continue
		}
		parentDate, err := messageWorkDate(parent, req.Timezone)
		if err != nil {
			return "", err
		}
		if parentDate >= req.Date {
			continue
		}
		block, ok, err := continuationForThread(ctx, src, req, parent, parentDate, threads)
		if err != nil {
			return "", err
		}
		if ok {
			blocks = append(blocks, block)
		}
	}
	if len(blocks) == 0 {
		return "", nil
	}
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].firstReply.Before(blocks[j].firstReply)
	})

	var out bytes.Buffer
	out.WriteString("---\n\n")
	out.WriteString("## Thread continuations\n")
	out.WriteString("Replies posted this day in threads started on earlier days.\n")
	out.WriteString("Lines marked [context] are repeated from the original day for readability.\n")
	for _, block := range blocks {
		fmt.Fprintf(
			&out,
			"\n### Thread started %s (see %s/%s-%s.md)\n",
			block.parentDate,
			block.parentDate,
			block.parentDate,
			req.ChannelName,
		)
		writeContextMessage(&out, block.parent, users)
		out.WriteByte('\n')
		for _, reply := range block.replies {
			writeMessage(&out, reply, "|   ", users)
		}
	}
	return out.String(), nil
}

func continuationForThread(
	ctx context.Context,
	src ArchiveMessageSource,
	req RenderRequest,
	parent rslack.Message,
	parentDate string,
	threads threadMessageCache,
) (continuationBlock, bool, error) {
	thread, err := threads.get(ctx, src, req.ChannelID, parent.ThreadTimestamp)
	if err != nil {
		return continuationBlock{}, false, err
	}

	var replies []rslack.Message
	var first time.Time
	for _, reply := range thread {
		if reply.Timestamp == parent.Timestamp || reply.SubType == rslack.MsgSubTypeThreadBroadcast {
			continue
		}
		if !messageBelongsToDate(reply, req.Date, req.Timezone) {
			continue
		}
		ts, err := parseSlackTimestamp(reply.Timestamp)
		if err != nil {
			return continuationBlock{}, false, err
		}
		if first.IsZero() {
			first = ts
		}
		replies = append(replies, reply)
	}
	if len(replies) == 0 {
		return continuationBlock{}, false, nil
	}
	return continuationBlock{parent: parent, parentDate: parentDate, replies: replies, firstReply: first}, true, nil
}

func (c threadMessageCache) get(
	ctx context.Context,
	src ArchiveMessageSource,
	channelID string,
	threadTS string,
) ([]rslack.Message, error) {
	key := channelID + "\x00" + threadTS
	if messages, ok := c[key]; ok {
		return messages, nil
	}
	messages, err := collectMessages(ctx, func() (iter.Seq2[rslack.Message, error], error) {
		return src.AllThreadMessages(ctx, channelID, threadTS)
	})
	if err != nil {
		return nil, err
	}
	sortMessages(messages)
	c[key] = messages
	return messages, nil
}

func collectMessages(
	ctx context.Context,
	open func() (iter.Seq2[rslack.Message, error], error),
) ([]rslack.Message, error) {
	seq, err := open()
	if err != nil {
		return nil, err
	}
	var messages []rslack.Message
	for msg, err := range seq {
		if err != nil {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		messages = append(messages, msg)
	}
	return messages, nil
}

func sortMessages(messages []rslack.Message) {
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp < messages[j].Timestamp
	})
}

func isThreadParent(msg rslack.Message) bool {
	return msg.ThreadTimestamp != "" && msg.ThreadTimestamp == msg.Timestamp && msg.ReplyCount > 0
}

func messageBelongsToDate(msg rslack.Message, date, timezone string) bool {
	workDate, err := messageWorkDate(msg, timezone)
	return err == nil && workDate == date
}

func messageWorkDate(msg rslack.Message, timezone string) (string, error) {
	ts, err := parseSlackTimestamp(msg.Timestamp)
	if err != nil {
		return "", err
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return "", err
	}
	local := ts.In(loc)
	if local.Hour() < 3 {
		local = local.AddDate(0, 0, -1)
	}
	return local.Format("2006-01-02"), nil
}

func parseSlackTimestamp(ts string) (time.Time, error) {
	parts := strings.SplitN(ts, ".", 2)
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing slack timestamp %q: %w", ts, err)
	}
	var nsec int64
	if len(parts) == 2 {
		frac := parts[1]
		if len(frac) > 9 {
			frac = frac[:9]
		}
		for len(frac) < 9 {
			frac += "0"
		}
		nsec, err = strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("parsing slack timestamp %q: %w", ts, err)
		}
	}
	return time.Unix(sec, nsec).UTC(), nil
}

func writeMessage(out *bytes.Buffer, msg rslack.Message, prefix string, users userLookup) {
	ts, err := parseSlackTimestamp(msg.Timestamp)
	if err != nil {
		return
	}
	fmt.Fprintf(out, "%s> %s [%s] @ %s:\n", prefix, senderName(msg, users), msg.User, ts.Format("02/01/2006 15:04:05 Z0700"))
	writeTextLines(out, prefix, resolveMentions(html.UnescapeString(msg.Text), users))
	out.WriteByte('\n')
}

func writeContextMessage(out *bytes.Buffer, msg rslack.Message, users userLookup) {
	var rendered bytes.Buffer
	writeMessage(&rendered, msg, "", users)
	for _, line := range strings.Split(strings.TrimRight(rendered.String(), "\n"), "\n") {
		out.WriteString("[context] ")
		out.WriteString(line)
		out.WriteByte('\n')
	}
}

func writeTextLines(out *bytes.Buffer, prefix, text string) {
	for _, line := range strings.Split(text, "\n") {
		out.WriteString(prefix)
		out.WriteString(line)
		out.WriteByte('\n')
	}
}

func resolveMentions(text string, users userLookup) string {
	return mentionPattern.ReplaceAllStringFunc(text, func(token string) string {
		matches := mentionPattern.FindStringSubmatch(token)
		if len(matches) != 2 {
			return token
		}
		return displayName(matches[1], users)
	})
}

func senderName(msg rslack.Message, users userLookup) string {
	if msg.User == "" && msg.Username != "" {
		return msg.Username
	}
	return displayName(msg.User, users)
}

func displayName(userID string, users userLookup) string {
	if userID == "" {
		return "unknown"
	}
	user, ok := users[userID]
	if !ok {
		return "<unknown>:" + userID
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
	return "<unknown>:" + userID
}
