package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wader/goutubedl"
	"gopkg.in/ini.v1"
)

// VideoInfo contains metadata about a YouTube video
type VideoInfo struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album,omitempty"`
	Duration    float64 `json:"duration"`
	ISRC        string  `json:"isrc,omitempty"`
	Thumbnail   string  `json:"thumbnail"`
	URL         string  `json:"url"`
	UploadDate  string  `json:"uploadDate,omitempty"`
	Description string  `json:"description,omitempty"`
	Channel     string  `json:"channel,omitempty"`
	ViewCount   int64   `json:"viewCount,omitempty"`
}

// VideoFormat represents an available video format
type VideoFormat struct {
	FormatID   string `json:"formatId"`
	Extension  string `json:"extension"`
	Resolution string `json:"resolution"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Codec      string `json:"codec"`
	Filesize   int64  `json:"filesize"`
	FPS        int    `json:"fps"`
	VCodec     string `json:"vcodec"`
	ACodec     string `json:"acodec"`
}

// DownloadProgress tracks download progress
type DownloadProgress struct {
	Percent     float64 `json:"percent"`
	Downloaded  int64   `json:"downloaded"`
	Total       int64   `json:"total"`
	Speed       float64 `json:"speed"`
	ETA         string  `json:"eta"`
}

// YouTube URL patterns
var (
	youtubeRegex      = regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtu\.be/|youtube\.com/embed/|youtube\.com/v/|youtube\.com/shorts/)([a-zA-Z0-9_-]{11})`)
	youtubeMusicRegex = regexp.MustCompile(`music\.youtube\.com/watch\?v=([a-zA-Z0-9_-]{11})`)
	playlistRegex     = regexp.MustCompile(`[?&]list=([a-zA-Z0-9_-]+)`)
)

// ParseYouTubeURL extracts video ID from various YouTube URL formats
// Supports: youtube.com, youtu.be, music.youtube.com, shorts
func ParseYouTubeURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)

	if rawURL == "" {
		return "", fmt.Errorf("empty URL")
	}

	// Try YouTube Music first
	if matches := youtubeMusicRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], nil
	}

	// Try standard YouTube patterns
	if matches := youtubeRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], nil
	}

	// Try parsing as URL and extract 'v' parameter
	parsedURL, err := url.Parse(rawURL)
	if err == nil {
		if v := parsedURL.Query().Get("v"); len(v) == 11 {
			return v, nil
		}
	}

	// Check if it's already just a video ID
	if len(rawURL) == 11 && regexp.MustCompile(`^[a-zA-Z0-9_-]{11}$`).MatchString(rawURL) {
		return rawURL, nil
	}

	return "", fmt.Errorf("could not extract video ID from URL: %s", rawURL)
}

// IsPlaylistURL checks if URL contains a playlist
func IsPlaylistURL(rawURL string) bool {
	return playlistRegex.MatchString(rawURL)
}

// ExtractPlaylistID extracts playlist ID from URL
func ExtractPlaylistID(rawURL string) string {
	if matches := playlistRegex.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// PlaylistVideo contains basic info about a video in a playlist
type PlaylistVideo struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Artist    string  `json:"artist"`
	Duration  float64 `json:"duration"`
	Thumbnail string  `json:"thumbnail"`
	URL       string  `json:"url"`
	Position  int     `json:"position"` // 1-based position in playlist
}

// PlaylistInfo contains playlist metadata and videos
type PlaylistInfo struct {
	ID     string          `json:"id"`
	Title  string          `json:"title"`
	Author string          `json:"author"`
	Videos []PlaylistVideo `json:"videos"`
}

// GetPlaylistVideos fetches all videos from a YouTube playlist
// Uses yt-dlp --flat-playlist for fast metadata extraction
func GetPlaylistVideos(playlistURL string) (*PlaylistInfo, error) {
	// Extract playlist ID
	playlistID := ExtractPlaylistID(playlistURL)
	if playlistID == "" {
		return nil, fmt.Errorf("could not extract playlist ID from URL: %s", playlistURL)
	}

	// Construct canonical playlist URL
	canonicalURL := fmt.Sprintf("https://www.youtube.com/playlist?list=%s", playlistID)

	// Use yt-dlp with --flat-playlist for fast extraction (no full video info)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--flat-playlist",
		"-j",
		"--no-warnings",
		canonicalURL,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch playlist: %w", err)
	}

	var videos []PlaylistVideo
	var playlistTitle string
	var playlistAuthor string
	lines := strings.Split(string(output), "\n")
	position := 0 // Track position in playlist

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry struct {
			ID               string  `json:"id"`
			Title            string  `json:"title"`
			Duration         float64 `json:"duration"`
			Channel          string  `json:"channel"`
			Uploader         string  `json:"uploader"`
			Thumbnail        string  `json:"thumbnail"`
			PlaylistTitle    string  `json:"playlist_title"`
			PlaylistUploader string  `json:"playlist_uploader"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue // Skip malformed entries
		}

		// Extract playlist info from first entry
		if playlistTitle == "" && entry.PlaylistTitle != "" {
			playlistTitle = entry.PlaylistTitle
			playlistAuthor = entry.PlaylistUploader
		}

		if entry.ID == "" {
			continue
		}

		position++ // Increment position for valid entries

		artist := entry.Channel
		if artist == "" {
			artist = entry.Uploader
		}
		// Clean up "- Topic" suffix from auto-generated channels
		artist = strings.TrimSuffix(artist, " - Topic")

		// Get best thumbnail
		thumbnail := entry.Thumbnail
		if thumbnail == "" {
			thumbnail = fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", entry.ID)
		}

		videos = append(videos, PlaylistVideo{
			ID:        entry.ID,
			Title:     entry.Title,
			Artist:    artist,
			Duration:  entry.Duration,
			Thumbnail: thumbnail,
			URL:       fmt.Sprintf("https://www.youtube.com/watch?v=%s", entry.ID),
			Position:  position, // Assign 1-based position
		})
	}

	if len(videos) == 0 {
		return nil, fmt.Errorf("playlist is empty or unavailable")
	}

	return &PlaylistInfo{
		ID:     playlistID,
		Title:  playlistTitle,
		Author: playlistAuthor,
		Videos: videos,
	}, nil
}

// SearchYouTube searches YouTube for videos matching a query
// Uses yt-dlp's ytsearch: prefix to search and return results
func SearchYouTube(query string, maxResults int) ([]VideoInfo, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// ytsearchN:query format searches YouTube and returns N results
	searchURL := fmt.Sprintf("ytsearch%d:%s", maxResults, query)

	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--flat-playlist",
		"-j",
		"--no-warnings",
		searchURL,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	var results []VideoInfo
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry struct {
			ID        string  `json:"id"`
			Title     string  `json:"title"`
			Duration  float64 `json:"duration"`
			Channel   string  `json:"channel"`
			Uploader  string  `json:"uploader"`
			Thumbnail string  `json:"thumbnail"`
			ViewCount int64   `json:"view_count"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.ID == "" {
			continue
		}

		artist := entry.Channel
		if artist == "" {
			artist = entry.Uploader
		}
		artist = strings.TrimSuffix(artist, " - Topic")

		// Clean title - remove artist prefix if present
		title := entry.Title
		if artist != "" && strings.HasPrefix(title, artist+" - ") {
			title = strings.TrimPrefix(title, artist+" - ")
		}

		// Get best thumbnail
		thumbnail := entry.Thumbnail
		if thumbnail == "" {
			thumbnail = fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", entry.ID)
		}

		results = append(results, VideoInfo{
			ID:        entry.ID,
			Title:     title,
			Artist:    artist,
			Duration:  entry.Duration,
			Thumbnail: thumbnail,
			URL:       fmt.Sprintf("https://www.youtube.com/watch?v=%s", entry.ID),
			ViewCount: entry.ViewCount,
		})
	}

	return results, nil
}

// SearchYouTubeWithCookies searches YouTube with browser cookies for better results
func SearchYouTubeWithCookies(query string, maxResults int, cookiesBrowser string) ([]VideoInfo, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	searchURL := fmt.Sprintf("ytsearch%d:%s", maxResults, query)

	args := []string{
		"--flat-playlist",
		"-j",
		"--no-warnings",
	}

	// Add cookies if browser specified
	if cookiesBrowser != "" {
		resolvedBrowser, err := resolveCookiesBrowser(cookiesBrowser)
		if err == nil {
			args = append(args, "--cookies-from-browser", resolvedBrowser)
		}
	}

	args = append(args, searchURL)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	var results []VideoInfo
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry struct {
			ID        string  `json:"id"`
			Title     string  `json:"title"`
			Duration  float64 `json:"duration"`
			Channel   string  `json:"channel"`
			Uploader  string  `json:"uploader"`
			Thumbnail string  `json:"thumbnail"`
			ViewCount int64   `json:"view_count"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.ID == "" {
			continue
		}

		artist := entry.Channel
		if artist == "" {
			artist = entry.Uploader
		}
		artist = strings.TrimSuffix(artist, " - Topic")

		title := entry.Title
		if artist != "" && strings.HasPrefix(title, artist+" - ") {
			title = strings.TrimPrefix(title, artist+" - ")
		}

		thumbnail := entry.Thumbnail
		if thumbnail == "" {
			thumbnail = fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", entry.ID)
		}

		results = append(results, VideoInfo{
			ID:        entry.ID,
			Title:     title,
			Artist:    artist,
			Duration:  entry.Duration,
			Thumbnail: thumbnail,
			URL:       fmt.Sprintf("https://www.youtube.com/watch?v=%s", entry.ID),
			ViewCount: entry.ViewCount,
		})
	}

	return results, nil
}

// GetVideoMetadata fetches video metadata using yt-dlp
func GetVideoMetadata(videoID string) (*VideoInfo, error) {
	ctx := context.Background()

	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	result, err := goutubedl.New(ctx, videoURL, goutubedl.Options{
		Type: goutubedl.TypeSingle,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	info := result.Info

	// Extract artist from various fields
	artist := info.Artist
	if artist == "" {
		artist = info.Creator
	}
	if artist == "" {
		artist = info.Uploader
	}
	if artist == "" {
		artist = info.Channel
	}

	// Clean title - remove artist prefix if present
	title := info.Title
	if artist != "" && strings.HasPrefix(title, artist+" - ") {
		title = strings.TrimPrefix(title, artist+" - ")
	}

	return &VideoInfo{
		ID:          info.ID,
		Title:       title,
		Artist:      artist,
		Album:       info.Album,
		Duration:    info.Duration,
		Thumbnail:   info.Thumbnail,
		URL:         videoURL,
		UploadDate:  info.UploadDate,
		Description: info.Description,
		Channel:     info.Channel,
		ViewCount:   int64(info.ViewCount),
	}, nil
}

// GetVideoMetadataFromURL fetches metadata directly from URL
func GetVideoMetadataFromURL(videoURL string) (*VideoInfo, error) {
	videoID, err := ParseYouTubeURL(videoURL)
	if err != nil {
		return nil, err
	}
	return GetVideoMetadata(videoID)
}

// GetAvailableFormats lists all available formats for a video
func GetAvailableFormats(videoID string) ([]VideoFormat, error) {
	ctx := context.Background()

	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	result, err := goutubedl.New(ctx, videoURL, goutubedl.Options{
		Type: goutubedl.TypeSingle,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch formats: %w", err)
	}

	var formats []VideoFormat
	for _, f := range result.Info.Formats {
		// Only include formats with video
		if f.VCodec == "none" || f.VCodec == "" {
			continue
		}

		resolution := fmt.Sprintf("%dx%d", int(f.Width), int(f.Height))
		if f.Width == 0 || f.Height == 0 {
			resolution = f.Resolution
		}

		formats = append(formats, VideoFormat{
			FormatID:   f.FormatID,
			Extension:  f.Ext,
			Resolution: resolution,
			Width:      int(f.Width),
			Height:     int(f.Height),
			Codec:      f.VCodec,
			Filesize:   int64(f.Filesize),
			FPS:        int(f.FPS),
			VCodec:     f.VCodec,
			ACodec:     f.ACodec,
		})
	}

	return formats, nil
}

// getLibrewolfProfilePath finds the default Librewolf profile path
func getLibrewolfProfilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	librewolfDir := filepath.Join(homeDir, ".librewolf")
	profilesIni := filepath.Join(librewolfDir, "profiles.ini")

	// Try to read profiles.ini to find default profile
	cfg, err := ini.Load(profilesIni)
	if err == nil {
		// Look for Install* sections first (newer format)
		for _, section := range cfg.Sections() {
			if strings.HasPrefix(section.Name(), "Install") {
				if path := section.Key("Default").String(); path != "" {
					fullPath := filepath.Join(librewolfDir, path)
					if _, err := os.Stat(fullPath); err == nil {
						return fullPath, nil
					}
				}
			}
		}
		// Fall back to Profile sections
		for _, section := range cfg.Sections() {
			if strings.HasPrefix(section.Name(), "Profile") {
				if section.Key("Default").String() == "1" {
					path := section.Key("Path").String()
					if path != "" {
						fullPath := filepath.Join(librewolfDir, path)
						if _, err := os.Stat(fullPath); err == nil {
							return fullPath, nil
						}
					}
				}
			}
		}
	}

	// Fallback: find first .default-default directory
	entries, err := os.ReadDir(librewolfDir)
	if err != nil {
		return "", fmt.Errorf("librewolf directory not found: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".default-default") {
			return filepath.Join(librewolfDir, entry.Name()), nil
		}
	}

	// Try any .default directory
	for _, entry := range entries {
		if entry.IsDir() && strings.Contains(entry.Name(), ".default") {
			return filepath.Join(librewolfDir, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no librewolf profile found")
}

// resolveCookiesBrowser converts browser name to yt-dlp format
// Handles special case for librewolf which needs firefox:PATH format
func resolveCookiesBrowser(browser string) (string, error) {
	if browser == "librewolf" {
		profilePath, err := getLibrewolfProfilePath()
		if err != nil {
			return "", fmt.Errorf("failed to find librewolf profile: %w", err)
		}
		return fmt.Sprintf("firefox:%s", profilePath), nil
	}
	return browser, nil
}

// DownloadVideo downloads video to specified path
// quality can be: "best", "1080p", "720p", "480p", "360p"
// cookiesBrowser can be: "firefox", "chrome", "chromium", "brave", "opera", "edge", "librewolf", or "" for none
func DownloadVideo(videoID string, quality string, outputDir string, cookiesBrowser string) (string, error) {
	ctx := context.Background()

	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	// Resolve browser name (handles librewolf -> firefox:path conversion)
	resolvedBrowser := cookiesBrowser
	if cookiesBrowser != "" {
		var err error
		resolvedBrowser, err = resolveCookiesBrowser(cookiesBrowser)
		if err != nil {
			return "", fmt.Errorf("failed to resolve browser cookies: %w", err)
		}
	}

	// Build args for metadata fetch
	metadataArgs := []string{
		"--dump-json",
		"--no-download",
		"--no-playlist",
	}
	if resolvedBrowser != "" {
		metadataArgs = append(metadataArgs, "--cookies-from-browser", resolvedBrowser)
	}
	metadataArgs = append(metadataArgs, videoURL)

	// Get metadata using yt-dlp directly (to support cookies)
	metadataCmd := exec.CommandContext(ctx, "yt-dlp", metadataArgs...)
	metadataOutput, err := metadataCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get video info: %w", err)
	}

	var videoInfo struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(metadataOutput, &videoInfo); err != nil {
		return "", fmt.Errorf("failed to parse video info: %w", err)
	}

	// Build format selector based on quality
	formatSelector := buildFormatSelector(quality)

	// Create output filename
	safeTitle := sanitizeVideoFileName(videoInfo.Title)
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.mp4", safeTitle))

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use yt-dlp directly via exec.Command
	args := []string{
		"-f", formatSelector,
		"--no-playlist",
		"--merge-output-format", "mp4",
		"-o", outputPath,
	}
	if resolvedBrowser != "" {
		args = append(args, "--cookies-from-browser", resolvedBrowser)
	}
	args = append(args, videoURL)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp download failed: %w", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("download completed but file not found: %s", outputPath)
	}

	return outputPath, nil
}

// DownloadVideoOnly downloads only video stream (no audio)
func DownloadVideoOnly(videoID string, quality string, outputDir string) (string, error) {
	ctx := context.Background()

	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	result, err := goutubedl.New(ctx, videoURL, goutubedl.Options{
		Type: goutubedl.TypeSingle,
	})
	if err != nil {
		return "", fmt.Errorf("failed to initialize: %w", err)
	}

	// Video only format selector
	formatSelector := buildVideoOnlyFormatSelector(quality)

	safeTitle := sanitizeVideoFileName(result.Info.Title)
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_video.mp4", safeTitle))

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	downloadResult, err := goutubedl.New(ctx, videoURL, goutubedl.Options{
		Type: goutubedl.TypeSingle,
	})
	if err != nil {
		return "", err
	}

	downloadReader, err := downloadResult.Download(ctx, formatSelector)
	if err != nil {
		return "", fmt.Errorf("failed to download video: %w", err)
	}
	defer downloadReader.Close()

	outFile, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	buf := make([]byte, 32*1024)
	for {
		n, err := downloadReader.Read(buf)
		if n > 0 {
			outFile.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return outputPath, nil
}

// buildFormatSelector creates yt-dlp format selector string
func buildFormatSelector(quality string) string {
	switch quality {
	case "1080p":
		return "bestvideo[height<=1080]+bestaudio/best[height<=1080]"
	case "720p":
		return "bestvideo[height<=720]+bestaudio/best[height<=720]"
	case "480p":
		return "bestvideo[height<=480]+bestaudio/best[height<=480]"
	case "360p":
		return "bestvideo[height<=360]+bestaudio/best[height<=360]"
	default: // "best"
		return "bestvideo+bestaudio/best"
	}
}

// buildVideoOnlyFormatSelector creates format selector for video-only download
func buildVideoOnlyFormatSelector(quality string) string {
	switch quality {
	case "1080p":
		return "bestvideo[height<=1080]"
	case "720p":
		return "bestvideo[height<=720]"
	case "480p":
		return "bestvideo[height<=480]"
	case "360p":
		return "bestvideo[height<=360]"
	default:
		return "bestvideo"
	}
}

// sanitizeVideoFileName removes invalid characters from filename (local helper)
func sanitizeVideoFileName(name string) string {
	// Use the shared SanitizeFileName from naming.go
	sanitized := SanitizeFileName(name)

	// Limit length for video files
	if len(sanitized) > 200 {
		sanitized = sanitized[:200]
	}

	if sanitized == "" {
		sanitized = "video"
	}

	return sanitized
}
