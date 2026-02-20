package backend

import (
	"fmt"
	"net/url"
	"strings"
)

// validAudioSources is the whitelist for AudioSourcePriority values.
var validAudioSources = map[string]bool{
	"tidal":  true,
	"qobuz":  true,
	"amazon": true,
	"deezer": true,
}

// systemPaths are directories that must never be used as output.
var systemPaths = []string{"/etc", "/root", "/proc", "/sys", "/bin", "/sbin", "/usr/bin", "/dev", "/boot"}

// ValidateYouTubeURL checks that a URL is a valid YouTube URL.
// It must use https, come from an approved domain, and be ≤2048 chars.
func ValidateYouTubeURL(rawURL string) error {
	if len(rawURL) > 2048 {
		return fmt.Errorf("URL exceeds maximum length of 2048 characters")
	}

	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format")
	}

	if u.Scheme != "https" {
		return fmt.Errorf("URL must use https")
	}

	host := strings.ToLower(u.Hostname())
	switch host {
	case "youtube.com", "www.youtube.com",
		"youtu.be",
		"music.youtube.com":
		// allowed
	default:
		return fmt.Errorf("URL must be from youtube.com, youtu.be, or music.youtube.com")
	}

	return nil
}

// SanitizePlaylistName removes characters that could cause path issues.
func SanitizePlaylistName(name string) (string, error) {
	if strings.Contains(name, "\x00") {
		return "", fmt.Errorf("playlist name contains null bytes")
	}

	// Remove traversal sequences and path separators
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")

	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("playlist name is empty after sanitization")
	}

	return name, nil
}

// ValidateOutputDirectory rejects paths that overlap with system directories.
func ValidateOutputDirectory(path string) error {
	if path == "" {
		return nil // empty means "use default", which is always safe
	}

	for _, sys := range systemPaths {
		if path == sys || strings.HasPrefix(path, sys+"/") {
			return fmt.Errorf("output directory cannot be a system path (%s)", sys)
		}
	}

	return nil
}

// ValidateAudioSources checks that every entry in the slice is a known source.
// Values are case-sensitive; only lowercase names are accepted.
func ValidateAudioSources(sources []string) error {
	for _, s := range sources {
		if !validAudioSources[s] {
			return fmt.Errorf("unknown audio source %q: must be one of tidal, qobuz, amazon, deezer", s)
		}
	}
	return nil
}

// ValidateTrackURL validates a music service URL before exec.Command usage.
// Only https URLs are permitted.
func ValidateTrackURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid track URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("track URL must use https, got %q", u.Scheme)
	}
	return nil
}
