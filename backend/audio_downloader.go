package backend

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// AudioDownloader provides a unified interface for downloading FLAC from various sources

// AudioTrackInfo contains common track metadata
type AudioTrackInfo struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	ISRC        string  `json:"isrc,omitempty"`
	Duration    float64 `json:"duration"`
	Quality     string  `json:"quality"` // e.g., "FLAC", "16-bit/44.1kHz", "24-bit/96kHz"
	Platform    string  `json:"platform"` // tidal, qobuz, amazon, deezer
	CoverURL    string  `json:"coverUrl,omitempty"`
	ReleaseDate string  `json:"releaseDate,omitempty"`
	TrackNumber int     `json:"trackNumber,omitempty"`
}

// AudioDownloadResult contains the result of a download
type AudioDownloadResult struct {
	FilePath string          `json:"filePath"`
	Track    *AudioTrackInfo `json:"track"`
	Format   string          `json:"format"` // flac, mp3, etc.
	Bitrate  int             `json:"bitrate,omitempty"`
	Size     int64           `json:"size"`
}

// AudioDownloadService defines the interface for download services
type AudioDownloadService interface {
	Name() string
	GetTrackInfo(trackURL string) (*AudioTrackInfo, error)
	Download(trackURL string, outputDir string, format string) (*AudioDownloadResult, error)
	SupportsFormat(format string) bool
	IsAvailable() bool
}

// DownloadConfig configures the download behavior
type DownloadConfig struct {
	PreferredFormat  string   // flac, mp3, wav
	PreferredQuality string   // highest, 24bit, 16bit
	PlatformPriority []string // Order of platform preference: tidal, qobuz, amazon, deezer
	OutputDir        string
	Timeout          time.Duration
}

// DefaultDownloadConfig returns sensible defaults
func DefaultDownloadConfig() *DownloadConfig {
	return &DownloadConfig{
		PreferredFormat:  "flac",
		PreferredQuality: "highest",
		PlatformPriority: []string{"tidal", "qobuz", "amazon", "deezer"},
		OutputDir:        os.TempDir(),
		Timeout:          5 * time.Minute,
	}
}

// ============================================================================
// Lucida.to Service Implementation
// Used by many music download tools including SpotiFLAC-style applications
// ============================================================================

const (
	lucidaAPIBase = "https://lucida.to"
	lucidaAPIPath = "/api/load"
)

// LucidaService implements AudioDownloadService using lucida.to
type LucidaService struct {
	client  *http.Client
	baseURL string
}

// LucidaResponse represents the API response from lucida.to
type LucidaResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Track   struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Artist      string  `json:"artist"`
		Album       string  `json:"album"`
		Duration    float64 `json:"duration"`
		CoverURL    string  `json:"cover"`
		ReleaseDate string  `json:"releaseDate"`
		ISRC        string  `json:"isrc"`
		Platform    string  `json:"platform"`
	} `json:"track"`
	Formats []struct {
		Format  string `json:"format"`
		Quality string `json:"quality"`
		Size    int64  `json:"size"`
		URL     string `json:"url"`
	} `json:"formats"`
}

// NewLucidaService creates a new Lucida download service
func NewLucidaService() *LucidaService {
	return &LucidaService{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: lucidaAPIBase,
	}
}

func (l *LucidaService) Name() string {
	return "lucida"
}

func (l *LucidaService) IsAvailable() bool {
	// Try to ping the service
	resp, err := l.client.Head(l.baseURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

func (l *LucidaService) SupportsFormat(format string) bool {
	supported := []string{"flac", "mp3", "wav", "aac", "ogg"}
	format = strings.ToLower(format)
	for _, f := range supported {
		if f == format {
			return true
		}
	}
	return false
}

func (l *LucidaService) GetTrackInfo(trackURL string) (*AudioTrackInfo, error) {
	resp, err := l.fetchTrackData(trackURL)
	if err != nil {
		return nil, err
	}

	return &AudioTrackInfo{
		ID:          resp.Track.ID,
		Title:       resp.Track.Title,
		Artist:      resp.Track.Artist,
		Album:       resp.Track.Album,
		Duration:    resp.Track.Duration,
		ISRC:        resp.Track.ISRC,
		Platform:    resp.Track.Platform,
		CoverURL:    resp.Track.CoverURL,
		ReleaseDate: resp.Track.ReleaseDate,
	}, nil
}

func (l *LucidaService) fetchTrackData(trackURL string) (*LucidaResponse, error) {
	// Build request
	apiURL := l.baseURL + lucidaAPIPath

	data := url.Values{}
	data.Set("url", trackURL)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Origin", "https://lucida.to")
	req.Header.Set("Referer", "https://lucida.to/")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result LucidaResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}

	return &result, nil
}

func (l *LucidaService) Download(trackURL string, outputDir string, format string) (*AudioDownloadResult, error) {
	resp, err := l.fetchTrackData(trackURL)
	if err != nil {
		return nil, err
	}

	// Find the requested format
	var downloadURL string
	var downloadFormat string
	var downloadSize int64
	var quality string

	format = strings.ToLower(format)

	// First try exact format match
	for _, f := range resp.Formats {
		if strings.ToLower(f.Format) == format {
			downloadURL = f.URL
			downloadFormat = f.Format
			downloadSize = f.Size
			quality = f.Quality
			break
		}
	}

	// If FLAC not available, fall back to best available
	if downloadURL == "" && format == "flac" {
		// Priority: flac > wav > mp3 320
		priorities := []string{"flac", "wav", "mp3"}
		for _, pf := range priorities {
			for _, f := range resp.Formats {
				if strings.ToLower(f.Format) == pf {
					downloadURL = f.URL
					downloadFormat = f.Format
					downloadSize = f.Size
					quality = f.Quality
					break
				}
			}
			if downloadURL != "" {
				break
			}
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("format %s not available for this track", format)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename
	safeTitle := SanitizeFileName(fmt.Sprintf("%s - %s", resp.Track.Artist, resp.Track.Title))
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.%s", safeTitle, strings.ToLower(downloadFormat)))

	// Download the file
	if err := l.downloadFile(downloadURL, outputPath); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Get actual file size
	stat, _ := os.Stat(outputPath)
	if stat != nil {
		downloadSize = stat.Size()
	}

	return &AudioDownloadResult{
		FilePath: outputPath,
		Track: &AudioTrackInfo{
			ID:          resp.Track.ID,
			Title:       resp.Track.Title,
			Artist:      resp.Track.Artist,
			Album:       resp.Track.Album,
			Duration:    resp.Track.Duration,
			ISRC:        resp.Track.ISRC,
			Platform:    resp.Track.Platform,
			CoverURL:    resp.Track.CoverURL,
			Quality:     quality,
			ReleaseDate: resp.Track.ReleaseDate,
		},
		Format: downloadFormat,
		Size:   downloadSize,
	}, nil
}

func (l *LucidaService) downloadFile(downloadURL, outputPath string) error {
	resp, err := l.client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download server returned %d", resp.StatusCode)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		os.Remove(outputPath) // Clean up partial file
		return fmt.Errorf("download interrupted: %w", err)
	}

	return nil
}

// ============================================================================
// Tidal HiFi API Service - vogel.qqdl.site (SpotiFLAC pattern)
// Free Tidal FLAC proxy without credentials
// ============================================================================

const (
	tidalHifiAPIBase = "https://vogel.qqdl.site"
)

// TidalHifiService implements AudioDownloadService using the hifi-api
type TidalHifiService struct {
	client  *http.Client
	baseURL string
}

// TidalManifest represents the decoded manifest from hifi-api
type TidalManifest struct {
	MimeType       string   `json:"mimeType"`
	Codecs         string   `json:"codecs"`
	EncryptionType string   `json:"encryptionType"`
	URLs           []string `json:"urls"`
}

// TidalTrackResponse represents the track info response
type TidalTrackResponse struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Duration    int    `json:"duration"`
	TrackNumber int    `json:"trackNumber"`
	ISRC        string `json:"isrc"`
	Explicit    bool   `json:"explicit"`
	Artist      struct {
		Name string `json:"name"`
	} `json:"artist"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Title string `json:"title"`
		Cover string `json:"cover"`
	} `json:"album"`
}

// TidalStreamResponse represents the stream/manifest response
type TidalStreamResponse struct {
	TrackID      int    `json:"trackId"`
	AssetID      int    `json:"assetId,omitempty"`
	AudioMode    string `json:"audioMode"`
	AudioQuality string `json:"audioQuality"`
	Manifest     string `json:"manifest"`        // Base64 encoded
	ManifestType string `json:"manifestMimeType"`
}

// TidalSearchResponse represents search results
type TidalSearchResponse struct {
	// Standard search endpoint format (version 2.0)
	Version string `json:"version,omitempty"`
	Data    struct {
		Items []TidalTrackResponse `json:"items"`
	} `json:"data,omitempty"`
	// Alternative format
	Tracks struct {
		Items []TidalTrackResponse `json:"items"`
	} `json:"tracks,omitempty"`
}

// NewTidalHifiService creates a new Tidal HiFi download service
func NewTidalHifiService() *TidalHifiService {
	return &TidalHifiService{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: tidalHifiAPIBase,
	}
}

func (t *TidalHifiService) Name() string {
	return "tidal-hifi"
}

func (t *TidalHifiService) IsAvailable() bool {
	// Try to ping the service
	resp, err := t.client.Head(t.baseURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

func (t *TidalHifiService) SupportsFormat(format string) bool {
	return strings.ToLower(format) == "flac"
}

// SearchTrack searches for a track on Tidal
func (t *TidalHifiService) SearchTrack(query string) (*TidalTrackResponse, error) {
	searchURL := fmt.Sprintf("%s/search/?s=%s", t.baseURL, url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	var searchResp TidalSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	// Check both possible formats (data.items for v2.0, tracks.items for older)
	var items []TidalTrackResponse
	if len(searchResp.Data.Items) > 0 {
		items = searchResp.Data.Items
	} else if len(searchResp.Tracks.Items) > 0 {
		items = searchResp.Tracks.Items
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no tracks found for query: %s", query)
	}

	return &items[0], nil
}

// TidalInfoResponse wraps track info with version
type TidalInfoResponse struct {
	Version string             `json:"version"`
	Data    TidalTrackResponse `json:"data"`
}

// TidalStreamDataResponse wraps stream response with version
type TidalStreamDataResponse struct {
	Version string              `json:"version"`
	Data    TidalStreamResponse `json:"data"`
}

// GetTrackByID fetches track info by Tidal ID
func (t *TidalHifiService) GetTrackByID(trackID int) (*TidalTrackResponse, error) {
	infoURL := fmt.Sprintf("%s/info/?id=%d", t.baseURL, trackID)

	req, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("info request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read info response: %w", err)
	}

	// Try v2.0 wrapper format first
	var infoResp TidalInfoResponse
	if err := json.Unmarshal(body, &infoResp); err != nil {
		return nil, fmt.Errorf("failed to parse track info: %w", err)
	}

	// Check if we got data from the wrapper
	if infoResp.Data.ID > 0 {
		return &infoResp.Data, nil
	}

	// Fallback: try direct format
	var trackInfo TidalTrackResponse
	if err := json.Unmarshal(body, &trackInfo); err != nil {
		return nil, fmt.Errorf("failed to parse track info (direct): %w", err)
	}

	return &trackInfo, nil
}

// GetStreamURL fetches the FLAC stream URL for a track
func (t *TidalHifiService) GetStreamURL(trackID int) (string, error) {
	streamURL := fmt.Sprintf("%s/track/?id=%d&quality=LOSSLESS", t.baseURL, trackID)

	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("stream request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read stream response: %w", err)
	}

	// Try v2.0 wrapper format first
	var streamDataResp TidalStreamDataResponse
	if err := json.Unmarshal(body, &streamDataResp); err != nil {
		return "", fmt.Errorf("failed to parse stream response: %w", err)
	}

	manifestBase64 := streamDataResp.Data.Manifest
	if manifestBase64 == "" {
		// Fallback: try direct format
		var streamResp TidalStreamResponse
		if err := json.Unmarshal(body, &streamResp); err != nil {
			return "", fmt.Errorf("failed to parse stream response (direct): %w", err)
		}
		manifestBase64 = streamResp.Manifest
	}

	if manifestBase64 == "" {
		return "", fmt.Errorf("no manifest in stream response")
	}

	// Decode base64 manifest
	manifestBytes, err := base64.StdEncoding.DecodeString(manifestBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode manifest: %w", err)
	}

	var manifest TidalManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return "", fmt.Errorf("failed to parse manifest: %w", err)
	}

	if len(manifest.URLs) == 0 {
		return "", fmt.Errorf("no download URLs in manifest")
	}

	return manifest.URLs[0], nil
}

// ExtractTidalID extracts the track ID from a Tidal URL
func ExtractTidalID(tidalURL string) (int, error) {
	// Patterns:
	// https://tidal.com/browse/track/12345678
	// https://listen.tidal.com/track/12345678
	// tidal:track:12345678

	patterns := []string{
		`tidal\.com/browse/track/(\d+)`,
		`listen\.tidal\.com/track/(\d+)`,
		`tidal:track:(\d+)`,
		`/track/(\d+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(tidalURL); len(matches) > 1 {
			var id int
			fmt.Sscanf(matches[1], "%d", &id)
			return id, nil
		}
	}

	return 0, fmt.Errorf("could not extract Tidal track ID from URL: %s", tidalURL)
}

func (t *TidalHifiService) GetTrackInfo(trackURL string) (*AudioTrackInfo, error) {
	trackID, err := ExtractTidalID(trackURL)
	if err != nil {
		return nil, err
	}

	track, err := t.GetTrackByID(trackID)
	if err != nil {
		return nil, err
	}

	artistName := track.Artist.Name
	if artistName == "" && len(track.Artists) > 0 {
		artistName = track.Artists[0].Name
	}

	return &AudioTrackInfo{
		ID:       fmt.Sprintf("%d", track.ID),
		Title:    track.Title,
		Artist:   artistName,
		Album:    track.Album.Title,
		ISRC:     track.ISRC,
		Duration: float64(track.Duration),
		Quality:  "FLAC 16-bit/44.1kHz",
		Platform: "tidal",
		CoverURL: fmt.Sprintf("https://resources.tidal.com/images/%s/640x640.jpg", strings.ReplaceAll(track.Album.Cover, "-", "/")),
	}, nil
}

func (t *TidalHifiService) Download(trackURL string, outputDir string, format string) (*AudioDownloadResult, error) {
	trackID, err := ExtractTidalID(trackURL)
	if err != nil {
		return nil, err
	}

	// Get track info
	track, err := t.GetTrackByID(trackID)
	if err != nil {
		return nil, fmt.Errorf("failed to get track info: %w", err)
	}

	// Get stream URL
	streamURL, err := t.GetStreamURL(trackID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream URL: %w", err)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename
	artistName := track.Artist.Name
	if artistName == "" && len(track.Artists) > 0 {
		artistName = track.Artists[0].Name
	}
	safeTitle := SanitizeFileName(fmt.Sprintf("%s - %s", artistName, track.Title))
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.flac", safeTitle))

	// Download the FLAC file
	if err := t.downloadFile(streamURL, outputPath); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Get actual file size
	stat, _ := os.Stat(outputPath)
	var fileSize int64
	if stat != nil {
		fileSize = stat.Size()
	}

	return &AudioDownloadResult{
		FilePath: outputPath,
		Track: &AudioTrackInfo{
			ID:       fmt.Sprintf("%d", track.ID),
			Title:    track.Title,
			Artist:   artistName,
			Album:    track.Album.Title,
			Duration: float64(track.Duration),
			ISRC:     track.ISRC,
			Platform: "tidal",
			Quality:  "FLAC LOSSLESS",
			CoverURL: fmt.Sprintf("https://resources.tidal.com/images/%s/640x640.jpg", strings.ReplaceAll(track.Album.Cover, "-", "/")),
		},
		Format: "flac",
		Size:   fileSize,
	}, nil
}

func (t *TidalHifiService) downloadFile(downloadURL, outputPath string) error {
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download server returned %d", resp.StatusCode)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		os.Remove(outputPath) // Clean up partial file
		return fmt.Errorf("download interrupted: %w", err)
	}

	return nil
}

// DownloadBySearch downloads FLAC by searching for artist + title
func (t *TidalHifiService) DownloadBySearch(artist, title, outputDir string) (*AudioDownloadResult, error) {
	query := fmt.Sprintf("%s %s", artist, title)

	track, err := t.SearchTrack(query)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Create a fake URL with the track ID
	trackURL := fmt.Sprintf("https://tidal.com/browse/track/%d", track.ID)

	return t.Download(trackURL, outputDir, "flac")
}

// ============================================================================
// Unified Downloader - Orchestrates multiple services
// ============================================================================

// UnifiedAudioDownloader coordinates downloads across multiple services
type UnifiedAudioDownloader struct {
	services []AudioDownloadService
	config   *DownloadConfig
}

// NewUnifiedAudioDownloader creates a downloader with all available services
func NewUnifiedAudioDownloader(config *DownloadConfig) *UnifiedAudioDownloader {
	if config == nil {
		config = DefaultDownloadConfig()
	}

	return &UnifiedAudioDownloader{
		services: []AudioDownloadService{
			NewLucidaService(),
			// Add more services here as they become available
		},
		config: config,
	}
}

// DownloadFromURL downloads audio from any supported music platform URL
func (u *UnifiedAudioDownloader) DownloadFromURL(musicURL string) (*AudioDownloadResult, error) {
	var lastErr error

	for _, service := range u.services {
		if !service.IsAvailable() {
			continue
		}

		result, err := service.Download(musicURL, u.config.OutputDir, u.config.PreferredFormat)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all services failed, last error: %w", lastErr)
	}
	return nil, fmt.Errorf("no download services available")
}

// DownloadFLAC downloads FLAC audio from the best available source
// using song.link resolution to find the track on streaming platforms
func (u *UnifiedAudioDownloader) DownloadFLAC(spotifyOrYouTubeURL string) (*AudioDownloadResult, error) {
	// First resolve to get platform URLs
	info, err := ResolveMusicURL(spotifyOrYouTubeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve URL: %w", err)
	}

	// Try each platform in priority order
	sources := GetAllFLACSources(info)
	if len(sources) == 0 {
		return nil, fmt.Errorf("no FLAC sources found for this track")
	}

	var lastErr error
	for _, source := range sources {
		result, err := u.DownloadFromURL(source.URL)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed to download from any source: %w", lastErr)
}

// GetTrackInfo gets metadata without downloading
func (u *UnifiedAudioDownloader) GetTrackInfo(musicURL string) (*AudioTrackInfo, error) {
	for _, service := range u.services {
		if !service.IsAvailable() {
			continue
		}

		info, err := service.GetTrackInfo(musicURL)
		if err == nil {
			return info, nil
		}
	}
	return nil, fmt.Errorf("no services could fetch track info")
}

// ============================================================================
// OrpheusDL Service - Python subprocess for high-quality FLAC
// Requires: Python 3 + orpheusdl installed (pip install orpheusdl)
// ============================================================================

// OrpheusDLService uses OrpheusDL Python tool as subprocess
type OrpheusDLService struct {
	pythonPath string
}

// NewOrpheusDLService creates a new OrpheusDL service
func NewOrpheusDLService() *OrpheusDLService {
	// Try to find python executable
	pythonPath := "python3"
	if _, err := exec.LookPath("python3"); err != nil {
		pythonPath = "python"
	}
	return &OrpheusDLService{
		pythonPath: pythonPath,
	}
}

func (o *OrpheusDLService) Name() string {
	return "orpheusdl"
}

func (o *OrpheusDLService) IsAvailable() bool {
	// Check if streamrip 'rip' command is available (installed via pipx)
	cmd := exec.Command("rip", "--version")
	if err := cmd.Run(); err == nil {
		return true
	}
	// Try python -m streamrip as fallback
	cmd2 := exec.Command(o.pythonPath, "-m", "streamrip", "--version")
	if err2 := cmd2.Run(); err2 == nil {
		return true
	}
	return false
}

func (o *OrpheusDLService) SupportsFormat(format string) bool {
	return strings.ToLower(format) == "flac"
}

func (o *OrpheusDLService) GetTrackInfo(trackURL string) (*AudioTrackInfo, error) {
	return nil, fmt.Errorf("orpheusdl does not support metadata-only queries")
}

func (o *OrpheusDLService) Download(trackURL string, outputDir string, format string) (*AudioDownloadResult, error) {
	// First try streamrip (more common)
	result, err := o.tryStreamrip(trackURL, outputDir)
	if err == nil {
		return result, nil
	}

	// Fallback to orpheusdl
	return o.tryOrpheusDL(trackURL, outputDir)
}

func (o *OrpheusDLService) tryStreamrip(trackURL string, outputDir string) (*AudioDownloadResult, error) {
	if err := ValidateTrackURL(trackURL); err != nil {
		return nil, fmt.Errorf("rejected track URL: %w", err)
	}

	// streamrip 'rip' command (installed via pipx): rip url <url>
	// Downloads to ~/music by default, we'll move it after

	// First try the 'rip' command directly (pipx install)
	cmd := exec.Command("rip", "url", trackURL)
	cmd.Dir = outputDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback to python -m streamrip
		cmd2 := exec.Command(o.pythonPath, "-m", "streamrip", "url", trackURL)
		cmd2.Dir = outputDir
		output, err = cmd2.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("streamrip failed: %w - %s", err, string(output))
		}
	}

	// Streamrip downloads to ~/music by default, search there too
	homeDir, _ := os.UserHomeDir()
	musicDir := filepath.Join(homeDir, "music")

	// Find the downloaded FLAC file (check both outputDir and ~/music)
	flacFile, err := o.findDownloadedFLAC(outputDir)
	if err != nil {
		// Try ~/music directory
		flacFile, err = o.findDownloadedFLAC(musicDir)
		if err != nil {
			return nil, fmt.Errorf("FLAC file not found after download: %w", err)
		}
	}

	return &AudioDownloadResult{
		FilePath: flacFile,
		Format:   "flac",
		Track: &AudioTrackInfo{
			Platform: "streamrip",
			Quality:  "FLAC",
		},
	}, nil
}

func (o *OrpheusDLService) tryOrpheusDL(trackURL string, outputDir string) (*AudioDownloadResult, error) {
	if err := ValidateTrackURL(trackURL); err != nil {
		return nil, fmt.Errorf("rejected track URL: %w", err)
	}

	// orpheusdl command varies by version, try common patterns
	cmd := exec.Command(o.pythonPath, "-m", "orpheusdl", trackURL, "-o", outputDir, "-q", "flac")
	cmd.Dir = outputDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("orpheusdl failed: %w - %s", err, string(output))
	}

	// Find the downloaded FLAC file
	flacFile, err := o.findDownloadedFLAC(outputDir)
	if err != nil {
		return nil, err
	}

	return &AudioDownloadResult{
		FilePath: flacFile,
		Format:   "flac",
		Track: &AudioTrackInfo{
			Platform: "orpheusdl",
			Quality:  "FLAC 24-bit",
		},
	}, nil
}

func (o *OrpheusDLService) findDownloadedFLAC(dir string) (string, error) {
	var flacFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".flac") {
			flacFiles = append(flacFiles, path)
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to search for FLAC: %w", err)
	}

	if len(flacFiles) == 0 {
		return "", fmt.Errorf("no FLAC file found in output directory")
	}

	// Return the most recently modified FLAC file
	var newestFile string
	var newestTime time.Time

	for _, f := range flacFiles {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if info.ModTime().After(newestTime) {
			newestTime = info.ModTime()
			newestFile = f
		}
	}

	return newestFile, nil
}
