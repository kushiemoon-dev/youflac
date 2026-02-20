package backend

import (
	"fmt"
	"os"
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
			NewLucidaService(nil),
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
	info, err := ResolveMusicURL(spotifyOrYouTubeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve URL: %w", err)
	}

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

// qualityTier maps a keyword to a numeric rank (higher = better).
// Order matters: longer/more specific keywords must appear before shorter ones
// (e.g. "highest" before "high", "hires" before "hi").
type qualityTier struct {
	keyword string
	rank    int
}

var qualityTiers = []qualityTier{
	{"hi_res", 3},
	{"hires", 3},
	{"24bit", 3},
	{"highest", 3},
	{"lossless", 2},
	{"flac", 2},
	{"16bit", 2},
	{"high", 1},
	{"lossy", 1},
	{"mp3", 1},
}

// isQualityDowngrade reports whether the actualQuality is lower than the
// requestedQuality, based on the qualityTiers table.
func isQualityDowngrade(requested, actual string) bool {
	reqRank := qualityRankOf(requested)
	actRank := qualityRankOf(actual)
	return reqRank > 0 && actRank > 0 && actRank < reqRank
}

func qualityRankOf(q string) int {
	q = strings.ToLower(q)
	for _, tier := range qualityTiers {
		if strings.Contains(q, tier.keyword) {
			return tier.rank
		}
	}
	return 0
}
