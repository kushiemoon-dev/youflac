package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// HistoryEntry represents a completed or failed download
type HistoryEntry struct {
	ID          string    `json:"id"`
	VideoURL    string    `json:"videoUrl"`
	Title       string    `json:"title"`
	Artist      string    `json:"artist"`
	AudioSource string    `json:"audioSource"` // tidal, qobuz, amazon, extracted
	Quality     string    `json:"quality"`
	OutputPath  string    `json:"outputPath"`
	Thumbnail   string    `json:"thumbnail,omitempty"`
	Duration    float64   `json:"duration,omitempty"`
	FileSize    int64     `json:"fileSize"`
	CompletedAt time.Time `json:"completedAt"`
	Status      string    `json:"status"` // complete, error
	Error       string    `json:"error,omitempty"`
}

// History manages the download history
type History struct {
	entries  []HistoryEntry
	filePath string
	mu       sync.RWMutex
}

// NewHistory creates a new History manager
func NewHistory() *History {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = os.TempDir()
	}

	historyPath := filepath.Join(configDir, "youflac", "history.json")

	h := &History{
		entries:  []HistoryEntry{},
		filePath: historyPath,
	}

	h.load()
	return h
}

// load reads history from disk
func (h *History) load() {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := os.ReadFile(h.filePath)
	if err != nil {
		// File doesn't exist or can't be read, start with empty history
		h.entries = []HistoryEntry{}
		return
	}

	if err := json.Unmarshal(data, &h.entries); err != nil {
		h.entries = []HistoryEntry{}
	}
}

// save writes history to disk
func (h *History) save() error {
	// Ensure directory exists
	dir := filepath.Dir(h.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(h.entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(h.filePath, data, 0644)
}

// Add adds a new entry to history
func (h *History) Add(entry HistoryEntry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.CompletedAt.IsZero() {
		entry.CompletedAt = time.Now()
	}

	// Prepend to keep newest first
	h.entries = append([]HistoryEntry{entry}, h.entries...)

	return h.save()
}

// AddFromQueueItem creates a history entry from a completed queue item
func (h *History) AddFromQueueItem(item *QueueItem, status string, errorMsg string) error {
	entry := HistoryEntry{
		ID:          uuid.New().String(),
		VideoURL:    item.VideoURL,
		Title:       item.Title,
		Artist:      item.Artist,
		AudioSource: item.AudioSource,
		Quality:     item.Quality,
		OutputPath:  item.OutputPath,
		Thumbnail:   item.Thumbnail,
		Duration:    item.Duration,
		FileSize:    item.FileSize,
		CompletedAt: time.Now(),
		Status:      status,
		Error:       errorMsg,
	}

	return h.Add(entry)
}

// GetAll returns all history entries
func (h *History) GetAll() []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Return a copy
	result := make([]HistoryEntry, len(h.entries))
	copy(result, h.entries)
	return result
}

// Search searches history by title or artist
func (h *History) Search(query string) []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	query = strings.ToLower(query)
	var results []HistoryEntry

	for _, entry := range h.entries {
		if strings.Contains(strings.ToLower(entry.Title), query) ||
			strings.Contains(strings.ToLower(entry.Artist), query) {
			results = append(results, entry)
		}
	}

	return results
}

// FilterBySource returns entries filtered by audio source
func (h *History) FilterBySource(source string) []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var results []HistoryEntry
	for _, entry := range h.entries {
		if entry.AudioSource == source {
			results = append(results, entry)
		}
	}

	return results
}

// FilterByStatus returns entries filtered by status
func (h *History) FilterByStatus(status string) []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var results []HistoryEntry
	for _, entry := range h.entries {
		if entry.Status == status {
			results = append(results, entry)
		}
	}

	return results
}

// GetByID returns a single entry by ID
func (h *History) GetByID(id string) *HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, entry := range h.entries {
		if entry.ID == id {
			entryCopy := entry
			return &entryCopy
		}
	}

	return nil
}

// Delete removes an entry by ID
func (h *History) Delete(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, entry := range h.entries {
		if entry.ID == id {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			return h.save()
		}
	}

	return nil
}

// Clear removes all history entries
func (h *History) Clear() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.entries = []HistoryEntry{}
	return h.save()
}

// GetStats returns statistics about the history
func (h *History) GetStats() HistoryStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := HistoryStats{
		SourceCounts: make(map[string]int),
	}

	for _, entry := range h.entries {
		stats.Total++
		if entry.Status == "complete" {
			stats.Completed++
		} else if entry.Status == "error" {
			stats.Failed++
		}
		stats.TotalSize += entry.FileSize

		if entry.AudioSource != "" {
			stats.SourceCounts[entry.AudioSource]++
		}
	}

	return stats
}

// HistoryStats contains aggregated history statistics
type HistoryStats struct {
	Total        int            `json:"total"`
	Completed    int            `json:"completed"`
	Failed       int            `json:"failed"`
	TotalSize    int64          `json:"totalSize"`
	SourceCounts map[string]int `json:"sourceCounts"`
}

// GetGroupedByDate returns history entries grouped by date
func (h *History) GetGroupedByDate() map[string][]HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	grouped := make(map[string][]HistoryEntry)

	for _, entry := range h.entries {
		dateKey := entry.CompletedAt.Format("2006-01-02")
		grouped[dateKey] = append(grouped[dateKey], entry)
	}

	return grouped
}

// GetRecent returns the most recent N entries
func (h *History) GetRecent(limit int) []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Entries are already sorted by newest first
	if limit >= len(h.entries) {
		result := make([]HistoryEntry, len(h.entries))
		copy(result, h.entries)
		return result
	}

	result := make([]HistoryEntry, limit)
	copy(result, h.entries[:limit])
	return result
}

// SortByDate sorts entries by date (newest first by default)
func (h *History) SortByDate(ascending bool) []HistoryEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]HistoryEntry, len(h.entries))
	copy(result, h.entries)

	sort.Slice(result, func(i, j int) bool {
		if ascending {
			return result[i].CompletedAt.Before(result[j].CompletedAt)
		}
		return result[i].CompletedAt.After(result[j].CompletedAt)
	})

	return result
}
