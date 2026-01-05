package backend

import (
	"fmt"
	"regexp"
)

// Amazon Music FLAC download - Tertiary audio source

// AmazonTrackInfo contains Amazon Music-specific track metadata
type AmazonTrackInfo struct {
	ID          string  `json:"id"`
	ASIN        string  `json:"asin,omitempty"` // Amazon Standard Identification Number
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	ISRC        string  `json:"isrc"`
	Duration    float64 `json:"duration"`
	Quality     string  `json:"quality"` // e.g., "HD", "ULTRA_HD"
	CoverURL    string  `json:"coverUrl,omitempty"`
	TrackNumber int     `json:"trackNumber,omitempty"`
	AlbumID     string  `json:"albumId,omitempty"`
	ReleaseDate string  `json:"releaseDate,omitempty"`
}

// Amazon Music URL patterns
var (
	amazonTrackRegex    = regexp.MustCompile(`music\.amazon\.[a-z.]+/(?:albums/[^/]+/)?([A-Z0-9]+)(?:\?trackAsin=([A-Z0-9]+))?`)
	amazonAlbumRegex    = regexp.MustCompile(`music\.amazon\.[a-z.]+/albums/([A-Z0-9]+)`)
	amazonPlaylistRegex = regexp.MustCompile(`music\.amazon\.[a-z.]+/playlists/([A-Z0-9]+)`)
	// Simplified pattern for direct track links
	amazonTrackRegexAlt = regexp.MustCompile(`amazon\.[a-z.]+/dp/([A-Z0-9]+)`)
)

// ParseAmazonURL extracts track/album ID from Amazon Music URL
func ParseAmazonURL(rawURL string) (id string, contentType string, err error) {
	// Check for track with trackAsin parameter
	if matches := amazonTrackRegex.FindStringSubmatch(rawURL); len(matches) > 2 && matches[2] != "" {
		return matches[2], "track", nil
	}

	// Check for album
	if matches := amazonAlbumRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "album", nil
	}

	// Check for track (general)
	if matches := amazonTrackRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "track", nil
	}

	// Check alternative format
	if matches := amazonTrackRegexAlt.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "track", nil
	}

	// Check for playlist
	if matches := amazonPlaylistRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "playlist", nil
	}

	return "", "", fmt.Errorf("could not parse Amazon Music URL: %s", rawURL)
}

// IsAmazonMusicURL checks if URL is an Amazon Music URL
func IsAmazonMusicURL(rawURL string) bool {
	return amazonTrackRegex.MatchString(rawURL) ||
		amazonAlbumRegex.MatchString(rawURL) ||
		amazonPlaylistRegex.MatchString(rawURL) ||
		amazonTrackRegexAlt.MatchString(rawURL)
}

// GetAmazonTrackInfo fetches track info from Amazon Music URL using unified downloader
func GetAmazonTrackInfo(trackURL string) (*AmazonTrackInfo, error) {
	downloader := NewUnifiedAudioDownloader(nil)
	info, err := downloader.GetTrackInfo(trackURL)
	if err != nil {
		return nil, err
	}

	return &AmazonTrackInfo{
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

// DownloadAmazonFLAC downloads FLAC from Amazon Music track URL
func DownloadAmazonFLAC(trackURL string, outputDir string) (*AudioDownloadResult, error) {
	if !IsAmazonMusicURL(trackURL) {
		return nil, fmt.Errorf("not a valid Amazon Music URL: %s", trackURL)
	}

	config := DefaultDownloadConfig()
	config.OutputDir = outputDir
	config.PreferredFormat = "flac"

	downloader := NewUnifiedAudioDownloader(config)
	return downloader.DownloadFromURL(trackURL)
}

// SearchAmazonByISRC finds a track on Amazon Music using ISRC
// Uses song.link to resolve ISRC to Amazon Music URL
func SearchAmazonByISRC(isrc string) (*AmazonTrackInfo, error) {
	// Use song.link to resolve ISRC
	info, err := GetPlatformURLsByISRC(isrc)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ISRC: %w", err)
	}

	if info.URLs.AmazonURL == "" {
		return nil, fmt.Errorf("track not found on Amazon Music for ISRC: %s", isrc)
	}

	return GetAmazonTrackInfo(info.URLs.AmazonURL)
}

// GetAmazonURLFromSpotify resolves Spotify URL to Amazon Music URL
func GetAmazonURLFromSpotify(spotifyURL string) (string, error) {
	info, err := ResolveSpotifyURL(spotifyURL)
	if err != nil {
		return "", err
	}

	if info.URLs.AmazonURL == "" {
		return "", fmt.Errorf("track not available on Amazon Music")
	}

	return info.URLs.AmazonURL, nil
}

// GetAmazonPriority returns download priority (3 = tertiary)
// Amazon Music is our last resort for FLAC
func GetAmazonPriority() int {
	return 3
}

// AmazonQuality represents Amazon Music audio quality tiers
type AmazonQuality string

const (
	AmazonQualitySD      AmazonQuality = "SD"       // Standard Definition
	AmazonQualityHD      AmazonQuality = "HD"       // 16-bit/44.1kHz FLAC
	AmazonQualityUltraHD AmazonQuality = "ULTRA_HD" // 24-bit/192kHz FLAC
)

// GetAmazonQualityLabel returns human-readable quality label
func GetAmazonQualityLabel(quality AmazonQuality) string {
	switch quality {
	case AmazonQualitySD:
		return "Standard Quality"
	case AmazonQualityHD:
		return "HD (16-bit/44.1kHz FLAC)"
	case AmazonQualityUltraHD:
		return "Ultra HD (24-bit/192kHz FLAC)"
	default:
		return string(quality)
	}
}
