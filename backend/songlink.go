package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"time"
)

// song.link (Odesli) API integration for cross-platform URL resolution
// API Docs: https://odesli.co/

const (
	songLinkAPIBase = "https://api.song.link/v1-alpha.1/links"
	userAgent       = "MKV-Video/1.0"
)

// SongLinkResponse represents the full API response from song.link
type SongLinkResponse struct {
	EntityUniqueID string                      `json:"entityUniqueId"`
	UserCountry    string                      `json:"userCountry"`
	PageURL        string                      `json:"pageUrl"`
	LinksByPlatform map[string]PlatformLink    `json:"linksByPlatform"`
	EntitiesByUniqueID map[string]EntityInfo   `json:"entitiesByUniqueId"`
}

// PlatformLink contains the URL and entity ID for a platform
type PlatformLink struct {
	URL              string `json:"url"`
	NativeAppURIDesktop string `json:"nativeAppUriDesktop,omitempty"`
	NativeAppURIMobile  string `json:"nativeAppUriMobile,omitempty"`
	EntityUniqueID   string `json:"entityUniqueId"`
}

// EntityInfo contains metadata about a track/album
type EntityInfo struct {
	ID               string   `json:"id"`
	Type             string   `json:"type"` // "song" or "album"
	Title            string   `json:"title"`
	ArtistName       string   `json:"artistName"`
	ThumbnailURL     string   `json:"thumbnailUrl"`
	ThumbnailWidth   int      `json:"thumbnailWidth"`
	ThumbnailHeight  int      `json:"thumbnailHeight"`
	APIProvider      string   `json:"apiProvider"`
	Platforms        []string `json:"platforms"`
}

// SongLinkURLs contains resolved URLs for all supported platforms
type SongLinkURLs struct {
	SpotifyURL     string `json:"spotifyUrl,omitempty"`
	TidalURL       string `json:"tidalUrl,omitempty"`
	QobuzURL       string `json:"qobuzUrl,omitempty"`
	AmazonURL      string `json:"amazonUrl,omitempty"`
	DeezerURL      string `json:"deezerUrl,omitempty"`
	AppleMusicURL  string `json:"appleMusicUrl,omitempty"`
	YouTubeURL     string `json:"youtubeUrl,omitempty"`
	YouTubeMusicURL string `json:"youtubeMusicUrl,omitempty"`
	SoundCloudURL  string `json:"soundcloudUrl,omitempty"`
	PageURL        string `json:"pageUrl,omitempty"`
}

// SongLinkTrackInfo contains full metadata from song.link resolution
type SongLinkTrackInfo struct {
	Title       string       `json:"title"`
	Artist      string       `json:"artist"`
	Thumbnail   string       `json:"thumbnail"`
	Type        string       `json:"type"` // "song" or "album"
	URLs        SongLinkURLs `json:"urls"`
	ISRC        string       `json:"isrc,omitempty"`
	SpotifyID   string       `json:"spotifyId,omitempty"`
	TidalID     string       `json:"tidalId,omitempty"`
	QobuzID     string       `json:"qobuzId,omitempty"`
	AmazonID    string       `json:"amazonId,omitempty"`
}

// Rate limiting: song.link allows 10 requests/minute
var (
	lastRequest  time.Time
	requestMutex sync.Mutex
	minInterval  = 7 * time.Second // ~8.5 requests/min for safety
	httpClient   = &http.Client{
		Timeout: 30 * time.Second,
	}
)

// Spotify URL patterns
var (
	spotifyTrackRegex    = regexp.MustCompile(`spotify\.com/track/([a-zA-Z0-9]+)`)
	spotifyAlbumRegex    = regexp.MustCompile(`spotify\.com/album/([a-zA-Z0-9]+)`)
	spotifyPlaylistRegex = regexp.MustCompile(`spotify\.com/playlist/([a-zA-Z0-9]+)`)
	spotifyIntlRegex     = regexp.MustCompile(`spotify\.com/intl-[a-z]+/track/([a-zA-Z0-9]+)`)
)

// ParseSpotifyURL extracts track/album ID from Spotify URL
func ParseSpotifyURL(rawURL string) (id string, contentType string, err error) {
	// Try intl format first (spotify.com/intl-fr/track/...)
	if matches := spotifyIntlRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "track", nil
	}

	if matches := spotifyTrackRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "track", nil
	}

	if matches := spotifyAlbumRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "album", nil
	}

	if matches := spotifyPlaylistRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], "playlist", nil
	}

	return "", "", fmt.Errorf("could not parse Spotify URL: %s", rawURL)
}

// IsSpotifyURL checks if URL is a Spotify URL
func IsSpotifyURL(rawURL string) bool {
	return spotifyTrackRegex.MatchString(rawURL) ||
		spotifyAlbumRegex.MatchString(rawURL) ||
		spotifyPlaylistRegex.MatchString(rawURL) ||
		spotifyIntlRegex.MatchString(rawURL)
}

// waitForRateLimit ensures we don't exceed API limits
func waitForRateLimit() {
	requestMutex.Lock()
	defer requestMutex.Unlock()

	elapsed := time.Since(lastRequest)
	if elapsed < minInterval {
		time.Sleep(minInterval - elapsed)
	}
	lastRequest = time.Now()
}

// ResolveMusicURL converts any music platform URL to cross-platform URLs
// Supports: Spotify, Tidal, Qobuz, Apple Music, Deezer, YouTube Music, SoundCloud
func ResolveMusicURL(musicURL string) (*SongLinkTrackInfo, error) {
	waitForRateLimit()

	// Build API URL
	apiURL := fmt.Sprintf("%s?url=%s", songLinkAPIBase, url.QueryEscape(musicURL))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited by song.link API, please wait")
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response SongLinkResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return parseSongLinkResponse(&response), nil
}

// parseSongLinkResponse extracts useful info from the API response
func parseSongLinkResponse(resp *SongLinkResponse) *SongLinkTrackInfo {
	info := &SongLinkTrackInfo{
		URLs: SongLinkURLs{
			PageURL: resp.PageURL,
		},
	}

	// Extract URLs from each platform
	if link, ok := resp.LinksByPlatform["spotify"]; ok {
		info.URLs.SpotifyURL = link.URL
		info.SpotifyID = extractIDFromEntityUniqueID(link.EntityUniqueID)
	}
	if link, ok := resp.LinksByPlatform["tidal"]; ok {
		info.URLs.TidalURL = link.URL
		info.TidalID = extractIDFromEntityUniqueID(link.EntityUniqueID)
	}
	if link, ok := resp.LinksByPlatform["qobuz"]; ok {
		info.URLs.QobuzURL = link.URL
		info.QobuzID = extractIDFromEntityUniqueID(link.EntityUniqueID)
	}
	if link, ok := resp.LinksByPlatform["amazonMusic"]; ok {
		info.URLs.AmazonURL = link.URL
		info.AmazonID = extractIDFromEntityUniqueID(link.EntityUniqueID)
	}
	if link, ok := resp.LinksByPlatform["deezer"]; ok {
		info.URLs.DeezerURL = link.URL
	}
	if link, ok := resp.LinksByPlatform["appleMusic"]; ok {
		info.URLs.AppleMusicURL = link.URL
	}
	if link, ok := resp.LinksByPlatform["youtube"]; ok {
		info.URLs.YouTubeURL = link.URL
	}
	if link, ok := resp.LinksByPlatform["youtubeMusic"]; ok {
		info.URLs.YouTubeMusicURL = link.URL
	}
	if link, ok := resp.LinksByPlatform["soundcloud"]; ok {
		info.URLs.SoundCloudURL = link.URL
	}

	// Get metadata from the primary entity
	if entity, ok := resp.EntitiesByUniqueID[resp.EntityUniqueID]; ok {
		info.Title = entity.Title
		info.Artist = entity.ArtistName
		info.Thumbnail = entity.ThumbnailURL
		info.Type = entity.Type
	}

	return info
}

// extractIDFromEntityUniqueID extracts the ID from entity unique ID
// Format: "PROVIDER_SONG::ID" or "PROVIDER_ALBUM::ID"
func extractIDFromEntityUniqueID(entityID string) string {
	// Find the last "::" and take everything after it
	for i := len(entityID) - 1; i >= 1; i-- {
		if entityID[i] == ':' && entityID[i-1] == ':' {
			return entityID[i+1:]
		}
	}
	return entityID
}

// ResolveSpotifyURL converts a Spotify URL to platform-specific URLs (alias for ResolveMusicURL)
func ResolveSpotifyURL(spotifyURL string) (*SongLinkTrackInfo, error) {
	if !IsSpotifyURL(spotifyURL) {
		return nil, fmt.Errorf("not a valid Spotify URL: %s", spotifyURL)
	}
	return ResolveMusicURL(spotifyURL)
}

// GetPlatformURLsByISRC resolves platform URLs using ISRC
// Note: song.link doesn't support ISRC directly, but we can use Spotify's search
func GetPlatformURLsByISRC(isrc string) (*SongLinkTrackInfo, error) {
	// song.link supports ISRC via a special URL format
	isrcURL := fmt.Sprintf("https://open.spotify.com/search/isrc:%s", isrc)
	return ResolveMusicURL(isrcURL)
}

// GetBestFLACSource returns the best available FLAC source URL in priority order
// Priority: Tidal > Qobuz > Amazon
func GetBestFLACSource(info *SongLinkTrackInfo) (platform string, trackURL string) {
	if info.URLs.TidalURL != "" {
		return "tidal", info.URLs.TidalURL
	}
	if info.URLs.QobuzURL != "" {
		return "qobuz", info.URLs.QobuzURL
	}
	if info.URLs.AmazonURL != "" {
		return "amazon", info.URLs.AmazonURL
	}
	return "", ""
}

// GetAllFLACSources returns all available FLAC sources sorted by priority
func GetAllFLACSources(info *SongLinkTrackInfo) []struct {
	Platform string
	URL      string
	Priority int
} {
	var sources []struct {
		Platform string
		URL      string
		Priority int
	}

	if info.URLs.TidalURL != "" {
		sources = append(sources, struct {
			Platform string
			URL      string
			Priority int
		}{"tidal", info.URLs.TidalURL, 1})
	}
	if info.URLs.QobuzURL != "" {
		sources = append(sources, struct {
			Platform string
			URL      string
			Priority int
		}{"qobuz", info.URLs.QobuzURL, 2})
	}
	if info.URLs.AmazonURL != "" {
		sources = append(sources, struct {
			Platform string
			URL      string
			Priority int
		}{"amazon", info.URLs.AmazonURL, 3})
	}
	if info.URLs.DeezerURL != "" {
		sources = append(sources, struct {
			Platform string
			URL      string
			Priority int
		}{"deezer", info.URLs.DeezerURL, 4})
	}

	return sources
}
