package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// LyricsResult contains fetched lyrics
type LyricsResult struct {
	PlainText    string `json:"plainText"`
	SyncedLyrics string `json:"syncedLyrics,omitempty"` // LRC format
	Source       string `json:"source"`
	HasSync      bool   `json:"hasSync"`
	TrackName    string `json:"trackName,omitempty"`
	ArtistName   string `json:"artistName,omitempty"`
	AlbumName    string `json:"albumName,omitempty"`
	Duration     int    `json:"duration,omitempty"`
}

// LyricsEmbedMode defines how lyrics should be saved
type LyricsEmbedMode string

const (
	LyricsEmbedFile LyricsEmbedMode = "embed" // Embed in audio/video file
	LyricsEmbedLRC  LyricsEmbedMode = "lrc"   // Save as separate .lrc file
	LyricsEmbedBoth LyricsEmbedMode = "both"  // Both methods
)

// LRCLIB API types
type lrclibSearchResult struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// lyricsHTTPClient is a dedicated HTTP client for lyrics API calls
var lyricsHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

// FetchLyrics fetches lyrics from LRCLIB for a track
func FetchLyrics(artist, title string) (*LyricsResult, error) {
	return FetchLyricsWithAlbum(artist, title, "")
}

// FetchLyricsWithAlbum fetches lyrics with album context for better matching
func FetchLyricsWithAlbum(artist, title, album string) (*LyricsResult, error) {
	if artist == "" || title == "" {
		return nil, fmt.Errorf("artist and title are required")
	}

	// Clean up the search terms
	artist = cleanSearchTerm(artist)
	title = cleanSearchTerm(title)

	// Try LRCLIB search
	result, err := searchLRCLIB(artist, title, album)
	if err == nil && result != nil {
		return result, nil
	}

	// If search fails, try direct get endpoint
	result, err = getLRCLIBDirect(artist, title, album)
	if err == nil && result != nil {
		return result, nil
	}

	return nil, fmt.Errorf("lyrics not found for %s - %s", artist, title)
}

// FetchLyricsByDuration fetches lyrics matching a specific duration
func FetchLyricsByDuration(artist, title, album string, durationSec int) (*LyricsResult, error) {
	if artist == "" || title == "" {
		return nil, fmt.Errorf("artist and title are required")
	}

	artist = cleanSearchTerm(artist)
	title = cleanSearchTerm(title)

	// Use LRCLIB get endpoint with duration
	baseURL := "https://lrclib.net/api/get"

	params := url.Values{}
	params.Set("artist_name", artist)
	params.Set("track_name", title)
	if album != "" {
		params.Set("album_name", album)
	}
	params.Set("duration", fmt.Sprintf("%d", durationSec))

	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "YouFlac/1.0 (https://github.com/youflac)")

	resp, err := lyricsHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LRCLIB request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("lyrics not found")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("LRCLIB error: %d", resp.StatusCode)
	}

	var lrcResult lrclibSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&lrcResult); err != nil {
		return nil, fmt.Errorf("failed to parse LRCLIB response: %w", err)
	}

	return convertLRCLIBResult(&lrcResult), nil
}

// searchLRCLIB searches for lyrics using LRCLIB search API
func searchLRCLIB(artist, title, album string) (*LyricsResult, error) {
	baseURL := "https://lrclib.net/api/search"

	// Build search query
	query := fmt.Sprintf("%s %s", artist, title)

	params := url.Values{}
	params.Set("q", query)

	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "YouFlac/1.0 (https://github.com/youflac)")

	resp, err := lyricsHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LRCLIB search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("LRCLIB error: %d", resp.StatusCode)
	}

	var results []lrclibSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to parse LRCLIB response: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no results found")
	}

	// Find best match
	var bestResult *lrclibSearchResult
	for i := range results {
		r := &results[i]
		// Skip instrumental tracks
		if r.Instrumental {
			continue
		}
		// Skip if no lyrics
		if r.PlainLyrics == "" && r.SyncedLyrics == "" {
			continue
		}
		// Prefer synced lyrics
		if bestResult == nil || (r.SyncedLyrics != "" && bestResult.SyncedLyrics == "") {
			bestResult = r
		}
	}

	if bestResult == nil {
		return nil, fmt.Errorf("no suitable lyrics found")
	}

	return convertLRCLIBResult(bestResult), nil
}

// getLRCLIBDirect tries the direct get endpoint
func getLRCLIBDirect(artist, title, album string) (*LyricsResult, error) {
	baseURL := "https://lrclib.net/api/get"

	params := url.Values{}
	params.Set("artist_name", artist)
	params.Set("track_name", title)
	if album != "" {
		params.Set("album_name", album)
	}

	reqURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "YouFlac/1.0 (https://github.com/youflac)")

	resp, err := lyricsHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LRCLIB request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("lyrics not found")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("LRCLIB error: %d", resp.StatusCode)
	}

	var lrcResult lrclibSearchResult
	if err := json.NewDecoder(resp.Body).Decode(&lrcResult); err != nil {
		return nil, fmt.Errorf("failed to parse LRCLIB response: %w", err)
	}

	return convertLRCLIBResult(&lrcResult), nil
}

func convertLRCLIBResult(r *lrclibSearchResult) *LyricsResult {
	return &LyricsResult{
		PlainText:    r.PlainLyrics,
		SyncedLyrics: r.SyncedLyrics,
		Source:       "lrclib",
		HasSync:      r.SyncedLyrics != "",
		TrackName:    r.TrackName,
		ArtistName:   r.ArtistName,
		AlbumName:    r.AlbumName,
		Duration:     int(r.Duration),
	}
}

func cleanSearchTerm(s string) string {
	// Remove common suffixes that interfere with search
	s = strings.TrimSpace(s)

	// Remove featuring artist notation
	for _, sep := range []string{" ft.", " ft ", " feat.", " feat ", " featuring "} {
		if idx := strings.Index(strings.ToLower(s), sep); idx > 0 {
			s = s[:idx]
		}
	}

	// Remove content in parentheses (remixes, versions, etc.)
	// Only if there's content before the parenthesis
	if idx := strings.Index(s, "("); idx > 3 {
		s = strings.TrimSpace(s[:idx])
	}

	return s
}

// SaveLRCFile saves synced lyrics to an LRC file
func SaveLRCFile(lyrics *LyricsResult, mediaFilePath string) (string, error) {
	if lyrics.SyncedLyrics == "" {
		return "", fmt.Errorf("no synced lyrics available")
	}

	// Create .lrc file path (same as media file but with .lrc extension)
	ext := filepath.Ext(mediaFilePath)
	lrcPath := mediaFilePath[:len(mediaFilePath)-len(ext)] + ".lrc"

	// Build LRC file content with metadata header
	var content strings.Builder

	// LRC metadata tags
	if lyrics.TrackName != "" {
		content.WriteString(fmt.Sprintf("[ti:%s]\n", lyrics.TrackName))
	}
	if lyrics.ArtistName != "" {
		content.WriteString(fmt.Sprintf("[ar:%s]\n", lyrics.ArtistName))
	}
	if lyrics.AlbumName != "" {
		content.WriteString(fmt.Sprintf("[al:%s]\n", lyrics.AlbumName))
	}
	if lyrics.Duration > 0 {
		mins := lyrics.Duration / 60
		secs := lyrics.Duration % 60
		content.WriteString(fmt.Sprintf("[length:%02d:%02d]\n", mins, secs))
	}
	content.WriteString("[by:YouFlac]\n")
	content.WriteString("[re:LRCLIB]\n\n")

	// Add synced lyrics
	content.WriteString(lyrics.SyncedLyrics)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(lrcPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(lrcPath, []byte(content.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write LRC file: %w", err)
	}

	return lrcPath, nil
}

// SavePlainLyricsFile saves plain text lyrics to a .txt file
func SavePlainLyricsFile(lyrics *LyricsResult, mediaFilePath string) (string, error) {
	if lyrics.PlainText == "" {
		return "", fmt.Errorf("no plain lyrics available")
	}

	ext := filepath.Ext(mediaFilePath)
	txtPath := mediaFilePath[:len(mediaFilePath)-len(ext)] + ".txt"

	// Build content
	var content strings.Builder

	// Header
	if lyrics.TrackName != "" && lyrics.ArtistName != "" {
		content.WriteString(fmt.Sprintf("%s - %s\n", lyrics.ArtistName, lyrics.TrackName))
		content.WriteString(strings.Repeat("-", 40) + "\n\n")
	}

	content.WriteString(lyrics.PlainText)

	if err := os.MkdirAll(filepath.Dir(txtPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(txtPath, []byte(content.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write lyrics file: %w", err)
	}

	return txtPath, nil
}

// EmbedLyricsInFile embeds lyrics into a media file using ffmpeg
// Supports FLAC (vorbis comments) and MKV (subtitle track)
func EmbedLyricsInFile(mediaPath string, lyrics *LyricsResult) error {
	ext := strings.ToLower(filepath.Ext(mediaPath))

	switch ext {
	case ".flac":
		return embedLyricsInFLAC(mediaPath, lyrics)
	case ".mkv":
		return embedLyricsInMKV(mediaPath, lyrics)
	default:
		return fmt.Errorf("unsupported format for lyrics embedding: %s", ext)
	}
}

// embedLyricsInFLAC adds lyrics as a FLAC vorbis comment
func embedLyricsInFLAC(flacPath string, lyrics *LyricsResult) error {
	ffmpegPath := GetFFmpegPath()

	// Create temp file
	tempPath := flacPath + ".tmp"

	// Use the synced lyrics if available, otherwise plain
	lyricsText := lyrics.SyncedLyrics
	if lyricsText == "" {
		lyricsText = lyrics.PlainText
	}

	if lyricsText == "" {
		return fmt.Errorf("no lyrics to embed")
	}

	// FFmpeg args to copy and add lyrics metadata
	args := []string{
		"-y",
		"-i", flacPath,
		"-c", "copy",
		"-metadata", fmt.Sprintf("LYRICS=%s", lyricsText),
		tempPath,
	}

	// If we have synced lyrics, also add as UNSYNCEDLYRICS for compatibility
	if lyrics.SyncedLyrics != "" && lyrics.PlainText != "" {
		args = append(args[:len(args)-1],
			"-metadata", fmt.Sprintf("UNSYNCEDLYRICS=%s", lyrics.PlainText),
			tempPath,
		)
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("ffmpeg failed: %v - %s", err, stderr.String())
	}

	// Replace original
	if err := os.Rename(tempPath, flacPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace file: %w", err)
	}

	return nil
}

// embedLyricsInMKV adds lyrics as a subtitle track in MKV
func embedLyricsInMKV(mkvPath string, lyrics *LyricsResult) error {
	// For MKV, we'll create an SRT subtitle file and mux it in
	// Synced LRC can be converted to SRT, or we use plain text as a single subtitle

	if lyrics.SyncedLyrics == "" && lyrics.PlainText == "" {
		return fmt.Errorf("no lyrics to embed")
	}

	// Create temp SRT file
	tempDir := os.TempDir()
	srtPath := filepath.Join(tempDir, "lyrics.srt")
	defer os.Remove(srtPath)

	var srtContent string
	if lyrics.SyncedLyrics != "" {
		srtContent = convertLRCtoSRT(lyrics.SyncedLyrics)
	} else {
		// Create a single subtitle entry for plain lyrics
		srtContent = fmt.Sprintf("1\n00:00:00,000 --> 99:59:59,999\n%s\n", lyrics.PlainText)
	}

	if err := os.WriteFile(srtPath, []byte(srtContent), 0644); err != nil {
		return fmt.Errorf("failed to create SRT file: %w", err)
	}

	// Mux SRT into MKV
	ffmpegPath := GetFFmpegPath()
	tempMKV := mkvPath + ".tmp"

	args := []string{
		"-y",
		"-i", mkvPath,
		"-i", srtPath,
		"-c", "copy",
		"-c:s", "srt",
		"-map", "0",
		"-map", "1",
		"-metadata:s:s:0", "language=eng",
		"-metadata:s:s:0", "title=Lyrics",
		tempMKV,
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(tempMKV)
		return fmt.Errorf("ffmpeg mux failed: %v - %s", err, stderr.String())
	}

	if err := os.Rename(tempMKV, mkvPath); err != nil {
		os.Remove(tempMKV)
		return fmt.Errorf("failed to replace file: %w", err)
	}

	return nil
}

// convertLRCtoSRT converts LRC format to SRT format
func convertLRCtoSRT(lrc string) string {
	lines := strings.Split(lrc, "\n")
	var srtLines []string
	entryNum := 1

	type lrcLine struct {
		time int // milliseconds
		text string
	}

	var parsedLines []lrcLine

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip metadata tags like [ti:], [ar:], etc.
		if strings.HasPrefix(line, "[") && !strings.HasPrefix(line, "[0") &&
			!strings.HasPrefix(line, "[1") && !strings.HasPrefix(line, "[2") &&
			!strings.HasPrefix(line, "[3") && !strings.HasPrefix(line, "[4") &&
			!strings.HasPrefix(line, "[5") && !strings.HasPrefix(line, "[6") &&
			!strings.HasPrefix(line, "[7") && !strings.HasPrefix(line, "[8") &&
			!strings.HasPrefix(line, "[9") {
			continue
		}

		// Parse timestamp [mm:ss.xx] or [mm:ss:xx]
		if len(line) < 10 || line[0] != '[' {
			continue
		}

		closeIdx := strings.Index(line, "]")
		if closeIdx < 0 {
			continue
		}

		timeStr := line[1:closeIdx]
		text := strings.TrimSpace(line[closeIdx+1:])

		if text == "" {
			continue
		}

		// Parse time
		ms := parseLRCTime(timeStr)
		if ms >= 0 {
			parsedLines = append(parsedLines, lrcLine{time: ms, text: text})
		}
	}

	// Generate SRT entries
	for i, pl := range parsedLines {
		// Calculate end time (use next line's start or add 5 seconds)
		endTime := pl.time + 5000 // Default 5 second duration
		if i+1 < len(parsedLines) {
			endTime = parsedLines[i+1].time
		}

		startStr := formatSRTTime(pl.time)
		endStr := formatSRTTime(endTime)

		srtLines = append(srtLines, fmt.Sprintf("%d", entryNum))
		srtLines = append(srtLines, fmt.Sprintf("%s --> %s", startStr, endStr))
		srtLines = append(srtLines, pl.text)
		srtLines = append(srtLines, "")

		entryNum++
	}

	return strings.Join(srtLines, "\n")
}

// parseLRCTime parses LRC timestamp to milliseconds
func parseLRCTime(timeStr string) int {
	// Formats: mm:ss.xx, mm:ss:xx, m:ss.xx
	parts := strings.Split(timeStr, ":")
	if len(parts) < 2 {
		return -1
	}

	mins := 0
	secs := 0.0

	fmt.Sscanf(parts[0], "%d", &mins)

	// Handle seconds part (may have . or : separator for centiseconds)
	secPart := parts[1]
	if len(parts) > 2 {
		// mm:ss:xx format
		secPart = parts[1] + "." + parts[2]
	}
	secPart = strings.Replace(secPart, ":", ".", 1)
	fmt.Sscanf(secPart, "%f", &secs)

	return mins*60000 + int(secs*1000)
}

// formatSRTTime formats milliseconds to SRT timestamp
func formatSRTTime(ms int) string {
	h := ms / 3600000
	m := (ms % 3600000) / 60000
	s := (ms % 60000) / 1000
	millis := ms % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, millis)
}

// FetchAndEmbedLyrics is a convenience function that fetches and embeds lyrics
func FetchAndEmbedLyrics(mediaPath, artist, title string, mode LyricsEmbedMode) error {
	lyrics, err := FetchLyrics(artist, title)
	if err != nil {
		return fmt.Errorf("failed to fetch lyrics: %w", err)
	}

	switch mode {
	case LyricsEmbedFile:
		return EmbedLyricsInFile(mediaPath, lyrics)

	case LyricsEmbedLRC:
		if lyrics.HasSync {
			_, err = SaveLRCFile(lyrics, mediaPath)
		} else {
			_, err = SavePlainLyricsFile(lyrics, mediaPath)
		}
		return err

	case LyricsEmbedBoth:
		// Save LRC file
		if lyrics.HasSync {
			if _, err := SaveLRCFile(lyrics, mediaPath); err != nil {
				// Non-fatal, continue with embedding
				slog.Warn("failed to save LRC file", "err", err)
			}
		}

		// Embed in file
		return EmbedLyricsInFile(mediaPath, lyrics)

	default:
		return fmt.Errorf("unknown embed mode: %s", mode)
	}
}

// HasLyrics checks if a media file already has embedded lyrics
func HasLyrics(mediaPath string) (bool, error) {
	ffprobePath := GetFFprobePath()

	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		mediaPath,
	}

	cmd := exec.Command(ffprobePath, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return false, err
	}

	var probeData struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &probeData); err != nil {
		return false, err
	}

	// Check common lyrics tag names
	for key := range probeData.Format.Tags {
		keyLower := strings.ToLower(key)
		if keyLower == "lyrics" || keyLower == "unsyncedlyrics" || keyLower == "syncedlyrics" {
			return true, nil
		}
	}

	return false, nil
}

// ExtractLyrics extracts embedded lyrics from a media file
func ExtractLyrics(mediaPath string) (*LyricsResult, error) {
	ffprobePath := GetFFprobePath()

	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		mediaPath,
	}

	cmd := exec.Command(ffprobePath, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var probeData struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &probeData); err != nil {
		return nil, err
	}

	result := &LyricsResult{
		Source: "embedded",
	}

	for key, value := range probeData.Format.Tags {
		keyLower := strings.ToLower(key)
		switch keyLower {
		case "lyrics", "syncedlyrics":
			if strings.Contains(value, "[") && strings.Contains(value, "]") {
				result.SyncedLyrics = value
				result.HasSync = true
			} else {
				result.PlainText = value
			}
		case "unsyncedlyrics":
			result.PlainText = value
		case "title":
			result.TrackName = value
		case "artist":
			result.ArtistName = value
		case "album":
			result.AlbumName = value
		}
	}

	if result.PlainText == "" && result.SyncedLyrics == "" {
		return nil, fmt.Errorf("no embedded lyrics found")
	}

	return result, nil
}

// ReadLRCFile reads an LRC file and returns a LyricsResult
func ReadLRCFile(lrcPath string) (*LyricsResult, error) {
	data, err := os.ReadFile(lrcPath)
	if err != nil {
		return nil, err
	}

	content := string(data)
	result := &LyricsResult{
		Source:       "lrc",
		SyncedLyrics: content,
		HasSync:      true,
	}

	// Parse metadata from LRC
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}

		if strings.HasPrefix(line, "[ti:") {
			result.TrackName = extractLRCTag(line, "ti")
		} else if strings.HasPrefix(line, "[ar:") {
			result.ArtistName = extractLRCTag(line, "ar")
		} else if strings.HasPrefix(line, "[al:") {
			result.AlbumName = extractLRCTag(line, "al")
		}
	}

	return result, nil
}

func extractLRCTag(line, tag string) string {
	prefix := fmt.Sprintf("[%s:", tag)
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	line = line[len(prefix):]
	if idx := strings.Index(line, "]"); idx >= 0 {
		return strings.TrimSpace(line[:idx])
	}
	return ""
}

// LRCLIBBatchSearch searches for multiple tracks at once
func LRCLIBBatchSearch(tracks []struct {
	Artist string
	Title  string
}) (map[string]*LyricsResult, error) {
	results := make(map[string]*LyricsResult)

	for _, track := range tracks {
		key := fmt.Sprintf("%s - %s", track.Artist, track.Title)
		result, err := FetchLyrics(track.Artist, track.Title)
		if err == nil {
			results[key] = result
		}
		// Rate limiting
		time.Sleep(200 * time.Millisecond)
	}

	return results, nil
}
