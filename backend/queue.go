package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Download queue management

type QueueStatus string

const (
	StatusPending          QueueStatus = "pending"
	StatusFetchingInfo     QueueStatus = "fetching_info"
	StatusDownloadingVideo QueueStatus = "downloading_video"
	StatusDownloadingAudio QueueStatus = "downloading_audio"
	StatusMuxing           QueueStatus = "muxing"
	StatusOrganizing       QueueStatus = "organizing"
	StatusComplete         QueueStatus = "complete"
	StatusError            QueueStatus = "error"
	StatusCancelled        QueueStatus = "cancelled"
)

// QueueItem represents a single download in the queue
type QueueItem struct {
	ID           string      `json:"id"`
	VideoURL     string      `json:"videoUrl"`
	SpotifyURL   string      `json:"spotifyUrl,omitempty"`
	Title        string      `json:"title"`
	Artist       string      `json:"artist"`
	Album        string      `json:"album,omitempty"`
	PlaylistName     string `json:"playlistName,omitempty"`     // Playlist folder name
	PlaylistPosition int    `json:"playlistPosition,omitempty"` // Position in playlist (1-based)
	Thumbnail    string      `json:"thumbnail,omitempty"`
	Duration     float64     `json:"duration,omitempty"`
	Status       QueueStatus `json:"status"`
	Progress     int         `json:"progress"` // 0-100
	Stage        string      `json:"stage"`    // Human-readable current stage
	Error        string      `json:"error,omitempty"`
	OutputPath   string      `json:"outputPath,omitempty"`
	VideoPath    string      `json:"videoPath,omitempty"`   // Temp video file
	AudioPath    string      `json:"audioPath,omitempty"`   // Temp audio file
	FileSize     int64       `json:"fileSize,omitempty"`    // Output file size
	CreatedAt    time.Time   `json:"createdAt"`
	StartedAt    time.Time   `json:"startedAt,omitempty"`
	CompletedAt  time.Time   `json:"completedAt,omitempty"`

	// Matching info
	MatchScore      int    `json:"matchScore,omitempty"`
	MatchConfidence string `json:"matchConfidence,omitempty"`
	AudioSource     string `json:"audioSource,omitempty"` // tidal, qobuz, amazon, etc.
	Quality         string `json:"quality,omitempty"` // FLAC quality (e.g. "24-bit/96kHz")

	// Audio-only fallback (video unavailable)
	AudioOnly bool `json:"audioOnly,omitempty"`

	// Cancel channel (not serialized)
	cancelFunc context.CancelFunc `json:"-"`
}

// DownloadRequest is the input for adding items to queue
type DownloadRequest struct {
	VideoURL   string `json:"videoUrl"`
	SpotifyURL string `json:"spotifyUrl,omitempty"`
	Quality    string `json:"quality,omitempty"` // "best", "1080p", "720p", "480p"
}

// QueueEvent is emitted to frontend for progress updates
type QueueEvent struct {
	Type     string      `json:"type"` // "added", "updated", "removed", "completed", "error"
	ItemID   string      `json:"itemId"`
	Item     *QueueItem  `json:"item,omitempty"`
	Progress int         `json:"progress,omitempty"`
	Status   QueueStatus `json:"status,omitempty"`
	Error    string      `json:"error,omitempty"`
}

// QueueProgressCallback is called when progress updates occur
type QueueProgressCallback func(event QueueEvent)

// Queue manages the download queue with concurrent workers
type Queue struct {
	items        []QueueItem
	mutex        sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	maxConc      int // Max concurrent downloads
	onProgress   QueueProgressCallback
	workerWG     sync.WaitGroup
	jobChan      chan string // Channel of item IDs to process
	processing   bool
	processMutex sync.Mutex

	// Configuration
	config *Config

	// File index for duplicate detection
	fileIndex *FileIndex

	// History for tracking completed downloads
	history *History
}

// NewQueue creates a new download queue
func NewQueue(ctx context.Context, maxConcurrent int) *Queue {
	ctx, cancel := context.WithCancel(ctx)
	return &Queue{
		items:   make([]QueueItem, 0),
		ctx:     ctx,
		cancel:  cancel,
		maxConc: maxConcurrent,
		jobChan: make(chan string, 100),
	}
}

// SetProgressCallback sets the callback for progress events
func (q *Queue) SetProgressCallback(cb QueueProgressCallback) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.onProgress = cb
}

// SetConfig sets the configuration for downloads
func (q *Queue) SetConfig(config *Config) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.config = config
}

// SetFileIndex sets the file index for duplicate detection
func (q *Queue) SetFileIndex(fi *FileIndex) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.fileIndex = fi
}

// SetHistory sets the history manager for recording completed downloads
func (q *Queue) SetHistory(h *History) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.history = h
}

// emit sends an event to the progress callback
func (q *Queue) emit(event QueueEvent) {
	q.mutex.RLock()
	cb := q.onProgress
	q.mutex.RUnlock()

	if cb != nil {
		cb(event)
	}
}

// AddToQueue adds a new download request to the queue
func (q *Queue) AddToQueue(request DownloadRequest) (string, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	item := QueueItem{
		ID:         uuid.New().String(),
		VideoURL:   request.VideoURL,
		SpotifyURL: request.SpotifyURL,
		Status:     StatusPending,
		Progress:   0,
		Stage:      "Waiting...",
		CreatedAt:  time.Now(),
	}

	q.items = append(q.items, item)

	// Emit event
	go q.emit(QueueEvent{
		Type:   "added",
		ItemID: item.ID,
		Item:   &item,
	})

	return item.ID, nil
}

// AddToQueueWithMetadata adds an item with pre-fetched metadata
func (q *Queue) AddToQueueWithMetadata(request DownloadRequest, videoInfo *VideoInfo) (string, error) {
	return q.AddToQueueWithPlaylist(request, videoInfo, "", 0)
}

// AddToQueueWithPlaylist adds an item with metadata and playlist name
func (q *Queue) AddToQueueWithPlaylist(request DownloadRequest, videoInfo *VideoInfo, playlistName string, playlistPosition int) (string, error) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	item := QueueItem{
		ID:               uuid.New().String(),
		VideoURL:         request.VideoURL,
		SpotifyURL:       request.SpotifyURL,
		Title:            videoInfo.Title,
		Artist:           videoInfo.Artist,
		Thumbnail:        videoInfo.Thumbnail,
		Duration:         videoInfo.Duration,
		PlaylistName:     playlistName,
		PlaylistPosition: playlistPosition,
		Status:           StatusPending,
		Progress:         0,
		Stage:            "Waiting...",
		CreatedAt:        time.Now(),
	}

	q.items = append(q.items, item)

	go q.emit(QueueEvent{
		Type:   "added",
		ItemID: item.ID,
		Item:   &item,
	})

	return item.ID, nil
}

// GetQueue returns all queue items
func (q *Queue) GetQueue() []QueueItem {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]QueueItem, len(q.items))
	copy(result, q.items)
	return result
}

// GetItem returns a specific queue item
func (q *Queue) GetItem(id string) *QueueItem {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	for i := range q.items {
		if q.items[i].ID == id {
			// Return a copy
			item := q.items[i]
			return &item
		}
	}
	return nil
}

// GetPendingCount returns the number of pending items
func (q *Queue) GetPendingCount() int {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	count := 0
	for _, item := range q.items {
		if item.Status == StatusPending {
			count++
		}
	}
	return count
}

// GetActiveCount returns the number of currently processing items
func (q *Queue) GetActiveCount() int {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	count := 0
	for _, item := range q.items {
		switch item.Status {
		case StatusFetchingInfo, StatusDownloadingVideo, StatusDownloadingAudio, StatusMuxing, StatusOrganizing:
			count++
		}
	}
	return count
}

// updateItem updates an item and emits event
func (q *Queue) updateItem(id string, updater func(*QueueItem)) {
	q.mutex.Lock()

	var updated *QueueItem
	for i := range q.items {
		if q.items[i].ID == id {
			updater(&q.items[i])
			item := q.items[i]
			updated = &item
			break
		}
	}

	q.mutex.Unlock()

	if updated != nil {
		q.emit(QueueEvent{
			Type:     "updated",
			ItemID:   id,
			Item:     updated,
			Progress: updated.Progress,
			Status:   updated.Status,
		})
	}
}

// UpdateStatus updates the status of a queue item
func (q *Queue) UpdateStatus(id string, status QueueStatus, progress int, stage string) {
	q.updateItem(id, func(item *QueueItem) {
		item.Status = status
		item.Progress = progress
		if stage != "" {
			item.Stage = stage
		}
		if status == StatusComplete {
			item.CompletedAt = time.Now()
		}
	})
}

// SetItemError sets an error on a queue item
func (q *Queue) SetItemError(id string, err error) {
	q.updateItem(id, func(item *QueueItem) {
		item.Status = StatusError
		item.Error = err.Error()
		item.Stage = "Error"
		item.CompletedAt = time.Now()
	})

	// Save to history as failed
	q.mutex.RLock()
	history := q.history
	q.mutex.RUnlock()
	if history != nil {
		item := q.GetItem(id)
		if item != nil {
			history.AddFromQueueItem(item, "error", err.Error())
		}
	}

	q.emit(QueueEvent{
		Type:   "error",
		ItemID: id,
		Error:  err.Error(),
	})
}

// SetItemOutput sets the output path for a completed item
func (q *Queue) SetItemOutput(id string, outputPath string) {
	q.updateItem(id, func(item *QueueItem) {
		item.OutputPath = outputPath
	})
}

// RemoveFromQueue removes an item from the queue
func (q *Queue) RemoveFromQueue(id string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	for i, item := range q.items {
		if item.ID == id {
			// Cancel if processing
			if item.cancelFunc != nil {
				item.cancelFunc()
			}
			q.items = append(q.items[:i], q.items[i+1:]...)

			go q.emit(QueueEvent{
				Type:   "removed",
				ItemID: id,
			})
			return nil
		}
	}
	return nil
}

// CancelItem cancels a processing item
func (q *Queue) CancelItem(id string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	for i := range q.items {
		if q.items[i].ID == id {
			if q.items[i].cancelFunc != nil {
				q.items[i].cancelFunc()
			}
			q.items[i].Status = StatusCancelled
			q.items[i].Stage = "Cancelled"
			return nil
		}
	}
	return fmt.Errorf("item not found: %s", id)
}

// ClearCompleted removes all completed items
func (q *Queue) ClearCompleted() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	filtered := make([]QueueItem, 0)
	removed := 0
	for _, item := range q.items {
		if item.Status != StatusComplete && item.Status != StatusError && item.Status != StatusCancelled {
			filtered = append(filtered, item)
		} else {
			removed++
		}
	}
	q.items = filtered
	return removed
}

// RetryFailed resets all failed items to pending for retry
func (q *Queue) RetryFailed() int {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	retried := 0
	for i := range q.items {
		if q.items[i].Status == StatusError {
			q.items[i].Status = StatusPending
			q.items[i].Progress = 0
			q.items[i].Error = ""
			q.items[i].Stage = "Waiting... (retry)"
			retried++

			item := q.items[i]
			go q.emit(QueueEvent{Type: "updated", ItemID: item.ID, Item: &item})
		}
	}
	return retried
}

// ClearAll removes all items from the queue
func (q *Queue) ClearAll() {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Cancel all processing items
	for _, item := range q.items {
		if item.cancelFunc != nil {
			item.cancelFunc()
		}
	}

	q.items = make([]QueueItem, 0)
}

// MoveItem moves an item to a new position in the queue
func (q *Queue) MoveItem(id string, newIndex int) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Find current index
	currentIndex := -1
	for i, item := range q.items {
		if item.ID == id {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return fmt.Errorf("item not found: %s", id)
	}

	if newIndex < 0 || newIndex >= len(q.items) {
		return fmt.Errorf("invalid index: %d", newIndex)
	}

	// Remove from current position
	item := q.items[currentIndex]
	q.items = append(q.items[:currentIndex], q.items[currentIndex+1:]...)

	// Insert at new position
	q.items = append(q.items[:newIndex], append([]QueueItem{item}, q.items[newIndex:]...)...)

	return nil
}

// =============================================================================
// Queue Processing (Worker Pool)
// =============================================================================

// StartProcessing starts the worker pool
func (q *Queue) StartProcessing() {
	q.processMutex.Lock()
	if q.processing {
		q.processMutex.Unlock()
		return
	}
	q.processing = true
	q.processMutex.Unlock()

	// Start workers
	for i := 0; i < q.maxConc; i++ {
		q.workerWG.Add(1)
		go q.worker(i)
	}

	// Start dispatcher
	go q.dispatcher()
}

// StopProcessing stops all workers
func (q *Queue) StopProcessing() {
	q.processMutex.Lock()
	if !q.processing {
		q.processMutex.Unlock()
		return
	}
	q.processMutex.Unlock()

	q.cancel()
	close(q.jobChan)
	q.workerWG.Wait()

	q.processMutex.Lock()
	q.processing = false
	q.processMutex.Unlock()
}

// dispatcher finds pending items and sends them to workers
func (q *Queue) dispatcher() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
			// Find pending items
			q.mutex.RLock()
			for _, item := range q.items {
				if item.Status == StatusPending {
					select {
					case q.jobChan <- item.ID:
					default:
						// Channel full, skip
					}
				}
			}
			q.mutex.RUnlock()
		}
	}
}

// worker processes items from the job channel
func (q *Queue) worker(workerID int) {
	defer q.workerWG.Done()

	for {
		select {
		case <-q.ctx.Done():
			return
		case itemID, ok := <-q.jobChan:
			if !ok {
				return
			}
			q.processItem(itemID)
		}
	}
}

// processItem processes a single queue item through the full pipeline
func (q *Queue) processItem(id string) {
	// Create cancellable context for this item
	itemCtx, cancel := context.WithCancel(q.ctx)

	// Store cancel func
	q.mutex.Lock()
	for i := range q.items {
		if q.items[i].ID == id {
			// Skip if not pending
			if q.items[i].Status != StatusPending {
				q.mutex.Unlock()
				cancel()
				return
			}
			q.items[i].cancelFunc = cancel
			q.items[i].Status = StatusFetchingInfo
			q.items[i].StartedAt = time.Now()
			q.items[i].Stage = "Fetching video info..."
			break
		}
	}
	q.mutex.Unlock()

	defer cancel()

	// Get item info
	item := q.GetItem(id)
	if item == nil {
		return
	}

	// Emit started event
	q.emit(QueueEvent{
		Type:   "updated",
		ItemID: id,
		Item:   item,
	})

	// Load config
	q.mutex.RLock()
	config := q.config
	q.mutex.RUnlock()

	if config == nil {
		config = &defaultConfig
	}

	// Create temp directory for this download
	tempDir := filepath.Join(os.TempDir(), "youflac", id)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		q.SetItemError(id, fmt.Errorf("failed to create temp dir: %w", err))
		return
	}
	defer os.RemoveAll(tempDir) // Cleanup on completion

	// ==========================================================================
	// Stage 1: Fetch Video Info (if not already present)
	// ==========================================================================
	var videoInfo *VideoInfo
	var videoID string
	var err error

	if item.Title == "" {
		// Normal flow: fetch from YouTube
		q.UpdateStatus(id, StatusFetchingInfo, 5, "Parsing URL...")

		videoID, err = ParseYouTubeURL(item.VideoURL)
		if err != nil {
			q.SetItemError(id, fmt.Errorf("invalid YouTube URL: %w", err))
			return
		}

		select {
		case <-itemCtx.Done():
			return
		default:
		}

		videoInfo, err = GetVideoMetadata(videoID)
		if err != nil {
			q.SetItemError(id, fmt.Errorf("failed to fetch video info: %w", err))
			return
		}

		// Update item with video info
		q.updateItem(id, func(item *QueueItem) {
			item.Title = videoInfo.Title
			item.Artist = videoInfo.Artist
			item.Thumbnail = videoInfo.Thumbnail
			item.Duration = videoInfo.Duration
		})
	} else {
		// Already have info (from import or previous fetch)
		if item.VideoURL != "" {
			videoID, _ = ParseYouTubeURL(item.VideoURL)
		}
		videoInfo = &VideoInfo{
			Title:     item.Title,
			Artist:    item.Artist,
			Thumbnail: item.Thumbnail,
			Duration:  item.Duration,
		}
	}

	// ==========================================================================
	// Stage 1.5: Check for Existing File (Skip Detection)
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	q.mutex.RLock()
	fileIndex := q.fileIndex
	q.mutex.RUnlock()

	if fileIndex != nil && videoInfo.Title != "" {
		existingFile := fileIndex.FindMatch(videoInfo.Title, videoInfo.Artist)
		if existingFile != nil {
			q.UpdateStatus(id, StatusOrganizing, 80, "Found existing file...")

			// Determine target path
			outputDir := config.OutputDirectory
			if outputDir == "" {
				outputDir = GetDefaultOutputDirectory()
			}

			// Get current item for playlist info
			item = q.GetItem(id)
			if item != nil && item.PlaylistName != "" {
				playlistFolder := SanitizeFileName(item.PlaylistName)
				outputDir = filepath.Join(outputDir, playlistFolder)
			}

			muxMetadata := &Metadata{
				Title:  videoInfo.Title,
				Artist: videoInfo.Artist,
				Track:  item.PlaylistPosition,
			}

			// Use original file extension when copying
			existingExt := filepath.Ext(existingFile.Path)
			if existingExt == "" {
				existingExt = ".mkv"
			}

			var targetPath string
			if item.PlaylistPosition > 0 {
				targetPath = GeneratePlaylistFilePath(muxMetadata, outputDir, existingExt)
			} else {
				targetPath = GenerateFilePath(muxMetadata, config.NamingTemplate, outputDir, existingExt)
			}

			// Check if it's the same path (already in correct location)
			if existingFile.Path == targetPath {
				// Already in correct location, just mark complete
				q.updateItem(id, func(item *QueueItem) {
					item.Status = StatusComplete
					item.Progress = 100
					item.Stage = "Skipped (already exists)"
					item.OutputPath = existingFile.Path
					item.CompletedAt = time.Now()
				})
				q.emit(QueueEvent{
					Type:     "completed",
					ItemID:   id,
					Progress: 100,
					Status:   StatusComplete,
				})
				fmt.Printf("[Queue] Skipped (already exists): %s\n", existingFile.Path)
				return
			}

			// Copy file to new location
			q.UpdateStatus(id, StatusOrganizing, 90, "Copying existing file...")
			if err := copyFile(existingFile.Path, targetPath); err == nil {
				// Update file index with new entry
				fileIndex.AddEntry(FileIndexEntry{
					Path:      targetPath,
					Title:     videoInfo.Title,
					Artist:    videoInfo.Artist,
					Duration:  existingFile.Duration,
					Size:      existingFile.Size,
					IndexedAt: time.Now(),
				})
				go fileIndex.Save()

				q.updateItem(id, func(item *QueueItem) {
					item.Status = StatusComplete
					item.Progress = 100
					item.Stage = "Copied from existing"
					item.OutputPath = targetPath
					item.CompletedAt = time.Now()
				})
				q.emit(QueueEvent{
					Type:     "completed",
					ItemID:   id,
					Progress: 100,
					Status:   StatusComplete,
				})
				fmt.Printf("[Queue] Copied from existing: %s -> %s\n", existingFile.Path, targetPath)
				return
			}
			// If copy fails, continue with normal download
			fmt.Printf("[Queue] Copy failed, proceeding with download\n")
		}
	}

	// ==========================================================================
	// Stage 2: Download Video
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	var videoPath string
	audioOnly := false

	// Download video from YouTube
	q.UpdateStatus(id, StatusDownloadingVideo, 10, "Downloading video...")

	videoPath, err = DownloadVideo(videoID, config.VideoQuality, tempDir, config.CookiesBrowser)
	if err != nil {
		// Don't fail immediately - try audio-only fallback
		fmt.Printf("[Queue] Video download failed: %v, trying audio-only fallback\n", err)
		q.UpdateStatus(id, StatusDownloadingAudio, 40, "Video unavailable, downloading audio only...")
		audioOnly = true
		videoPath = ""

		q.updateItem(id, func(item *QueueItem) {
			item.AudioOnly = true
		})
	} else {
		q.UpdateStatus(id, StatusDownloadingVideo, 40, "Video downloaded")
		fmt.Printf("[Queue] Video downloaded: %s\n", videoPath)

		q.updateItem(id, func(item *QueueItem) {
			item.VideoPath = videoPath
		})
	}

	// ==========================================================================
	// Stage 3: Find and Download Audio
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	q.UpdateStatus(id, StatusDownloadingAudio, 40, "Finding audio match...")

	audioPath := ""

	// Create metadata for NFO
	metadata := &Metadata{
		Title:    videoInfo.Title,
		Artist:   videoInfo.Artist,
		Duration: videoInfo.Duration,
	}

	// Try to find and download FLAC audio using multi-service cascade
	audioDownloaded := false

	// Initialize download services
	tidalHifiService := NewTidalHifiService()
	lucidaService := NewLucidaService()
	orpheusService := NewOrpheusDLService()

	// Get audio links via songlink
	if item.SpotifyURL != "" || item.VideoURL != "" {
		q.UpdateStatus(id, StatusDownloadingAudio, 45, "Resolving audio sources...")
		fmt.Printf("[Queue] Resolving audio sources for: %s\n", item.VideoURL)

		sourceURL := item.VideoURL
		if item.SpotifyURL != "" {
			sourceURL = item.SpotifyURL
		}

		fmt.Printf("[Queue] Calling ResolveMusicURL: %s\n", sourceURL)
		links, err := ResolveMusicURL(sourceURL)
		fmt.Printf("[Queue] ResolveMusicURL result: err=%v, links=%v\n", err, links != nil)
		if err == nil && links != nil {
			// Try each audio source in priority order
			for _, source := range config.AudioSourcePriority {
				select {
				case <-itemCtx.Done():
					return
				default:
				}

				var downloadURL string
				switch source {
				case "tidal":
					downloadURL = links.URLs.TidalURL
				case "qobuz":
					downloadURL = links.URLs.QobuzURL
				case "amazon":
					downloadURL = links.URLs.AmazonURL
				case "deezer":
					downloadURL = links.URLs.DeezerURL
				}

				if downloadURL == "" {
					continue
				}

				fmt.Printf("[Queue] Trying %s: %s\n", source, downloadURL)
				q.UpdateStatus(id, StatusDownloadingAudio, 50, fmt.Sprintf("Downloading from %s...", source))

				// Service cascade for FLAC download
				var result *AudioDownloadResult
				var downloadErr error

				// 1. Try TidalHifiService FIRST for Tidal URLs (vogel.qqdl.site - works!)
				if source == "tidal" && tidalHifiService.IsAvailable() {
					fmt.Printf("[Queue] Trying TidalHifi API for %s...\n", source)
					q.UpdateStatus(id, StatusDownloadingAudio, 51, "Downloading FLAC from Tidal...")
					result, downloadErr = tidalHifiService.Download(downloadURL, tempDir, "flac")
					if downloadErr != nil {
						fmt.Printf("[Queue] TidalHifi failed: %v\n", downloadErr)
					}
				}

				// 2. Try Lucida (web API) if TidalHifi failed or not Tidal
				if result == nil {
					fmt.Printf("[Queue] Trying Lucida for %s...\n", source)
					result, downloadErr = lucidaService.Download(downloadURL, tempDir, "flac")
					if downloadErr != nil {
						fmt.Printf("[Queue] Lucida failed: %v\n", downloadErr)
					}
				}

				// 3. Try OrpheusDL/Streamrip (Python subprocess) as last resort
				if result == nil && orpheusService.IsAvailable() {
					fmt.Printf("[Queue] Trying OrpheusDL/Streamrip for %s...\n", source)
					q.UpdateStatus(id, StatusDownloadingAudio, 52, fmt.Sprintf("Trying OrpheusDL for %s...", source))
					result, downloadErr = orpheusService.Download(downloadURL, tempDir, "flac")
					if downloadErr != nil {
						fmt.Printf("[Queue] OrpheusDL failed: %v\n", downloadErr)
					}
				}

				// Success!
				if result != nil {
					fmt.Printf("[Queue] FLAC downloaded from %s: %s\n", source, result.FilePath)
					audioDownloaded = true
					audioPath = result.FilePath
					q.updateItem(id, func(item *QueueItem) {
						item.AudioSource = source
						item.AudioPath = audioPath
					})
					break
				}
			}
		}
	}

	// If songlink resolution failed or no FLAC sources found, try TidalHifi search
	if !audioDownloaded && videoInfo.Artist != "" && videoInfo.Title != "" {
		fmt.Printf("[Queue] Trying TidalHifi search for: %s - %s\n", videoInfo.Artist, videoInfo.Title)
		q.UpdateStatus(id, StatusDownloadingAudio, 55, "Searching Tidal for track...")

		if tidalHifiService.IsAvailable() {
			result, err := tidalHifiService.DownloadBySearch(videoInfo.Artist, videoInfo.Title, tempDir)
			if err == nil && result != nil {
				fmt.Printf("[Queue] FLAC found via Tidal search: %s\n", result.FilePath)
				audioDownloaded = true
				audioPath = result.FilePath
				q.updateItem(id, func(item *QueueItem) {
					item.AudioSource = "tidal-search"
					item.AudioPath = audioPath
				})
			} else {
				fmt.Printf("[Queue] Tidal search failed: %v\n", err)
			}
		}
	}

	if !audioDownloaded {
		// Fallback: extract audio from video (only if video exists)
		if videoPath != "" {
			q.UpdateStatus(id, StatusDownloadingAudio, 55, "Extracting audio from video...")
			// Use .mka (Matroska audio) which supports any codec (opus, aac, etc.)
			audioPath = filepath.Join(tempDir, "audio.mka")

			err = ExtractAudioFromVideo(videoPath, audioPath)
			if err != nil {
				q.SetItemError(id, fmt.Errorf("failed to extract audio: %w", err))
				return
			}

			q.updateItem(id, func(item *QueueItem) {
				item.AudioSource = "extracted"
				item.AudioPath = audioPath
			})
		} else {
			// Audio-only mode but no audio was downloaded from services
			q.SetItemError(id, fmt.Errorf("failed to download audio: no audio source available and video unavailable"))
			return
		}
	}

	// ==========================================================================
	// Stage 4: Mux Video + Audio
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	q.UpdateStatus(id, StatusMuxing, 70, "Muxing video and audio...")

	// Determine output path
	outputDir := config.OutputDirectory
	if outputDir == "" {
		outputDir = GetDefaultOutputDirectory()
	}

	// Get current item for updated paths
	item = q.GetItem(id)

	// If item is part of a playlist, create playlist subfolder
	if item.PlaylistName != "" {
		playlistFolder := SanitizeFileName(item.PlaylistName)
		outputDir = filepath.Join(outputDir, playlistFolder)
	}

	// Create metadata for muxing
	muxMetadata := &Metadata{
		Title:     videoInfo.Title,
		Artist:    videoInfo.Artist,
		Album:     item.Album,
		Thumbnail: videoInfo.Thumbnail,
		Duration:  videoInfo.Duration,
		Track:     item.PlaylistPosition, // Use playlist position as track number
	}

	// Generate output path using naming template
	// Use .flac extension for audio-only, .mkv for video+audio
	outputExt := ".mkv"
	if audioOnly {
		outputExt = ".flac"
	}

	var outputPath string
	if item.PlaylistPosition > 0 {
		// Playlist item: use track number prefix format "01 - Artist - Title"
		outputPath = GeneratePlaylistFilePath(muxMetadata, outputDir, outputExt)
	} else {
		// Regular item: use configured naming template
		outputPath = GenerateFilePath(muxMetadata, config.NamingTemplate, outputDir, outputExt)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		q.SetItemError(id, fmt.Errorf("failed to create output directory: %w", err))
		return
	}

	// Check for conflicts
	if exists, _ := CheckFileConflict(outputPath); exists {
		outputPath = ResolveConflict(outputPath)
	}

	// Download cover if embedding
	var coverPath string
	if config.EmbedCoverArt && videoInfo.Thumbnail != "" {
		coverPath = filepath.Join(tempDir, "cover.jpg")
		if err := DownloadPoster(videoInfo.Thumbnail, coverPath); err != nil {
			coverPath = "" // Failed to download, proceed without cover
		}
	}

	var result *MuxResult
	if audioOnly {
		// Audio-only fallback: create FLAC file
		q.UpdateStatus(id, StatusMuxing, 80, "Creating FLAC file...")
		result, err = CreateFLACWithMetadata(item.AudioPath, outputPath, muxMetadata, coverPath)
		if err != nil {
			q.SetItemError(id, fmt.Errorf("failed to create FLAC: %w", err))
			return
		}
	} else {
		// Normal case: mux video + audio into MKV
		q.UpdateStatus(id, StatusMuxing, 80, "Creating MKV file...")
		result, err = MuxVideoWithFLAC(item.VideoPath, item.AudioPath, outputPath, muxMetadata, coverPath, nil)
		if err != nil {
			q.SetItemError(id, fmt.Errorf("failed to mux: %w", err))
			return
		}
	}

	// ==========================================================================
	// Stage 4.5: Fetch and Embed Lyrics (if enabled)
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	if config.LyricsEnabled && videoInfo.Artist != "" && videoInfo.Title != "" {
		q.UpdateStatus(id, StatusOrganizing, 85, "Fetching lyrics...")

		lyrics, lyricsErr := FetchLyrics(videoInfo.Artist, videoInfo.Title)
		if lyricsErr == nil && lyrics != nil {
			embedMode := LyricsEmbedMode(config.LyricsEmbedMode)
			if embedMode == "" {
				embedMode = LyricsEmbedLRC // Default to LRC file
			}

			switch embedMode {
			case LyricsEmbedFile:
				if err := EmbedLyricsInFile(result.OutputPath, lyrics); err != nil {
					fmt.Printf("[Queue] Warning: failed to embed lyrics: %v\n", err)
				} else {
					fmt.Printf("[Queue] Lyrics embedded in file\n")
				}
			case LyricsEmbedLRC:
				if lyrics.HasSync {
					if lrcPath, err := SaveLRCFile(lyrics, result.OutputPath); err != nil {
						fmt.Printf("[Queue] Warning: failed to save LRC file: %v\n", err)
					} else {
						fmt.Printf("[Queue] LRC file saved: %s\n", lrcPath)
					}
				} else if lyrics.PlainText != "" {
					if txtPath, err := SavePlainLyricsFile(lyrics, result.OutputPath); err != nil {
						fmt.Printf("[Queue] Warning: failed to save lyrics file: %v\n", err)
					} else {
						fmt.Printf("[Queue] Lyrics file saved: %s\n", txtPath)
					}
				}
			case LyricsEmbedBoth:
				// Save LRC/TXT file
				if lyrics.HasSync {
					if lrcPath, err := SaveLRCFile(lyrics, result.OutputPath); err == nil {
						fmt.Printf("[Queue] LRC file saved: %s\n", lrcPath)
					}
				} else if lyrics.PlainText != "" {
					if txtPath, err := SavePlainLyricsFile(lyrics, result.OutputPath); err == nil {
						fmt.Printf("[Queue] Lyrics file saved: %s\n", txtPath)
					}
				}
				// Also embed in file
				if err := EmbedLyricsInFile(result.OutputPath, lyrics); err != nil {
					fmt.Printf("[Queue] Warning: failed to embed lyrics: %v\n", err)
				} else {
					fmt.Printf("[Queue] Lyrics embedded in file\n")
				}
			}
		} else if lyricsErr != nil {
			fmt.Printf("[Queue] Lyrics not found: %v\n", lyricsErr)
		}
	}

	// ==========================================================================
	// Stage 5: Organize and Generate NFO
	// ==========================================================================
	select {
	case <-itemCtx.Done():
		return
	default:
	}

	q.UpdateStatus(id, StatusOrganizing, 90, "Organizing files...")

	// Generate NFO if enabled
	if config.GenerateNFO {
		nfoPath := outputPath[:len(outputPath)-4] + ".nfo"
		nfoOpts := &NFOOptions{
			IncludeFileInfo: true,
		}

		// Get file info for NFO
		if mediaInfo, err := GetMediaInfo(result.OutputPath); err == nil {
			nfoOpts.MediaInfo = mediaInfo
		}

		if err := WriteNFO(metadata, nfoPath, nfoOpts); err != nil {
			// Non-fatal, just log
			fmt.Printf("Warning: failed to write NFO: %v\n", err)
		}
	}

	// Download poster alongside MKV
	if videoInfo.Thumbnail != "" {
		posterPath := outputPath[:len(outputPath)-4] + "-poster.jpg"
		DownloadPoster(videoInfo.Thumbnail, posterPath) // Ignore error, non-fatal
	}

	// ==========================================================================
	// Complete
	// ==========================================================================

	// Add completed file to index for future duplicate detection
	q.mutex.RLock()
	fi := q.fileIndex
	q.mutex.RUnlock()
	if fi != nil && videoInfo != nil {
		stat, _ := os.Stat(result.OutputPath)
		var fileSize int64
		if stat != nil {
			fileSize = stat.Size()
		}
		fi.AddEntry(FileIndexEntry{
			Path:      result.OutputPath,
			Title:     videoInfo.Title,
			Artist:    videoInfo.Artist,
			Duration:  videoInfo.Duration,
			Size:      fileSize,
			IndexedAt: time.Now(),
		})
		go fi.Save()
	}

	// Get file size for history
	var fileSize int64
	if stat, err := os.Stat(result.OutputPath); err == nil {
		fileSize = stat.Size()
	}

	q.updateItem(id, func(item *QueueItem) {
		item.Status = StatusComplete
		item.Progress = 100
		item.Stage = "Complete"
		item.OutputPath = result.OutputPath
		item.FileSize = fileSize
		item.CompletedAt = time.Now()
	})

	// Save to history
	q.mutex.RLock()
	history := q.history
	q.mutex.RUnlock()
	if history != nil {
		item = q.GetItem(id)
		if item != nil {
			history.AddFromQueueItem(item, "complete", "")
		}
	}

	q.emit(QueueEvent{
		Type:     "completed",
		ItemID:   id,
		Progress: 100,
		Status:   StatusComplete,
	})
}

// ExtractAudioFromVideo extracts audio track from video file
func ExtractAudioFromVideo(videoPath, audioPath string) error {
	// Use ExtractAudioStream from ffmpeg.go
	return ExtractAudioStream(videoPath, audioPath)
}

// =============================================================================
// Persistence (JSON)
// =============================================================================

// QueueState represents the serializable state of the queue
type QueueState struct {
	Items     []QueueItem `json:"items"`
	UpdatedAt time.Time   `json:"updatedAt"`
}

// GetQueueFilePath returns the path to the queue state file
func GetQueueFilePath() string {
	return filepath.Join(GetDataPath(), "queue.json")
}

// SaveQueue persists the queue to disk
func (q *Queue) SaveQueue() error {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	state := QueueState{
		Items:     q.items,
		UpdatedAt: time.Now(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal queue: %w", err)
	}

	queuePath := GetQueueFilePath()
	if err := os.MkdirAll(filepath.Dir(queuePath), 0755); err != nil {
		return fmt.Errorf("failed to create queue directory: %w", err)
	}

	if err := os.WriteFile(queuePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write queue file: %w", err)
	}

	return nil
}

// LoadQueue loads the queue from disk
func (q *Queue) LoadQueue() error {
	queuePath := GetQueueFilePath()

	data, err := os.ReadFile(queuePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No queue file, start fresh
		}
		return fmt.Errorf("failed to read queue file: %w", err)
	}

	var state QueueState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal queue: %w", err)
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Reset in-progress items to pending (they were interrupted)
	for i := range state.Items {
		switch state.Items[i].Status {
		case StatusFetchingInfo, StatusDownloadingVideo, StatusDownloadingAudio, StatusMuxing, StatusOrganizing:
			state.Items[i].Status = StatusPending
			state.Items[i].Progress = 0
			state.Items[i].Stage = "Waiting... (resumed)"
		}
	}

	q.items = state.Items
	return nil
}

// AutoSave starts periodic auto-saving of the queue
func (q *Queue) AutoSave(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-q.ctx.Done():
				// Final save on shutdown
				q.SaveQueue()
				return
			case <-ticker.C:
				q.SaveQueue()
			}
		}
	}()
}

// =============================================================================
// Helper Functions
// =============================================================================

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return destFile.Sync()
}

// =============================================================================
// Queue Statistics
// =============================================================================

// QueueStats provides statistics about the queue
type QueueStats struct {
	Total       int `json:"total"`
	Pending     int `json:"pending"`
	Active      int `json:"active"`
	Completed   int `json:"completed"`
	Failed      int `json:"failed"`
	Cancelled   int `json:"cancelled"`
}

// GetStats returns queue statistics
func (q *Queue) GetStats() QueueStats {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	stats := QueueStats{
		Total: len(q.items),
	}

	for _, item := range q.items {
		switch item.Status {
		case StatusPending:
			stats.Pending++
		case StatusFetchingInfo, StatusDownloadingVideo, StatusDownloadingAudio, StatusMuxing, StatusOrganizing:
			stats.Active++
		case StatusComplete:
			stats.Completed++
		case StatusError:
			stats.Failed++
		case StatusCancelled:
			stats.Cancelled++
		}
	}

	return stats
}
