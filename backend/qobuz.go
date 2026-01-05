package backend

import (
	"fmt"
	"regexp"
)

// Qobuz FLAC download - Secondary audio source

// QobuzTrackInfo contains Qobuz-specific track metadata
type QobuzTrackInfo struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Artist       string  `json:"artist"`
	Album        string  `json:"album"`
	ISRC         string  `json:"isrc"`
	Duration     float64 `json:"duration"`
	Quality      string  `json:"quality"` // e.g., "24-bit/96kHz", "24-bit/192kHz"
	CoverURL     string  `json:"coverUrl,omitempty"`
	TrackNumber  int     `json:"trackNumber,omitempty"`
	AlbumID      string  `json:"albumId,omitempty"`
	ReleaseDate  string  `json:"releaseDate,omitempty"`
	Label        string  `json:"label,omitempty"`
	Composer     string  `json:"composer,omitempty"`
	SampleRate   int     `json:"sampleRate,omitempty"`   // e.g., 96000, 192000
	BitDepth     int     `json:"bitDepth,omitempty"`     // e.g., 16, 24
}

// Qobuz URL patterns
var (
	qobuzTrackRegex    = regexp.MustCompile(`qobuz\.com/[a-z]{2}-[a-z]{2}/track/(\d+)`)
	qobuzAlbumRegex    = regexp.MustCompile(`qobuz\.com/[a-z]{2}-[a-z]{2}/album/[^/]+/(\d+)`)
	qobuzPlaylistRegex = regexp.MustCompile(`qobuz\.com/[a-z]{2}-[a-z]{2}/playlist/(\d+)`)
	// Alternative format without locale
	qobuzTrackRegexAlt = regexp.MustCompile(`qobuz\.com/track/(\d+)`)
	qobuzAlbumRegexAlt = regexp.MustCompile(`qobuz\.com/album/[^/]+/(\d+)`)
)

// ParseQobuzURL extracts track/album ID from Qobuz URL
func ParseQobuzURL(rawURL string) (id string, contentType string, err error) {
	// Try locale format first
	if matches := qobuzTrackRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "track", nil
	}
	if matches := qobuzTrackRegexAlt.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "track", nil
	}

	if matches := qobuzAlbumRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "album", nil
	}
	if matches := qobuzAlbumRegexAlt.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "album", nil
	}

	if matches := qobuzPlaylistRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "playlist", nil
	}

	return "", "", fmt.Errorf("could not parse Qobuz URL: %s", rawURL)
}

// IsQobuzURL checks if URL is a Qobuz URL
func IsQobuzURL(rawURL string) bool {
	return qobuzTrackRegex.MatchString(rawURL) ||
		qobuzTrackRegexAlt.MatchString(rawURL) ||
		qobuzAlbumRegex.MatchString(rawURL) ||
		qobuzAlbumRegexAlt.MatchString(rawURL) ||
		qobuzPlaylistRegex.MatchString(rawURL)
}

// GetQobuzTrackInfo fetches track info from Qobuz URL using unified downloader
func GetQobuzTrackInfo(trackURL string) (*QobuzTrackInfo, error) {
	downloader := NewUnifiedAudioDownloader(nil)
	info, err := downloader.GetTrackInfo(trackURL)
	if err != nil {
		return nil, err
	}

	return &QobuzTrackInfo{
		ID:          info.ID,
		Title:       info.Title,
		Artist:      info.Artist,
		Album:       info.Album,
		ISRC:        info.ISRC,
		Duration:    info.Duration,
		Quality:     info.Quality,
		CoverURL:    info.CoverURL,
		ReleaseDate: info.ReleaseDate,
		TrackNumber: info.TrackNumber,
	}, nil
}

// DownloadQobuzFLAC downloads FLAC from Qobuz track URL
func DownloadQobuzFLAC(trackURL string, outputDir string) (*AudioDownloadResult, error) {
	if !IsQobuzURL(trackURL) {
		return nil, fmt.Errorf("not a valid Qobuz URL: %s", trackURL)
	}

	config := DefaultDownloadConfig()
	config.OutputDir = outputDir
	config.PreferredFormat = "flac"

	downloader := NewUnifiedAudioDownloader(config)
	return downloader.DownloadFromURL(trackURL)
}

// SearchQobuzByISRC finds a track on Qobuz using ISRC
// Uses song.link to resolve ISRC to Qobuz URL
func SearchQobuzByISRC(isrc string) (*QobuzTrackInfo, error) {
	// Use song.link to resolve ISRC
	info, err := GetPlatformURLsByISRC(isrc)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ISRC: %w", err)
	}

	if info.URLs.QobuzURL == "" {
		return nil, fmt.Errorf("track not found on Qobuz for ISRC: %s", isrc)
	}

	return GetQobuzTrackInfo(info.URLs.QobuzURL)
}

// GetQobuzURLFromSpotify resolves Spotify URL to Qobuz URL
func GetQobuzURLFromSpotify(spotifyURL string) (string, error) {
	info, err := ResolveSpotifyURL(spotifyURL)
	if err != nil {
		return "", err
	}

	if info.URLs.QobuzURL == "" {
		return "", fmt.Errorf("track not available on Qobuz")
	}

	return info.URLs.QobuzURL, nil
}

// GetQobuzPriority returns download priority (2 = secondary)
// Qobuz is our fallback for FLAC when Tidal unavailable
func GetQobuzPriority() int {
	return 2
}

// QobuzQuality represents Qobuz audio quality tiers
type QobuzQuality string

const (
	QobuzQualityMP3_320  QobuzQuality = "MP3_320"   // 320 kbps MP3
	QobuzQualityCD       QobuzQuality = "CD"        // 16-bit/44.1kHz FLAC
	QobuzQualityHiRes96  QobuzQuality = "HIRES_96"  // 24-bit/96kHz FLAC
	QobuzQualityHiRes192 QobuzQuality = "HIRES_192" // 24-bit/192kHz FLAC
)

// GetQobuzQualityLabel returns human-readable quality label
func GetQobuzQualityLabel(quality QobuzQuality) string {
	switch quality {
	case QobuzQualityMP3_320:
		return "MP3 320 kbps"
	case QobuzQualityCD:
		return "CD Quality (16-bit/44.1kHz FLAC)"
	case QobuzQualityHiRes96:
		return "Hi-Res (24-bit/96kHz FLAC)"
	case QobuzQualityHiRes192:
		return "Hi-Res (24-bit/192kHz FLAC)"
	default:
		return string(quality)
	}
}

// ParseQobuzQualityFromString parses quality string (e.g., "24-bit/96kHz")
func ParseQobuzQualityFromString(qualityStr string) QobuzQuality {
	switch qualityStr {
	case "24-bit/192kHz", "24bit/192kHz", "192kHz":
		return QobuzQualityHiRes192
	case "24-bit/96kHz", "24bit/96kHz", "96kHz":
		return QobuzQualityHiRes96
	case "16-bit/44.1kHz", "CD", "44.1kHz":
		return QobuzQualityCD
	default:
		return QobuzQualityCD // Default to CD quality
	}
}
