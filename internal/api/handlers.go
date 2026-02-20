package api

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"

	"youflac/backend"
)

const AppVersion = "1.0.1"

// Health check
func (s *Server) handleHealth(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"version": AppVersion,
	})
}

func (s *Server) handleGetVersion(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"version": AppVersion})
}

// ============== Queue Handlers ==============

func (s *Server) handleGetQueue(c *fiber.Ctx) error {
	items := s.queue.GetQueue()
	return c.JSON(items)
}

func (s *Server) handleAddToQueue(c *fiber.Ctx) error {
	var req backend.DownloadRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := backend.ValidateYouTubeURL(req.VideoURL); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid video URL: " + err.Error()})
	}

	id, err := s.queue.AddToQueue(req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"id": id})
}

func (s *Server) handleGetQueueItem(c *fiber.Ctx) error {
	id := c.Params("id")
	item := s.queue.GetItem(id)
	if item == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Item not found"})
	}
	return c.JSON(item)
}

func (s *Server) handleRemoveFromQueue(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := s.queue.RemoveFromQueue(id); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) handleCancelQueueItem(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := s.queue.CancelItem(id); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) handleMoveQueueItem(c *fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		NewPosition int `json:"newPosition"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := s.queue.MoveItem(id, body.NewPosition); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) handleGetQueueStats(c *fiber.Ctx) error {
	stats := s.queue.GetStats()
	return c.JSON(stats)
}

func (s *Server) handleClearCompleted(c *fiber.Ctx) error {
	count := s.queue.ClearCompleted()
	return c.JSON(fiber.Map{"cleared": count})
}

func (s *Server) handleRetryFailed(c *fiber.Ctx) error {
	count := s.queue.RetryFailed()
	return c.JSON(fiber.Map{"retried": count})
}

// ============== Playlist Handlers ==============

func (s *Server) handleAddPlaylistToQueue(c *fiber.Ctx) error {
	var body struct {
		URL     string `json:"url"`
		Quality string `json:"quality"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := backend.ValidateYouTubeURL(body.URL); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid playlist URL: " + err.Error()})
	}

	quality := body.Quality
	if quality == "" {
		quality = s.config.VideoQuality
	}

	// Get playlist info
	playlist, err := backend.GetPlaylistVideos(body.URL)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Add each video to queue
	ids := []string{}
	for _, video := range playlist.Videos {
		req := backend.DownloadRequest{
			VideoURL: video.URL,
			Quality:  quality,
		}
		// Convert PlaylistVideo to VideoInfo
		videoInfo := &backend.VideoInfo{
			ID:        video.ID,
			Title:     video.Title,
			Artist:    video.Artist,
			Duration:  video.Duration,
			Thumbnail: video.Thumbnail,
			URL:       video.URL,
		}
		id, err := s.queue.AddToQueueWithPlaylist(req, videoInfo, playlist.Title, video.Position)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}

	return c.JSON(fiber.Map{"ids": ids, "playlistTitle": playlist.Title})
}

// ============== Config Handlers ==============

func (s *Server) handleGetConfig(c *fiber.Ctx) error {
	config, err := backend.LoadConfig()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(config)
}

func (s *Server) handleSaveConfig(c *fiber.Ctx) error {
	var config backend.Config
	if err := c.BodyParser(&config); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := backend.ValidateOutputDirectory(config.OutputDirectory); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid output directory: " + err.Error()})
	}

	if len(config.AudioSourcePriority) > 0 {
		if err := backend.ValidateAudioSources(config.AudioSourcePriority); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid audio source priority: " + err.Error()})
		}
	}

	if err := backend.SaveConfig(&config); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Update server config
	s.config = &config

	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) handleGetDefaultOutput(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"path": backend.GetDefaultOutputDirectory()})
}

// ============== History Handlers ==============

func (s *Server) handleGetHistory(c *fiber.Ctx) error {
	entries := s.history.GetAll()
	return c.JSON(entries)
}

func (s *Server) handleGetHistoryStats(c *fiber.Ctx) error {
	stats := s.history.GetStats()
	return c.JSON(stats)
}

func (s *Server) handleSearchHistory(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.JSON(s.history.GetAll())
	}
	results := s.history.Search(query)
	return c.JSON(results)
}

func (s *Server) handleDeleteHistoryEntry(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := s.history.Delete(id); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) handleClearHistory(c *fiber.Ctx) error {
	if err := s.history.Clear(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) handleRedownloadFromHistory(c *fiber.Ctx) error {
	id := c.Params("id")

	// Find entry in history
	entries := s.history.GetAll()
	var entry *backend.HistoryEntry
	for _, e := range entries {
		if e.ID == id {
			entry = &e
			break
		}
	}

	if entry == nil {
		return c.Status(404).JSON(fiber.Map{"error": "History entry not found"})
	}

	// Add to queue
	req := backend.DownloadRequest{
		VideoURL: entry.VideoURL,
		Quality:  entry.Quality,
	}

	newID, err := s.queue.AddToQueue(req)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"id": newID})
}

// ============== Video/URL Handlers ==============

type ParseURLResult struct {
	Type       string `json:"type"`       // "video", "playlist", "invalid"
	VideoID    string `json:"videoId"`
	PlaylistID string `json:"playlistId"`
	URL        string `json:"url"`
}

func (s *Server) handleParseURL(c *fiber.Ctx) error {
	var body struct {
		URL string `json:"url"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	url := strings.TrimSpace(body.URL)
	result := ParseURLResult{URL: url}

	// Check if playlist
	if strings.Contains(url, "list=") {
		result.Type = "playlist"
		// Extract playlist ID
		if idx := strings.Index(url, "list="); idx != -1 {
			pID := url[idx+5:]
			if ampIdx := strings.Index(pID, "&"); ampIdx != -1 {
				pID = pID[:ampIdx]
			}
			result.PlaylistID = pID
		}
	} else {
		// Try to parse as video
		videoID, err := backend.ParseYouTubeURL(url)
		if err != nil {
			result.Type = "invalid"
		} else {
			result.Type = "video"
			result.VideoID = videoID
		}
	}

	return c.JSON(result)
}

func (s *Server) handleGetVideoInfo(c *fiber.Ctx) error {
	url := c.Query("url")
	if url == "" {
		return c.Status(400).JSON(fiber.Map{"error": "URL required"})
	}

	videoID, err := backend.ParseYouTubeURL(url)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	info, err := backend.GetVideoMetadata(videoID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(info)
}

func (s *Server) handleFindAudioMatch(c *fiber.Ctx) error {
	var videoInfo backend.VideoInfo
	if err := c.BodyParser(&videoInfo); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Use empty candidates and default options - the matcher will try to find matches
	result, err := backend.MatchVideoToAudio(&videoInfo, nil, nil)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(result)
}

// ============== Files Handlers ==============

type FileInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"isDir"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"`
	Type      string `json:"type"` // "video", "audio", "cover", "nfo", "other"
}

// getFileType determines the type of file based on its extension
func getFileType(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".mkv", ".mp4", ".webm", ".avi", ".mov":
		return "video"
	case ".flac", ".mp3", ".m4a", ".aac", ".ogg", ".opus", ".wav":
		return "audio"
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return "cover"
	case ".nfo":
		return "nfo"
	case ".lrc":
		return "lyrics"
	default:
		return "other"
	}
}

func (s *Server) handleListFiles(c *fiber.Ctx) error {
	dir := c.Query("dir")
	if dir == "" {
		dir = s.config.OutputDirectory
		if dir == "" {
			dir = backend.GetDefaultOutputDirectory()
		}
	}

	filter := c.Query("filter") // e.g., ".mkv,.flac"

	entries, err := os.ReadDir(dir)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	files := []FileInfo{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))

		// Apply filter if specified
		if filter != "" {
			filters := strings.Split(filter, ",")
			matched := entry.IsDir() // Always include directories
			for _, f := range filters {
				if strings.ToLower(strings.TrimSpace(f)) == ext {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		files = append(files, FileInfo{
			Name:      entry.Name(),
			Path:      filepath.Join(dir, entry.Name()),
			IsDir:     entry.IsDir(),
			Size:      info.Size(),
			Extension: ext,
			Type:      getFileType(ext),
		})
	}

	return c.JSON(files)
}

func (s *Server) handleGetPlaylistFolders(c *fiber.Ctx) error {
	outputDir := s.config.OutputDirectory
	if outputDir == "" {
		outputDir = backend.GetDefaultOutputDirectory()
	}

	folders := []string{}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return c.JSON(folders) // Return empty if can't read
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Check if it looks like a playlist folder (has numbered files)
			subPath := filepath.Join(outputDir, entry.Name())
			subEntries, _ := os.ReadDir(subPath)
			for _, sub := range subEntries {
				if !sub.IsDir() && strings.HasPrefix(sub.Name(), "01") {
					folders = append(folders, entry.Name())
					break
				}
			}
		}
	}

	return c.JSON(folders)
}

type ReorganizeResult struct {
	Success   bool     `json:"success"`
	Moved     int      `json:"moved"`
	Errors    []string `json:"errors,omitempty"`
	NewFolder string   `json:"newFolder,omitempty"`
}

func (s *Server) handleReorganizePlaylist(c *fiber.Ctx) error {
	var body struct {
		FolderPath string `json:"folderPath"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// For now, return a simple response
	// Full implementation would reorganize files into artist/title structure
	return c.JSON(ReorganizeResult{
		Success: true,
		Moved:   0,
	})
}

func (s *Server) handleFlattenPlaylist(c *fiber.Ctx) error {
	var body struct {
		FolderPath string `json:"folderPath"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// For now, return a simple response
	return c.JSON(ReorganizeResult{
		Success: true,
		Moved:   0,
	})
}

// ============== Analyzer Handlers ==============

func (s *Server) handleAnalyzeAudio(c *fiber.Ctx) error {
	var body struct {
		FilePath string `json:"filePath"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	analysis, err := backend.AnalyzeAudio(body.FilePath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(analysis)
}

func (s *Server) handleGenerateSpectrogram(c *fiber.Ctx) error {
	var body struct {
		FilePath string `json:"filePath"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Generate spectrogram to temp file
	tempDir := os.TempDir()
	outputPath := filepath.Join(tempDir, "spectrogram_"+filepath.Base(body.FilePath)+".png")

	if err := backend.GenerateSpectrogram(body.FilePath, outputPath); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	// Return path that can be fetched via /api/image
	return c.JSON(fiber.Map{"path": outputPath})
}

func (s *Server) handleGenerateWaveform(c *fiber.Ctx) error {
	var body struct {
		FilePath string `json:"filePath"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	tempDir := os.TempDir()
	outputPath := filepath.Join(tempDir, "waveform_"+filepath.Base(body.FilePath)+".png")

	if err := backend.GenerateWaveform(body.FilePath, outputPath); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"path": outputPath})
}

// ============== Lyrics Handlers ==============

func (s *Server) handleFetchLyrics(c *fiber.Ctx) error {
	artist := c.Query("artist")
	title := c.Query("title")
	album := c.Query("album")

	if artist == "" || title == "" {
		return c.Status(400).JSON(fiber.Map{"error": "artist and title required"})
	}

	var lyrics *backend.LyricsResult
	var err error

	if album != "" {
		lyrics, err = backend.FetchLyricsWithAlbum(artist, title, album)
	} else {
		lyrics, err = backend.FetchLyrics(artist, title)
	}

	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(lyrics)
}

func (s *Server) handleEmbedLyrics(c *fiber.Ctx) error {
	var body struct {
		MediaPath string               `json:"mediaPath"`
		Lyrics    backend.LyricsResult `json:"lyrics"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if err := backend.EmbedLyricsInFile(body.MediaPath, &body.Lyrics); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true})
}

func (s *Server) handleSaveLRCFile(c *fiber.Ctx) error {
	var body struct {
		MediaPath string               `json:"mediaPath"`
		Lyrics    backend.LyricsResult `json:"lyrics"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	lrcPath, err := backend.SaveLRCFile(&body.Lyrics, body.MediaPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"path": lrcPath})
}

// ============== Image Handler ==============

func (s *Server) handleGetImage(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return c.Status(400).JSON(fiber.Map{"error": "path required"})
	}

	// Security: resolve the real path and check it's within allowed directories.
	// filepath.Abs normalizes ".." traversal sequences before we compare.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return c.Status(403).JSON(fiber.Map{"error": "Access denied"})
	}

	absTemp, _ := filepath.Abs(os.TempDir())
	absOutput := s.config.OutputDirectory
	if absOutput == "" {
		absOutput = backend.GetDefaultOutputDirectory()
	}
	absOutput, _ = filepath.Abs(absOutput)

	// Ensure the separator-terminated prefix so "/tmp" doesn't match "/tmpother"
	if !strings.HasPrefix(absPath, absTemp+string(filepath.Separator)) &&
		!strings.HasPrefix(absPath, absOutput+string(filepath.Separator)) {
		return c.Status(403).JSON(fiber.Map{"error": "Access denied"})
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "File not found"})
	}

	// Return as data URL
	ext := strings.ToLower(filepath.Ext(absPath))
	mimeType := "image/png"
	if ext == ".jpg" || ext == ".jpeg" {
		mimeType = "image/jpeg"
	}

	dataURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	return c.JSON(fiber.Map{"dataUrl": dataURL})
}
