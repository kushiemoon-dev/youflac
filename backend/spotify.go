package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// Spotify metadata extraction
// Note: URL parsing is handled by songlink.go (ParseSpotifyURL, IsSpotifyURL)

type SpotifyTrackInfo struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	ISRC        string  `json:"isrc"`
	Duration    float64 `json:"duration"`
	CoverURL    string  `json:"coverUrl"`
	ReleaseDate string  `json:"releaseDate,omitempty"`
	TrackNumber int     `json:"trackNumber,omitempty"`
}

// SpotifyEmbed response from embed API
type spotifyEmbedResponse struct {
	Title        string `json:"title"`
	ThumbnailURL string `json:"thumbnail_url"`
	HTML         string `json:"html"`
}

// GetSpotifyTrackInfo fetches track metadata from Spotify embed API (no auth required)
func GetSpotifyTrackInfo(trackID string) (*SpotifyTrackInfo, error) {
	// Use embed API which doesn't require authentication
	embedURL := fmt.Sprintf("https://open.spotify.com/oembed?url=https://open.spotify.com/track/%s", trackID)

	resp, err := http.Get(embedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Spotify embed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Spotify embed API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var embed spotifyEmbedResponse
	if err := json.Unmarshal(body, &embed); err != nil {
		return nil, fmt.Errorf("failed to parse embed response: %w", err)
	}

	// Parse title - usually "Song Name - Artist Name" or "Song Name"
	title, artist := parseSpotifyTitle(embed.Title)

	return &SpotifyTrackInfo{
		ID:       trackID,
		Title:    title,
		Artist:   artist,
		CoverURL: embed.ThumbnailURL,
	}, nil
}

// parseSpotifyTitle extracts song title and artist from embed title
func parseSpotifyTitle(fullTitle string) (title, artist string) {
	// Spotify embed titles are usually "Song Name - Artist Name"
	parts := strings.SplitN(fullTitle, " - ", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return fullTitle, ""
}

// GetSpotifyTrackInfoFromURL extracts track ID and fetches metadata
func GetSpotifyTrackInfoFromURL(spotifyURL string) (*SpotifyTrackInfo, error) {
	id, contentType, err := ParseSpotifyURL(spotifyURL)
	if err != nil {
		return nil, err
	}

	if contentType != "track" {
		return nil, fmt.Errorf("URL is not a track URL (got %s)", contentType)
	}

	return GetSpotifyTrackInfo(id)
}

// ExtractSpotifyID extracts the track ID from various Spotify URL formats
// This is a lightweight version that just extracts the ID without fetching metadata
func ExtractSpotifyID(rawURL string) (string, error) {
	id, _, err := ParseSpotifyURL(rawURL)
	return id, err
}

// GetISRCFromSpotify retrieves ISRC for a track using song.link
// Note: Spotify embed API doesn't provide ISRC, so we use song.link
func GetISRCFromSpotify(trackID string) (string, error) {
	spotifyURL := fmt.Sprintf("https://open.spotify.com/track/%s", trackID)
	info, err := ResolveMusicURL(spotifyURL)
	if err != nil {
		return "", fmt.Errorf("failed to resolve track: %w", err)
	}
	return info.ISRC, nil
}

// SearchSpotifyTracks searches for tracks (requires song.link resolution)
// Returns basic track info without ISRC
func SearchSpotifyTracks(query string) ([]SpotifyTrackInfo, error) {
	// Note: Direct Spotify search requires OAuth
	// For now, this is a placeholder - users should paste URLs
	return nil, fmt.Errorf("direct Spotify search not implemented, please use track URLs")
}

// Spotify URI patterns
var spotifyURIRegex = regexp.MustCompile(`spotify:track:([a-zA-Z0-9]+)`)

// ParseSpotifyURI parses Spotify URI format (spotify:track:ID)
func ParseSpotifyURI(uri string) (string, error) {
	if matches := spotifyURIRegex.FindStringSubmatch(uri); len(matches) > 1 {
		return matches[1], nil
	}
	return "", fmt.Errorf("invalid Spotify URI: %s", uri)
}

// ConvertSpotifyURIToURL converts Spotify URI to URL
func ConvertSpotifyURIToURL(uri string) (string, error) {
	id, err := ParseSpotifyURI(uri)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://open.spotify.com/track/%s", id), nil
}
