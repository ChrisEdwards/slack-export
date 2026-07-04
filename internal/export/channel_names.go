package export

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	appslack "github.com/chrisedwards/slack-export/internal/slack"
	rslack "github.com/rusq/slack"
)

const channelNamesFilename = ".slack-export-channel-names.json"

type channelNamesData struct {
	Channels map[string]string `json:"channels"`
}

type channelNameResolver map[string]string

func RenderSourceRangeWithChannelNames(
	ctx context.Context,
	src ArchiveMessageSource,
	outputDir string,
	from string,
	to string,
	timezone string,
	channelNames map[string]string,
) (int, error) {
	return renderSourceRange(ctx, src, outputDir, from, to, timezone, channelNameResolver(channelNames))
}

func (r channelNameResolver) fileName(ch rslack.Channel) string {
	if name := strings.TrimSpace(r[ch.ID]); name != "" {
		return name
	}
	if name := strings.TrimSpace(ch.Name); name != "" && name != ch.ID {
		return name
	}
	if name := strings.TrimSpace(ch.NameNormalized); name != "" && name != ch.ID {
		return name
	}
	if name := strings.TrimSpace(ch.Name); name != "" {
		return name
	}
	return ch.ID
}

func saveChannelNames(archiveDir string, chans []appslack.Channel) error {
	names := make(map[string]string, len(chans))
	for _, ch := range chans {
		id := strings.TrimSpace(ch.ID)
		name := strings.TrimSpace(ch.Name)
		if id == "" || name == "" {
			continue
		}
		names[id] = name
	}
	if err := os.MkdirAll(archiveDir, 0750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(channelNamesData{Channels: names}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(channelNamesPath(archiveDir), data, 0600)
}

func loadChannelNames(archiveDir string) (map[string]string, error) {
	data, err := os.ReadFile(channelNamesPath(archiveDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var stored channelNamesData
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, err
	}
	return stored.Channels, nil
}

func channelNamesPath(archiveDir string) string {
	return filepath.Join(archiveDir, channelNamesFilename)
}
