package backend

import (
	"fmt"
	"regexp"
)

// Tidal FLAC download - Primary audio source

// TidalTrackInfo contains Tidal-specific track metadata
type TidalTrackInfo struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	Artist       string  `json:"artist"`
	Album        string  `json:"album"`
	ISRC         string  `json:"isrc"`
	Duration     float64 `json:"duration"`
	Quality      string  `json:"quality"` // e.g., "LOSSLESS", "HI_RES", "HI_RES_LOSSLESS"
	CoverURL     string  `json:"coverUrl,omitempty"`
	TrackNumber  int     `json:"trackNumber,omitempty"`
	AlbumID      string  `json:"albumId,omitempty"`
	ReleaseDate  string  `json:"releaseDate,omitempty"`
	ExplicitFlag bool    `json:"explicit,omitempty"`
}

// Tidal URL patterns
var (
	tidalTrackRegex   = regexp.MustCompile(`tidal\.com/(?:browse/)?track/(\d+)`)
	tidalAlbumRegex   = regexp.MustCompile(`tidal\.com/(?:browse/)?album/(\d+)`)
	tidalPlaylistRegex = regexp.MustCompile(`tidal\.com/(?:browse/)?playlist/([a-f0-9-]+)`)
)

// ParseTidalURL extracts track/album ID from Tidal URL
func ParseTidalURL(rawURL string) (id string, contentType string, err error) {
	if matches := tidalTrackRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "track", nil
	}

	if matches := tidalAlbumRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "album", nil
	}

	if matches := tidalPlaylistRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "playlist", nil
	}

	return "", "", fmt.Errorf("could not parse Tidal URL: %s", rawURL)
}

// IsTidalURL checks if URL is a Tidal URL
func IsTidalURL(rawURL string) bool {
	return tidalTrackRegex.MatchString(rawURL) ||
		tidalAlbumRegex.MatchString(rawURL) ||
		tidalPlaylistRegex.MatchString(rawURL)
}

// GetTidalTrackInfo fetches track info from Tidal URL using unified downloader
func GetTidalTrackInfo(trackURL string) (*TidalTrackInfo, error) {
	downloader := NewUnifiedAudioDownloader(nil)
	info, err := downloader.GetTrackInfo(trackURL)
	if err != nil {
		return nil, err
	}

	return &TidalTrackInfo{
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

// DownloadTidalFLAC downloads FLAC from Tidal track URL
func DownloadTidalFLAC(trackURL string, outputDir string) (*AudioDownloadResult, error) {
	if !IsTidalURL(trackURL) {
		return nil, fmt.Errorf("not a valid Tidal URL: %s", trackURL)
	}

	config := DefaultDownloadConfig()
	config.OutputDir = outputDir
	config.PreferredFormat = "flac"

	downloader := NewUnifiedAudioDownloader(config)
	return downloader.DownloadFromURL(trackURL)
}

// SearchTidalByISRC finds a track on Tidal using ISRC
// Uses song.link to resolve ISRC to Tidal URL
func SearchTidalByISRC(isrc string) (*TidalTrackInfo, error) {
	// Use song.link to resolve ISRC
	info, err := GetPlatformURLsByISRC(isrc)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ISRC: %w", err)
	}

	if info.URLs.TidalURL == "" {
		return nil, fmt.Errorf("track not found on Tidal for ISRC: %s", isrc)
	}

	return GetTidalTrackInfo(info.URLs.TidalURL)
}

// GetTidalURLFromSpotify resolves Spotify URL to Tidal URL
func GetTidalURLFromSpotify(spotifyURL string) (string, error) {
	info, err := ResolveSpotifyURL(spotifyURL)
	if err != nil {
		return "", err
	}

	if info.URLs.TidalURL == "" {
		return "", fmt.Errorf("track not available on Tidal")
	}

	return info.URLs.TidalURL, nil
}

// GetTidalPriority returns download priority (1 = highest)
// Tidal is our primary source for FLAC
func GetTidalPriority() int {
	return 1
}

// TidalQuality represents Tidal audio quality tiers
type TidalQuality string

const (
	TidalQualityLow      TidalQuality = "LOW"      // 96 kbps AAC
	TidalQualityHigh     TidalQuality = "HIGH"     // 320 kbps AAC
	TidalQualityLossless TidalQuality = "LOSSLESS" // 16-bit/44.1kHz FLAC
	TidalQualityHiRes    TidalQuality = "HI_RES"   // 24-bit/96kHz MQA
	TidalQualityMax      TidalQuality = "MAX"      // 24-bit/192kHz FLAC
)

// GetTidalQualityLabel returns human-readable quality label
func GetTidalQualityLabel(quality TidalQuality) string {
	switch quality {
	case TidalQualityLow:
		return "Low (96 kbps AAC)"
	case TidalQualityHigh:
		return "High (320 kbps AAC)"
	case TidalQualityLossless:
		return "Lossless (16-bit/44.1kHz FLAC)"
	case TidalQualityHiRes:
		return "Hi-Res (24-bit/96kHz MQA)"
	case TidalQualityMax:
		return "Max (24-bit/192kHz FLAC)"
	default:
		return string(quality)
	}
}
