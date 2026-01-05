package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"youflac/backend"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct - main Wails application
type App struct {
	ctx       context.Context
	queue     *backend.Queue
	config    *backend.Config
	fileIndex *backend.FileIndex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load config first
	config, err := backend.LoadConfig()
	if err == nil {
		a.config = config
	} else {
		// Use default config
		a.config = &backend.Config{
			OutputDirectory:     backend.GetDefaultOutputDirectory(),
			VideoQuality:        "best",
			AudioSourcePriority: []string{"tidal", "qobuz", "amazon"},
			NamingTemplate:      "{artist}/{title}/{title}",
			GenerateNFO:         true,
			ConcurrentDownloads: 2,
			EmbedCoverArt:       true,
			Theme:               "system",
		}
	}

	// Create queue with concurrency from config
	maxConcurrent := a.config.ConcurrentDownloads
	if maxConcurrent < 1 {
		maxConcurrent = 2
	}
	a.queue = backend.NewQueue(ctx, maxConcurrent)

	// Set config for queue
	a.queue.SetConfig(a.config)

	// Set up progress callback to emit Wails events
	a.queue.SetProgressCallback(func(event backend.QueueEvent) {
		runtime.EventsEmit(ctx, "queue:event", event)
	})

	// Load persisted queue
	a.queue.LoadQueue()

	// Start auto-save (every 30 seconds)
	a.queue.AutoSave(30 * time.Second)

	// Initialize file index for duplicate detection
	a.fileIndex = backend.NewFileIndex(backend.GetDataPath())
	a.fileIndex.Load()

	// Scan output directory in background
	go func() {
		outputDir := a.config.OutputDirectory
		if outputDir == "" {
			outputDir = backend.GetDefaultOutputDirectory()
		}
		a.fileIndex.ScanDirectory(outputDir)
		a.fileIndex.Save()
	}()

	// Pass file index to queue for skip detection
	a.queue.SetFileIndex(a.fileIndex)

	// Start processing queue
	a.queue.StartProcessing()
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	// Stop processing and save queue
	if a.queue != nil {
		a.queue.StopProcessing()
		a.queue.SaveQueue()
	}
}

// =============================================================================
// URL Processing
// =============================================================================

// ParseURL detects URL type and extracts metadata
func (a *App) ParseURL(url string) (*ParseURLResult, error) {
	// Detect if YouTube or Spotify URL
	result := &ParseURLResult{
		URL:  url,
		Type: detectURLType(url),
	}
	return result, nil
}

type ParseURLResult struct {
	URL  string `json:"url"`
	Type string `json:"type"` // "youtube", "spotify", "unknown"
}

func detectURLType(url string) string {
	if strings.Contains(url, "youtube.com") || strings.Contains(url, "youtu.be") {
		return "youtube"
	}
	if strings.Contains(url, "spotify.com") {
		return "spotify"
	}
	return "unknown"
}

// =============================================================================
// Video Operations
// =============================================================================

// GetVideoInfo fetches video metadata from YouTube
func (a *App) GetVideoInfo(url string) (*backend.VideoInfo, error) {
	videoID, err := backend.ParseYouTubeURL(url)
	if err != nil {
		return nil, err
	}
	return backend.GetVideoMetadata(videoID)
}

// =============================================================================
// Audio Matching
// =============================================================================

// FindAudioMatch finds best FLAC audio match for a video
func (a *App) FindAudioMatch(videoInfo *backend.VideoInfo) (*backend.MatchResult, error) {
	// Audio matching is handled automatically in the queue processing pipeline
	return nil, nil
}

// =============================================================================
// Queue Management
// =============================================================================

// AddToQueue adds a download request to the queue
// If the URL is a playlist, all videos are added individually
func (a *App) AddToQueue(request backend.DownloadRequest) (string, error) {
	// Check if it's a playlist URL
	if backend.IsPlaylistURL(request.VideoURL) {
		// Try to extract video ID first (playlist URL might include a video)
		_, err := backend.ParseYouTubeURL(request.VideoURL)
		if err != nil {
			// Pure playlist URL (no video ID), fetch all videos
			ids, err := a.AddPlaylistToQueue(request.VideoURL, request.Quality)
			if err != nil {
				return "", err
			}
			if len(ids) > 0 {
				return ids[0], nil // Return first video ID
			}
			return "", nil
		}
	}
	return a.queue.AddToQueue(request)
}

// AddPlaylistToQueue fetches playlist videos and adds each to the queue
func (a *App) AddPlaylistToQueue(playlistURL string, quality string) ([]string, error) {
	playlistInfo, err := backend.GetPlaylistVideos(playlistURL)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, video := range playlistInfo.Videos {
		request := backend.DownloadRequest{
			VideoURL: video.URL,
			Quality:  quality,
		}

		// Add with metadata already fetched
		videoInfo := &backend.VideoInfo{
			ID:        video.ID,
			Title:     video.Title,
			Artist:    video.Artist,
			Duration:  video.Duration,
			Thumbnail: video.Thumbnail,
			URL:       video.URL,
		}

		// Pass playlist name and position for folder organization
		id, err := a.queue.AddToQueueWithPlaylist(request, videoInfo, playlistInfo.Title, video.Position)
		if err != nil {
			continue // Skip failed items
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// AddToQueueWithMetadata adds an item with pre-fetched metadata
func (a *App) AddToQueueWithMetadata(request backend.DownloadRequest, videoInfo *backend.VideoInfo) (string, error) {
	return a.queue.AddToQueueWithMetadata(request, videoInfo)
}

// GetQueue returns all queue items
func (a *App) GetQueue() []backend.QueueItem {
	return a.queue.GetQueue()
}

// GetQueueItem returns a specific queue item
func (a *App) GetQueueItem(id string) *backend.QueueItem {
	return a.queue.GetItem(id)
}

// GetQueueStats returns queue statistics
func (a *App) GetQueueStats() backend.QueueStats {
	return a.queue.GetStats()
}

// RemoveFromQueue removes an item from the queue
func (a *App) RemoveFromQueue(id string) error {
	return a.queue.RemoveFromQueue(id)
}

// CancelQueueItem cancels a processing item
func (a *App) CancelQueueItem(id string) error {
	return a.queue.CancelItem(id)
}

// ClearCompleted removes all completed items from the queue
func (a *App) ClearCompleted() int {
	return a.queue.ClearCompleted()
}

// RetryFailed resets all failed items to pending for retry
func (a *App) RetryFailed() int {
	return a.queue.RetryFailed()
}

// ClearQueue removes all items from the queue
func (a *App) ClearQueue() {
	a.queue.ClearAll()
}

// MoveQueueItem moves an item to a new position
func (a *App) MoveQueueItem(id string, newIndex int) error {
	return a.queue.MoveItem(id, newIndex)
}

// SaveQueue persists the queue to disk
func (a *App) SaveQueue() error {
	return a.queue.SaveQueue()
}

// =============================================================================
// Settings
// =============================================================================

// GetConfig returns current configuration
func (a *App) GetConfig() *backend.Config {
	return a.config
}

// SaveConfig saves configuration
func (a *App) SaveConfig(config backend.Config) error {
	a.config = &config
	return backend.SaveConfig(&config)
}

// GetDefaultOutputDirectory returns default output path
func (a *App) GetDefaultOutputDirectory() string {
	return backend.GetDefaultOutputDirectory()
}

// =============================================================================
// File Manager
// =============================================================================

// FileInfo represents a file in the file manager
type FileInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Type      string `json:"type"` // "video", "cover", "nfo"
	Thumbnail string `json:"thumbnail,omitempty"`
}

// ListFiles lists files in a directory filtered by type
func (a *App) ListFiles(directory string, fileType string) ([]FileInfo, error) {
	if directory == "" {
		directory = a.config.OutputDirectory
		if directory == "" {
			directory = backend.GetDefaultOutputDirectory()
		}
	}

	var files []FileInfo
	var extensions []string

	switch fileType {
	case "videos":
		extensions = []string{".mkv", ".mp4", ".webm", ".avi"}
	case "audio":
		extensions = []string{".flac", ".mp3", ".wav", ".m4a"}
	case "covers":
		extensions = []string{".jpg", ".jpeg", ".png", ".webp"}
	case "nfo":
		extensions = []string{".nfo"}
	default:
		extensions = []string{".mkv", ".mp4", ".webm", ".avi", ".flac", ".mp3", ".wav", ".m4a", ".jpg", ".jpeg", ".png", ".webp", ".nfo"}
	}

	// Walk directory recursively
	filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, e := range extensions {
			if ext == e {
				ft := "video"
				if ext == ".flac" || ext == ".mp3" || ext == ".wav" || ext == ".m4a" {
					ft = "audio"
				} else if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
					ft = "cover"
				} else if ext == ".nfo" {
					ft = "nfo"
				}

				files = append(files, FileInfo{
					Name: info.Name(),
					Path: path,
					Size: info.Size(),
					Type: ft,
				})
				break
			}
		}
		return nil
	})

	return files, nil
}

// BrowseDirectory opens a directory picker dialog
func (a *App) BrowseDirectory() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Select Directory",
		DefaultDirectory: backend.GetDefaultOutputDirectory(),
	})
	return dir, err
}

// OpenFile opens a file with the default system application
func (a *App) OpenFile(path string) {
	runtime.BrowserOpenURL(a.ctx, "file://"+path)
}

// OpenDirectory opens a directory in the file manager
func (a *App) OpenDirectory(path string) {
	dir := filepath.Dir(path)
	runtime.BrowserOpenURL(a.ctx, "file://"+dir)
}

// GetImageAsDataURL reads an image file and returns it as a data URL
func (a *App) GetImageAsDataURL(imagePath string) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(imagePath))
	mimeType := "image/jpeg"
	switch ext {
	case ".png":
		mimeType = "image/png"
	case ".webp":
		mimeType = "image/webp"
	case ".gif":
		mimeType = "image/gif"
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return "data:" + mimeType + ";base64," + encoded, nil
}

// =============================================================================
// Playlist Reorganization
// =============================================================================

// ReorganizePlaylistResult contains the result of playlist reorganization
type ReorganizePlaylistResult struct {
	Renamed int      `json:"renamed"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

// ReorganizePlaylist renames files in a playlist folder with track number prefixes
func (a *App) ReorganizePlaylist(playlistFolder string) (*ReorganizePlaylistResult, error) {
	result := &ReorganizePlaylistResult{}

	// Get playlist name from folder path
	playlistName := filepath.Base(playlistFolder)

	// Get all queue items for this playlist
	allItems := a.queue.GetQueue()
	playlistItems := make(map[string]*backend.QueueItem) // key: normalized title+artist

	for i := range allItems {
		item := &allItems[i]
		if item.PlaylistName == playlistName && item.PlaylistPosition > 0 {
			// Create normalized key for matching
			key := backend.NormalizeForMatching(item.Title, item.Artist)
			keyStr := key.Title + "|" + key.Artist
			playlistItems[keyStr] = item
		}
	}

	if len(playlistItems) == 0 {
		return result, nil // No playlist items with positions
	}

	// Walk through the playlist folder
	err := filepath.Walk(playlistFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".mkv" && ext != ".mp4" && ext != ".flac" {
			return nil
		}

		// Try to match file to a queue item
		// Extract title and artist from filename or embedded metadata
		title, artist := backend.ParseFilename(path)

		key := backend.NormalizeForMatching(title, artist)
		keyStr := key.Title + "|" + key.Artist

		item, found := playlistItems[keyStr]
		if !found {
			result.Skipped++
			return nil
		}

		// Check if already has track prefix
		filename := info.Name()
		if strings.HasPrefix(filename, fmt.Sprintf("%02d - ", item.PlaylistPosition)) {
			result.Skipped++
			return nil
		}

		// Generate new filename with track prefix
		newFilename := fmt.Sprintf("%02d - %s - %s%s",
			item.PlaylistPosition,
			backend.SanitizeFileName(item.Artist),
			backend.SanitizeFileName(item.Title),
			ext)

		// Create new folder with track prefix
		parentDir := filepath.Dir(path)
		grandParentDir := filepath.Dir(parentDir)

		// New folder structure: "01 - Artist - Title/01 - Artist - Title.mkv"
		newFolderName := strings.TrimSuffix(newFilename, ext)
		newFolder := filepath.Join(grandParentDir, newFolderName)
		newPath := filepath.Join(newFolder, newFilename)

		// Create new directory
		if err := os.MkdirAll(newFolder, 0755); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to create dir for %s: %v", filename, err))
			return nil
		}

		// Move file
		if err := os.Rename(path, newPath); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to rename %s: %v", filename, err))
			return nil
		}

		// Also move associated files (NFO, poster)
		nfoPath := strings.TrimSuffix(path, ext) + ".nfo"
		if _, err := os.Stat(nfoPath); err == nil {
			newNfoPath := strings.TrimSuffix(newPath, ext) + ".nfo"
			os.Rename(nfoPath, newNfoPath)
		}

		posterPath := filepath.Join(parentDir, "poster.jpg")
		if _, err := os.Stat(posterPath); err == nil {
			newPosterPath := filepath.Join(newFolder, "poster.jpg")
			os.Rename(posterPath, newPosterPath)
		}

		// Try to remove old empty directory
		os.Remove(parentDir)

		result.Renamed++
		return nil
	})

	if err != nil {
		return result, err
	}

	return result, nil
}

// GetPlaylistFolders returns list of playlist folders in output directory
func (a *App) GetPlaylistFolders() ([]string, error) {
	outputDir := a.config.OutputDirectory
	if outputDir == "" {
		outputDir = backend.GetDefaultOutputDirectory()
	}

	var folders []string

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return nil, err
	}

	// Get all playlist names from queue
	allItems := a.queue.GetQueue()
	playlistNames := make(map[string]bool)
	for _, item := range allItems {
		if item.PlaylistName != "" {
			playlistNames[backend.SanitizeFileName(item.PlaylistName)] = true
		}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Check if this folder corresponds to a playlist
			if playlistNames[entry.Name()] {
				folders = append(folders, filepath.Join(outputDir, entry.Name()))
			}
		}
	}

	return folders, nil
}

// FlattenPlaylistResult contains the result of flattening a playlist folder
type FlattenPlaylistResult struct {
	Moved   int      `json:"moved"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

// FlattenPlaylistFolder moves all files from subfolders to the root of the playlist folder
func (a *App) FlattenPlaylistFolder(playlistFolder string) (*FlattenPlaylistResult, error) {
	result := &FlattenPlaylistResult{}

	// Collect all files to move first (to avoid modifying while iterating)
	type fileToMove struct {
		srcPath  string
		filename string
	}
	var filesToMove []fileToMove

	// Walk through the playlist folder
	err := filepath.Walk(playlistFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip files already at root level
		if filepath.Dir(path) == playlistFolder {
			result.Skipped++
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		// Only move media files
		if ext == ".mkv" || ext == ".mp4" || ext == ".flac" || ext == ".mp3" || ext == ".wav" {
			filesToMove = append(filesToMove, fileToMove{
				srcPath:  path,
				filename: info.Name(),
			})
		}
		return nil
	})

	if err != nil {
		return result, err
	}

	// Move files to root
	for _, f := range filesToMove {
		destPath := filepath.Join(playlistFolder, f.filename)

		// Handle filename collision
		if _, err := os.Stat(destPath); err == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("File already exists: %s", f.filename))
			continue
		}

		if err := os.Rename(f.srcPath, destPath); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to move %s: %v", f.filename, err))
			continue
		}
		result.Moved++
	}

	// Clean up empty directories
	filepath.Walk(playlistFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == playlistFolder {
			return nil
		}
		// Try to remove (will fail if not empty, which is fine)
		os.Remove(path)
		return nil
	})

	return result, nil
}

// =============================================================================
// System
// =============================================================================

// GetAppVersion returns application version
func (a *App) GetAppVersion() string {
	return "1.0.0"
}

