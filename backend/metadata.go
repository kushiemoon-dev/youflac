package backend

import (
	"fmt"
)

// MKV metadata embedding helpers

// GetMKVMetadataArgs returns ffmpeg args for embedding metadata
func GetMKVMetadataArgs(metadata *Metadata) []string {
	args := []string{}

	if metadata.Title != "" {
		args = append(args, "-metadata", fmt.Sprintf("title=%s", metadata.Title))
	}
	if metadata.Artist != "" {
		args = append(args, "-metadata", fmt.Sprintf("artist=%s", metadata.Artist))
	}
	if metadata.Album != "" {
		args = append(args, "-metadata", fmt.Sprintf("album=%s", metadata.Album))
	}
	if metadata.Year > 0 {
		args = append(args, "-metadata", fmt.Sprintf("date=%d", metadata.Year))
	}
	if metadata.Genre != "" {
		args = append(args, "-metadata", fmt.Sprintf("genre=%s", metadata.Genre))
	}
	if metadata.ISRC != "" {
		args = append(args, "-metadata", fmt.Sprintf("isrc=%s", metadata.ISRC))
	}

	return args
}

