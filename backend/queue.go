package backend

import (
	"context"
	"fmt"
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
	ID               string      `json:"id"`
	VideoURL         string      `json:"videoUrl"`
	SpotifyURL       string      `json:"spotifyUrl,omitempty"`
	Title            string      `json:"title"`
	Artist           string      `json:"artist"`
	Album            string      `json:"album,omitempty"`
	PlaylistName     string      `json:"playlistName,omitempty"`     // Playlist folder name
	PlaylistPosition int         `json:"playlistPosition,omitempty"` // Position in playlist (1-based)
	Thumbnail        string      `json:"thumbnail,omitempty"`
	Duration         float64     `json:"duration,omitempty"`
	Status           QueueStatus `json:"status"`
	Progress         int         `json:"progress"` // 0-100
	Stage            string      `json:"stage"`    // Human-readable current stage
	Error            string      `json:"error,omitempty"`
	OutputPath       string      `json:"outputPath,omitempty"`
	VideoPath        string      `json:"videoPath,omitempty"` // Temp video file
	AudioPath        string      `json:"audioPath,omitempty"` // Temp audio file
	FileSize         int64       `json:"fileSize,omitempty"`  // Output file size
	CreatedAt        time.Time   `json:"createdAt"`
	StartedAt        time.Time   `json:"startedAt,omitempty"`
	CompletedAt      time.Time   `json:"completedAt,omitempty"`

	// Matching info
	MatchScore      int    `json:"matchScore,omitempty"`
	MatchConfidence string `json:"matchConfidence,omitempty"`
	AudioSource     string `json:"audioSource,omitempty"`   // tidal, qobuz, amazon, etc.
	Quality         string `json:"quality,omitempty"`       // Requested quality tier
	ActualQuality   string `json:"actualQuality,omitempty"` // Actual quality obtained (may differ from requested)

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

// GetFailedItems returns all queue items that have Status == StatusError.
func (q *Queue) GetFailedItems() []QueueItem {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	var failed []QueueItem
	for _, item := range q.items {
		if item.Status == StatusError {
			failed = append(failed, item)
		}
	}
	return failed
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
