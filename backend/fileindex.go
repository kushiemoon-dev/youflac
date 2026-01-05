package backend

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// FileIndexEntry represents a single indexed file
type FileIndexEntry struct {
	Path      string    `json:"path"`
	Title     string    `json:"title"`
	Artist    string    `json:"artist"`
	Album     string    `json:"album,omitempty"`
	Duration  float64   `json:"duration,omitempty"`
	Size      int64     `json:"size"`
	IndexedAt time.Time `json:"indexedAt"`
}

// NormalizedKey is used for matching (lowercase, sanitized)
type NormalizedKey struct {
	Title  string
	Artist string
}

// FileIndex maintains an index of existing files for duplicate detection
type FileIndex struct {
	entries   map[NormalizedKey][]FileIndexEntry
	mutex     sync.RWMutex
	indexPath string
	dirty     bool
}

// NewFileIndex creates a new file index
func NewFileIndex(dataPath string) *FileIndex {
	return &FileIndex{
		entries:   make(map[NormalizedKey][]FileIndexEntry),
		indexPath: filepath.Join(dataPath, "fileindex.json"),
	}
}

// NormalizeForMatching creates a normalized key for matching
func NormalizeForMatching(title, artist string) NormalizedKey {
	return NormalizedKey{
		Title:  normalizeString(title),
		Artist: normalizeString(artist),
	}
}

// normalizeString lowercases and removes special chars for matching
func normalizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)

	// Remove common suffixes like "(Official Video)", "(HD)", "[Official]", etc.
	suffixPatterns := []string{
		`\s*\(official\s*(video|audio|music\s*video|mv|lyric\s*video|visualizer)?\)`,
		`\s*\[official\s*(video|audio|music\s*video|mv|lyric\s*video|visualizer)?\]`,
		`\s*\(hd\)`,
		`\s*\[hd\]`,
		`\s*\(hq\)`,
		`\s*\[hq\]`,
		`\s*\(4k\)`,
		`\s*\[4k\]`,
		`\s*\(lyrics?\)`,
		`\s*\[lyrics?\]`,
		`\s*\(audio\)`,
		`\s*\[audio\]`,
		`\s*\(visualizer\)`,
		`\s*\[visualizer\]`,
		`\s*\(remaster(ed)?\)`,
		`\s*\[remaster(ed)?\]`,
		`\s*-\s*topic$`,
	}

	for _, pattern := range suffixPatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		s = re.ReplaceAllString(s, "")
	}

	// Remove special characters but keep alphanumeric, spaces, and common punctuation
	re := regexp.MustCompile(`[^\p{L}\p{N}\s'-]`)
	s = re.ReplaceAllString(s, "")

	// Normalize multiple spaces
	re = regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")

	return strings.TrimSpace(s)
}

// ScanDirectory scans a directory and indexes all MKV files
func (fi *FileIndex) ScanDirectory(dir string) error {
	fi.mutex.Lock()
	defer fi.mutex.Unlock()

	// Walk the directory recursively
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".mkv" && ext != ".mp4" {
			return nil
		}

		entry := fi.extractMetadataFromFile(path)
		if entry != nil {
			key := NormalizeForMatching(entry.Title, entry.Artist)
			fi.entries[key] = append(fi.entries[key], *entry)
			fi.dirty = true
		}
		return nil
	})
}

// extractMetadataFromFile extracts title/artist from MKV file
func (fi *FileIndex) extractMetadataFromFile(path string) *FileIndexEntry {
	entry := &FileIndexEntry{
		Path:      path,
		IndexedAt: time.Now(),
	}

	// Get file info
	if stat, err := os.Stat(path); err == nil {
		entry.Size = stat.Size()
	}

	// Try to extract embedded metadata using ffprobe
	metadata := extractMKVTags(path)
	if metadata != nil {
		entry.Title = metadata["title"]
		entry.Artist = metadata["artist"]
		entry.Album = metadata["album"]
	}

	// Fallback: parse from filename using naming patterns
	if entry.Title == "" || entry.Artist == "" {
		title, artist := ParseFilename(path)
		if entry.Title == "" {
			entry.Title = title
		}
		if entry.Artist == "" {
			entry.Artist = artist
		}
	}

	// Skip if we couldn't determine title
	if entry.Title == "" {
		return nil
	}

	return entry
}

// extractMKVTags uses ffprobe to extract embedded tags
func extractMKVTags(path string) map[string]string {
	ffprobePath := GetFFprobePath()
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		path,
	}

	cmd := exec.Command(ffprobePath, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil
	}

	var probeData struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &probeData); err != nil {
		return nil
	}

	// Normalize tag keys to lowercase
	result := make(map[string]string)
	for k, v := range probeData.Format.Tags {
		result[strings.ToLower(k)] = v
	}
	return result
}

// ParseFilename extracts title and artist from filename
// Handles patterns like "Artist - Title.mkv" or "Artist/Title/Title.mkv"
func ParseFilename(path string) (title, artist string) {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	// Try "Artist - Title" pattern
	if parts := strings.SplitN(name, " - ", 2); len(parts) == 2 {
		return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0])
	}

	// Try to get from directory structure (Jellyfin style: Artist/Title/Title.mkv)
	dir := filepath.Dir(path)
	parentDir := filepath.Base(dir)
	grandParentDir := filepath.Base(filepath.Dir(dir))

	// If parent dir matches filename, grandparent might be artist
	if strings.EqualFold(parentDir, name) && grandParentDir != "." && grandParentDir != "" {
		return name, grandParentDir
	}

	// Fallback: just use filename as title
	return name, ""
}

// FindMatch looks for an existing file matching title + artist
func (fi *FileIndex) FindMatch(title, artist string) *FileIndexEntry {
	fi.mutex.RLock()
	defer fi.mutex.RUnlock()

	key := NormalizeForMatching(title, artist)
	entries, exists := fi.entries[key]
	if !exists || len(entries) == 0 {
		return nil
	}

	// Verify file still exists
	for _, entry := range entries {
		if _, err := os.Stat(entry.Path); err == nil {
			return &entry
		}
	}

	return nil
}

// AddEntry adds a new entry to the index
func (fi *FileIndex) AddEntry(entry FileIndexEntry) {
	fi.mutex.Lock()
	defer fi.mutex.Unlock()

	key := NormalizeForMatching(entry.Title, entry.Artist)
	fi.entries[key] = append(fi.entries[key], entry)
	fi.dirty = true
}

// Save persists the index to disk
func (fi *FileIndex) Save() error {
	fi.mutex.Lock()
	defer fi.mutex.Unlock()

	if !fi.dirty {
		return nil
	}

	// Convert map to slice for JSON
	var allEntries []FileIndexEntry
	for _, entries := range fi.entries {
		allEntries = append(allEntries, entries...)
	}

	data, err := json.MarshalIndent(allEntries, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(fi.indexPath), 0755); err != nil {
		return err
	}

	fi.dirty = false
	return os.WriteFile(fi.indexPath, data, 0644)
}

// Load loads the index from disk
func (fi *FileIndex) Load() error {
	data, err := os.ReadFile(fi.indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var entries []FileIndexEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	fi.mutex.Lock()
	defer fi.mutex.Unlock()

	fi.entries = make(map[NormalizedKey][]FileIndexEntry)
	for _, entry := range entries {
		key := NormalizeForMatching(entry.Title, entry.Artist)
		fi.entries[key] = append(fi.entries[key], entry)
	}

	return nil
}

// Count returns the number of indexed files
func (fi *FileIndex) Count() int {
	fi.mutex.RLock()
	defer fi.mutex.RUnlock()

	count := 0
	for _, entries := range fi.entries {
		count += len(entries)
	}
	return count
}
