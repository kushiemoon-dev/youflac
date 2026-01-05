package backend

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Jellyfin/Plex compatible file naming and organization

// Metadata contains all information for naming and NFO generation
type Metadata struct {
	Title       string   `json:"title"`
	Artist      string   `json:"artist"`
	Album       string   `json:"album"`
	Year        int      `json:"year,omitempty"`
	ISRC        string   `json:"isrc,omitempty"`
	Duration    float64  `json:"duration,omitempty"`
	Genre       string   `json:"genre,omitempty"`
	Track       int      `json:"track,omitempty"`
	Description string   `json:"description,omitempty"`
	YouTubeID   string   `json:"youtubeId,omitempty"`
	YouTubeURL  string   `json:"youtubeUrl,omitempty"`
	Thumbnail   string   `json:"thumbnail,omitempty"`
	Directors   []string `json:"directors,omitempty"`
	Studios     []string `json:"studios,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// FolderLayout defines how files are organized
type FolderLayout string

const (
	LayoutJellyfin FolderLayout = "jellyfin" // {artist}/{title}/{title}.mkv
	LayoutPlex     FolderLayout = "plex"     // {artist}/{title}.mkv
	LayoutFlat     FolderLayout = "flat"     // {artist} - {title}.mkv
	LayoutCustom   FolderLayout = "custom"   // User-defined template
)

// NamingTemplate defines available placeholders
type NamingTemplate struct {
	Name        string `json:"name"`
	Template    string `json:"template"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// Predefined templates
var PredefinedTemplates = []NamingTemplate{
	{
		Name:        "Jellyfin",
		Template:    "{artist}/{title}/{title}",
		Description: "Artist folder → Title folder → Title.mkv (Jellyfin music videos)",
		Example:     "Rick Astley/Never Gonna Give You Up/Never Gonna Give You Up.mkv",
	},
	{
		Name:        "Plex",
		Template:    "{artist}/{title}",
		Description: "Artist folder → Title.mkv (Plex music videos)",
		Example:     "Rick Astley/Never Gonna Give You Up.mkv",
	},
	{
		Name:        "Flat",
		Template:    "{artist} - {title}",
		Description: "All files in root folder",
		Example:     "Rick Astley - Never Gonna Give You Up.mkv",
	},
	{
		Name:        "Album",
		Template:    "{artist}/{album}/{title}",
		Description: "Artist folder → Album folder → Title.mkv",
		Example:     "Rick Astley/Whenever You Need Somebody/Never Gonna Give You Up.mkv",
	},
	{
		Name:        "Year",
		Template:    "{year}/{artist} - {title}",
		Description: "Year folder → Artist - Title.mkv",
		Example:     "1987/Rick Astley - Never Gonna Give You Up.mkv",
	},
}

// Default template: Jellyfin style
const DefaultTemplate = "{artist}/{title}/{title}"

// OrganizeResult contains the result of file organization
type OrganizeResult struct {
	MKVPath      string `json:"mkvPath"`
	NFOPath      string `json:"nfoPath,omitempty"`
	PosterPath   string `json:"posterPath,omitempty"`
	Created      bool   `json:"created"`
	DirectoryCreated bool `json:"directoryCreated"`
}

// GenerateFilePath generates full file path based on template
func GenerateFilePath(metadata *Metadata, template, baseDir, extension string) string {
	if template == "" {
		template = DefaultTemplate
	}

	path := ApplyTemplate(template, metadata)
	return filepath.Join(baseDir, path+extension)
}

// ApplyTemplate replaces placeholders with actual values
func ApplyTemplate(template string, metadata *Metadata) string {
	if metadata == nil {
		return template
	}

	path := template

	// Basic replacements (only sanitize non-empty values)
	path = strings.ReplaceAll(path, "{artist}", sanitizeOrEmpty(metadata.Artist))
	path = strings.ReplaceAll(path, "{title}", sanitizeOrEmpty(metadata.Title))
	path = strings.ReplaceAll(path, "{album}", sanitizeOrEmpty(metadata.Album))

	// Year handling
	yearStr := ""
	if metadata.Year > 0 {
		yearStr = strconv.Itoa(metadata.Year)
	}
	path = strings.ReplaceAll(path, "{year}", yearStr)

	// Track number with padding
	trackStr := ""
	if metadata.Track > 0 {
		trackStr = fmt.Sprintf("%02d", metadata.Track)
	}
	path = strings.ReplaceAll(path, "{track}", trackStr)

	// Genre
	path = strings.ReplaceAll(path, "{genre}", sanitizeOrEmpty(metadata.Genre))

	// YouTube ID
	path = strings.ReplaceAll(path, "{youtube_id}", metadata.YouTubeID)

	// Clean up empty segments and multiple slashes
	path = cleanupPath(path)

	return path
}

// sanitizeOrEmpty sanitizes the filename but returns empty string for empty input
// This allows cleanupPath to remove empty segments
func sanitizeOrEmpty(name string) string {
	if name == "" {
		return ""
	}
	return SanitizeFileName(name)
}

// cleanupPath removes empty path segments and cleans up the path
func cleanupPath(path string) string {
	// Replace multiple slashes with single
	multiSlash := regexp.MustCompile(`[/\\]+`)
	path = multiSlash.ReplaceAllString(path, string(filepath.Separator))

	// Remove leading/trailing slashes
	path = strings.Trim(path, string(filepath.Separator))

	// Remove empty segments (caused by missing placeholders)
	parts := strings.Split(path, string(filepath.Separator))
	var cleanParts []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" && part != "-" {
			cleanParts = append(cleanParts, part)
		}
	}

	return strings.Join(cleanParts, string(filepath.Separator))
}

// SanitizeFileName removes invalid characters from file/folder names
func SanitizeFileName(name string) string {
	if name == "" {
		return "Unknown"
	}

	// Remove characters invalid on Windows/Linux/macOS
	// < > : " / \ | ? * and control characters
	invalid := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	sanitized := invalid.ReplaceAllString(name, "")

	// Replace multiple spaces with single space
	spaces := regexp.MustCompile(`\s+`)
	sanitized = spaces.ReplaceAllString(sanitized, " ")

	// Remove leading/trailing dots and spaces (Windows issue)
	sanitized = strings.Trim(sanitized, ". ")

	// Limit length (255 is max for most filesystems, but we use 200 for safety)
	if len(sanitized) > 200 {
		sanitized = sanitized[:200]
	}

	// Fallback for empty names
	if sanitized == "" {
		sanitized = "Unknown"
	}

	return sanitized
}

// GenerateJellyfinPath generates Jellyfin-compatible path
// Jellyfin expects: MusicVideos/Artist Name/Video Title/Video Title.mkv
func GenerateJellyfinPath(metadata *Metadata, baseDir string) string {
	return GenerateFilePath(metadata, "{artist}/{title}/{title}", baseDir, ".mkv")
}

// GeneratePlexPath generates Plex-compatible path
// Plex expects: MusicVideos/Artist Name/Video Title.mkv
func GeneratePlexPath(metadata *Metadata, baseDir string) string {
	return GenerateFilePath(metadata, "{artist}/{title}", baseDir, ".mkv")
}

// GenerateFlatPath generates flat structure path
func GenerateFlatPath(metadata *Metadata, baseDir string) string {
	return GenerateFilePath(metadata, "{artist} - {title}", baseDir, ".mkv")
}

// PlaylistTemplate is the template for playlist items with track numbers
const PlaylistTemplate = "{track} - {artist} - {title}/{track} - {artist} - {title}"

// GeneratePlaylistFilePath generates file path for playlist items with track number prefix
// Format: "01 - Artist - Title/01 - Artist - Title.mkv"
func GeneratePlaylistFilePath(metadata *Metadata, baseDir, extension string) string {
	return GenerateFilePath(metadata, PlaylistTemplate, baseDir, extension)
}

// GeneratePathForLayout generates path based on layout type
func GeneratePathForLayout(metadata *Metadata, layout FolderLayout, baseDir, customTemplate string) string {
	switch layout {
	case LayoutJellyfin:
		return GenerateJellyfinPath(metadata, baseDir)
	case LayoutPlex:
		return GeneratePlexPath(metadata, baseDir)
	case LayoutFlat:
		return GenerateFlatPath(metadata, baseDir)
	case LayoutCustom:
		return GenerateFilePath(metadata, customTemplate, baseDir, ".mkv")
	default:
		return GenerateJellyfinPath(metadata, baseDir)
	}
}

// PreviewNaming generates a preview of how files will be named
func PreviewNaming(metadata *Metadata, template string) string {
	return ApplyTemplate(template, metadata) + ".mkv"
}

// GetAvailableTemplates returns all predefined templates
func GetAvailableTemplates() []NamingTemplate {
	return PredefinedTemplates
}

// ValidateTemplate checks if a template is valid
func ValidateTemplate(template string) error {
	if template == "" {
		return fmt.Errorf("template cannot be empty")
	}

	// Check for at least one placeholder
	placeholders := []string{"{artist}", "{title}", "{album}", "{year}", "{track}", "{genre}", "{youtube_id}"}
	hasPlaceholder := false
	for _, p := range placeholders {
		if strings.Contains(template, p) {
			hasPlaceholder = true
			break
		}
	}

	if !hasPlaceholder {
		return fmt.Errorf("template must contain at least one placeholder: %v", placeholders)
	}

	// Check for invalid characters
	invalid := regexp.MustCompile(`[<>:"|?*]`)
	if invalid.MatchString(template) {
		return fmt.Errorf("template contains invalid characters")
	}

	return nil
}

// GenerateNFOPath returns the path for the NFO file
func GenerateNFOPath(mkvPath string) string {
	return strings.TrimSuffix(mkvPath, filepath.Ext(mkvPath)) + ".nfo"
}

// GeneratePosterPath returns the path for the poster image
func GeneratePosterPath(mkvPath string) string {
	dir := filepath.Dir(mkvPath)
	return filepath.Join(dir, "poster.jpg")
}

// GenerateFanartPath returns the path for fanart image
func GenerateFanartPath(mkvPath string) string {
	dir := filepath.Dir(mkvPath)
	return filepath.Join(dir, "fanart.jpg")
}

// CreateDirectoryStructure creates necessary directories for the output path
func CreateDirectoryStructure(outputPath string) error {
	dir := filepath.Dir(outputPath)
	return os.MkdirAll(dir, 0755)
}

// OrganizeOutput creates directory structure and returns paths for all output files
func OrganizeOutput(metadata *Metadata, layout FolderLayout, baseDir, customTemplate string) (*OrganizeResult, error) {
	mkvPath := GeneratePathForLayout(metadata, layout, baseDir, customTemplate)

	result := &OrganizeResult{
		MKVPath:    mkvPath,
		NFOPath:    GenerateNFOPath(mkvPath),
		PosterPath: GeneratePosterPath(mkvPath),
	}

	// Create directory structure
	dir := filepath.Dir(mkvPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		result.DirectoryCreated = true
	}

	result.Created = true
	return result, nil
}

// ===============================
// NFO Generation (Kodi/Jellyfin format)
// ===============================

// MusicVideoNFO represents the XML structure for music video NFO
type MusicVideoNFO struct {
	XMLName       xml.Name       `xml:"musicvideo"`
	Title         string         `xml:"title"`
	Artist        string         `xml:"artist"`
	Album         string         `xml:"album,omitempty"`
	Year          int            `xml:"year,omitempty"`
	Runtime       int            `xml:"runtime,omitempty"` // in minutes
	Plot          string         `xml:"plot,omitempty"`
	Genre         string         `xml:"genre,omitempty"`
	Directors     []string       `xml:"director,omitempty"`
	Studios       []string       `xml:"studio,omitempty"`
	Tags          []string       `xml:"tag,omitempty"`
	UniqueID      []UniqueID     `xml:"uniqueid,omitempty"`
	Thumb         []NFOThumb     `xml:"thumb,omitempty"`
	Fanart        *NFOFanart     `xml:"fanart,omitempty"`
	DateAdded     string         `xml:"dateadded,omitempty"`
	FileInfo      *NFOFileInfo   `xml:"fileinfo,omitempty"`
}

// UniqueID represents external IDs (YouTube, ISRC, etc.)
type UniqueID struct {
	Type    string `xml:"type,attr"`
	Default bool   `xml:"default,attr,omitempty"`
	Value   string `xml:",chardata"`
}

// NFOThumb represents thumbnail/poster entry
type NFOThumb struct {
	Aspect  string `xml:"aspect,attr,omitempty"`
	Preview string `xml:"preview,attr,omitempty"`
	URL     string `xml:",chardata"`
}

// NFOFanart represents fanart container
type NFOFanart struct {
	Thumbs []NFOThumb `xml:"thumb,omitempty"`
}

// NFOFileInfo contains technical file information
type NFOFileInfo struct {
	StreamDetails *StreamDetails `xml:"streamdetails,omitempty"`
}

// StreamDetails contains video/audio stream info
type StreamDetails struct {
	Video *VideoStreamInfo `xml:"video,omitempty"`
	Audio *AudioStreamInfo `xml:"audio,omitempty"`
}

// VideoStreamInfo contains video stream details
type VideoStreamInfo struct {
	Codec             string `xml:"codec,omitempty"`
	Aspect            string `xml:"aspect,omitempty"`
	Width             int    `xml:"width,omitempty"`
	Height            int    `xml:"height,omitempty"`
	DurationInSeconds int    `xml:"durationinseconds,omitempty"`
}

// AudioStreamInfo contains audio stream details
type AudioStreamInfo struct {
	Codec    string `xml:"codec,omitempty"`
	Channels int    `xml:"channels,omitempty"`
}

// NFOOptions configures NFO generation
type NFOOptions struct {
	IncludeFileInfo  bool        `json:"includeFileInfo"`
	IncludeThumbnail bool        `json:"includeThumbnail"`
	MediaInfo        *MediaInfo  `json:"mediaInfo,omitempty"`
}

// GenerateNFO creates NFO XML content for a music video
func GenerateNFO(metadata *Metadata, opts *NFOOptions) ([]byte, error) {
	if metadata == nil {
		return nil, fmt.Errorf("metadata is required")
	}

	nfo := MusicVideoNFO{
		Title:     metadata.Title,
		Artist:    metadata.Artist,
		Album:     metadata.Album,
		Year:      metadata.Year,
		Plot:      metadata.Description,
		Genre:     metadata.Genre,
		Directors: metadata.Directors,
		Studios:   metadata.Studios,
		Tags:      metadata.Tags,
		DateAdded: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Runtime in minutes
	if metadata.Duration > 0 {
		nfo.Runtime = int(metadata.Duration / 60)
	}

	// Add unique IDs
	if metadata.YouTubeID != "" {
		nfo.UniqueID = append(nfo.UniqueID, UniqueID{
			Type:    "youtube",
			Default: true,
			Value:   metadata.YouTubeID,
		})
	}
	if metadata.ISRC != "" {
		nfo.UniqueID = append(nfo.UniqueID, UniqueID{
			Type:  "isrc",
			Value: metadata.ISRC,
		})
	}

	// Add thumbnail
	if opts != nil && opts.IncludeThumbnail && metadata.Thumbnail != "" {
		nfo.Thumb = append(nfo.Thumb, NFOThumb{
			Aspect: "poster",
			URL:    metadata.Thumbnail,
		})
		nfo.Fanart = &NFOFanart{
			Thumbs: []NFOThumb{{URL: metadata.Thumbnail}},
		}
	}

	// Add file info from MediaInfo
	if opts != nil && opts.IncludeFileInfo && opts.MediaInfo != nil {
		mi := opts.MediaInfo
		nfo.FileInfo = &NFOFileInfo{
			StreamDetails: &StreamDetails{
				Video: &VideoStreamInfo{
					Codec:             mi.VideoCodec,
					Width:             mi.Width,
					Height:            mi.Height,
					DurationInSeconds: int(mi.Duration),
				},
				Audio: &AudioStreamInfo{
					Codec:    mi.AudioCodec,
					Channels: mi.Channels,
				},
			},
		}
		if mi.Width > 0 && mi.Height > 0 {
			nfo.FileInfo.StreamDetails.Video.Aspect = fmt.Sprintf("%.2f", float64(mi.Width)/float64(mi.Height))
		}
	}

	// Marshal with proper XML header and indentation
	output, err := xml.MarshalIndent(nfo, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to generate NFO: %w", err)
	}

	// Add XML header
	xmlHeader := []byte(xml.Header)
	return append(xmlHeader, output...), nil
}

// WriteNFO generates and writes NFO file to disk
func WriteNFO(metadata *Metadata, nfoPath string, opts *NFOOptions) error {
	content, err := GenerateNFO(metadata, opts)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(nfoPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(nfoPath, content, 0644)
}

// DownloadPoster downloads thumbnail and saves as poster.jpg
func DownloadPoster(thumbnailURL, posterPath string) error {
	if thumbnailURL == "" {
		return fmt.Errorf("thumbnail URL is empty")
	}

	// Ensure directory exists
	dir := filepath.Dir(posterPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Download using ffmpeg (handles various protocols)
	ffmpegPath := GetFFmpegPath()
	if ffmpegPath == "" {
		return fmt.Errorf("ffmpeg not found, cannot download thumbnail")
	}

	// Use ffmpeg to download and convert to jpg
	args := []string{
		"-y",
		"-i", thumbnailURL,
		"-vframes", "1",
		"-q:v", "2",
		posterPath,
	}

	cmd := exec.Command(ffmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to download poster: %w, output: %s", err, string(output))
	}

	return nil
}

// ===============================
// Batch Operations
// ===============================

// RenameOptions configures batch rename operations
type RenameOptions struct {
	Template     string       `json:"template"`
	Layout       FolderLayout `json:"layout"`
	DryRun       bool         `json:"dryRun"`
	CreateNFO    bool         `json:"createNfo"`
	DownloadArt  bool         `json:"downloadArt"`
}

// RenameResult contains the result of a rename operation
type RenameResult struct {
	OldPath   string `json:"oldPath"`
	NewPath   string `json:"newPath"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	NFOPath   string `json:"nfoPath,omitempty"`
	DryRun    bool   `json:"dryRun"`
}

// RenameMKV renames an MKV file according to template
func RenameMKV(mkvPath string, metadata *Metadata, baseDir string, opts RenameOptions) (*RenameResult, error) {
	result := &RenameResult{
		OldPath: mkvPath,
		DryRun:  opts.DryRun,
	}

	// Generate new path
	var newPath string
	if opts.Template != "" {
		newPath = GenerateFilePath(metadata, opts.Template, baseDir, ".mkv")
	} else {
		newPath = GeneratePathForLayout(metadata, opts.Layout, baseDir, "")
	}
	result.NewPath = newPath

	// Check if paths are the same
	if mkvPath == newPath {
		result.Success = true
		return result, nil
	}

	// Dry run - just return what would happen
	if opts.DryRun {
		result.Success = true
		return result, nil
	}

	// Create destination directory
	if err := CreateDirectoryStructure(newPath); err != nil {
		result.Error = err.Error()
		return result, err
	}

	// Move file
	if err := os.Rename(mkvPath, newPath); err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.Success = true

	// Create NFO if requested
	if opts.CreateNFO {
		nfoPath := GenerateNFOPath(newPath)
		if err := WriteNFO(metadata, nfoPath, nil); err == nil {
			result.NFOPath = nfoPath
		}
	}

	return result, nil
}

// CheckFileConflict checks if a file already exists at the target path
func CheckFileConflict(outputPath string) (bool, error) {
	_, err := os.Stat(outputPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ResolveConflict returns a unique path by adding a suffix
func ResolveConflict(outputPath string) string {
	ext := filepath.Ext(outputPath)
	base := strings.TrimSuffix(outputPath, ext)

	for i := 1; i <= 100; i++ {
		newPath := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}

	// Fallback with timestamp
	return fmt.Sprintf("%s_%d%s", base, time.Now().Unix(), ext)
}
