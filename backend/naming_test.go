package backend

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal Name", "Normal Name"},
		{"With: Colon", "With Colon"},
		{"With/Slash", "WithSlash"},
		{"With\\Backslash", "WithBackslash"},
		{"With<Brackets>", "WithBrackets"},
		{"With|Pipe", "WithPipe"},
		{"With?Question", "WithQuestion"},
		{"With*Star", "WithStar"},
		{"With\"Quotes\"", "WithQuotes"},
		{"  Extra   Spaces  ", "Extra Spaces"},
		{"", "Unknown"},
		{"...dots...", "dots"},
		{"   ", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := SanitizeFileName(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeFileName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestApplyTemplate(t *testing.T) {
	metadata := &Metadata{
		Title:     "Never Gonna Give You Up",
		Artist:    "Rick Astley",
		Album:     "Whenever You Need Somebody",
		Year:      1987,
		Track:     1,
		Genre:     "Pop",
		YouTubeID: "dQw4w9WgXcQ",
	}

	tests := []struct {
		template string
		expected string
	}{
		{"{artist}/{title}/{title}", "Rick Astley/Never Gonna Give You Up/Never Gonna Give You Up"},
		{"{artist}/{title}", "Rick Astley/Never Gonna Give You Up"},
		{"{artist} - {title}", "Rick Astley - Never Gonna Give You Up"},
		{"{artist}/{album}/{title}", "Rick Astley/Whenever You Need Somebody/Never Gonna Give You Up"},
		{"{year}/{artist} - {title}", "1987/Rick Astley - Never Gonna Give You Up"},
		{"{track} - {title}", "01 - Never Gonna Give You Up"},
		{"{genre}/{artist}/{title}", "Pop/Rick Astley/Never Gonna Give You Up"},
		{"{youtube_id}", "dQw4w9WgXcQ"},
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			result := ApplyTemplate(tt.template, metadata)
			if result != tt.expected {
				t.Errorf("ApplyTemplate(%q) = %q, want %q", tt.template, result, tt.expected)
			}
		})
	}
}

func TestApplyTemplate_MissingFields(t *testing.T) {
	metadata := &Metadata{
		Title:  "Song Title",
		Artist: "Artist Name",
		// Album, Year, Track are empty/zero
	}

	tests := []struct {
		template string
		expected string
	}{
		// Missing year should be cleaned up (empty segment removed)
		{"{year}/{artist}/{title}", "Artist Name/Song Title"},
		// Missing album should be cleaned up
		{"{artist}/{album}/{title}", "Artist Name/Song Title"},
		// Missing track keeps the separator (not in a path segment)
		{"{track} - {title}", "- Song Title"},
		// Path with missing track is cleaned up
		{"{track}/{title}", "Song Title"},
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			result := ApplyTemplate(tt.template, metadata)
			if result != tt.expected {
				t.Errorf("ApplyTemplate(%q) = %q, want %q", tt.template, result, tt.expected)
			}
		})
	}
}

func TestGenerateFilePath(t *testing.T) {
	metadata := &Metadata{
		Title:  "Never Gonna Give You Up",
		Artist: "Rick Astley",
	}
	baseDir := "/music/videos"

	tests := []struct {
		template string
		expected string
	}{
		{"{artist}/{title}/{title}", "/music/videos/Rick Astley/Never Gonna Give You Up/Never Gonna Give You Up.mkv"},
		{"{artist}/{title}", "/music/videos/Rick Astley/Never Gonna Give You Up.mkv"},
		{"{artist} - {title}", "/music/videos/Rick Astley - Never Gonna Give You Up.mkv"},
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			result := GenerateFilePath(metadata, tt.template, baseDir, ".mkv")
			if result != tt.expected {
				t.Errorf("GenerateFilePath(%q) = %q, want %q", tt.template, result, tt.expected)
			}
		})
	}
}

func TestGenerateJellyfinPath(t *testing.T) {
	metadata := &Metadata{
		Title:  "Never Gonna Give You Up",
		Artist: "Rick Astley",
	}
	baseDir := "/srv/jellyfin/MusicVideos"

	expected := "/srv/jellyfin/MusicVideos/Rick Astley/Never Gonna Give You Up/Never Gonna Give You Up.mkv"
	result := GenerateJellyfinPath(metadata, baseDir)

	if result != expected {
		t.Errorf("GenerateJellyfinPath() = %q, want %q", result, expected)
	}
}

func TestGeneratePlexPath(t *testing.T) {
	metadata := &Metadata{
		Title:  "Never Gonna Give You Up",
		Artist: "Rick Astley",
	}
	baseDir := "/srv/plex/MusicVideos"

	expected := "/srv/plex/MusicVideos/Rick Astley/Never Gonna Give You Up.mkv"
	result := GeneratePlexPath(metadata, baseDir)

	if result != expected {
		t.Errorf("GeneratePlexPath() = %q, want %q", result, expected)
	}
}

func TestGenerateFlatPath(t *testing.T) {
	metadata := &Metadata{
		Title:  "Never Gonna Give You Up",
		Artist: "Rick Astley",
	}
	baseDir := "/music/videos"

	expected := "/music/videos/Rick Astley - Never Gonna Give You Up.mkv"
	result := GenerateFlatPath(metadata, baseDir)

	if result != expected {
		t.Errorf("GenerateFlatPath() = %q, want %q", result, expected)
	}
}

func TestGeneratePathForLayout(t *testing.T) {
	metadata := &Metadata{
		Title:  "Test Song",
		Artist: "Test Artist",
	}
	baseDir := "/music"

	tests := []struct {
		layout   FolderLayout
		expected string
	}{
		{LayoutJellyfin, "/music/Test Artist/Test Song/Test Song.mkv"},
		{LayoutPlex, "/music/Test Artist/Test Song.mkv"},
		{LayoutFlat, "/music/Test Artist - Test Song.mkv"},
	}

	for _, tt := range tests {
		t.Run(string(tt.layout), func(t *testing.T) {
			result := GeneratePathForLayout(metadata, tt.layout, baseDir, "")
			if result != tt.expected {
				t.Errorf("GeneratePathForLayout(%s) = %q, want %q", tt.layout, result, tt.expected)
			}
		})
	}
}

func TestGeneratePathForLayout_Custom(t *testing.T) {
	metadata := &Metadata{
		Title:  "Test Song",
		Artist: "Test Artist",
		Year:   2023,
	}
	baseDir := "/music"
	customTemplate := "{year}/{artist}/{title}"

	result := GeneratePathForLayout(metadata, LayoutCustom, baseDir, customTemplate)
	expected := "/music/2023/Test Artist/Test Song.mkv"

	if result != expected {
		t.Errorf("GeneratePathForLayout(custom) = %q, want %q", result, expected)
	}
}

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		template  string
		expectErr bool
	}{
		{"{artist}/{title}", false},
		{"{title}", false},
		{"{year}/{artist}", false},
		{"", true},                // Empty template
		{"no placeholders", true}, // No placeholders
		{"{artist}:{title}", true}, // Invalid character
		{"{artist}|{title}", true}, // Invalid character
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			err := ValidateTemplate(tt.template)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateTemplate(%q) error = %v, expectErr = %v", tt.template, err, tt.expectErr)
			}
		})
	}
}

func TestPreviewNaming(t *testing.T) {
	metadata := &Metadata{
		Title:  "Test Song",
		Artist: "Test Artist",
	}

	result := PreviewNaming(metadata, "{artist} - {title}")
	expected := "Test Artist - Test Song.mkv"

	if result != expected {
		t.Errorf("PreviewNaming() = %q, want %q", result, expected)
	}
}

func TestGetAvailableTemplates(t *testing.T) {
	templates := GetAvailableTemplates()

	if len(templates) < 3 {
		t.Errorf("GetAvailableTemplates() returned %d templates, expected at least 3", len(templates))
	}

	// Verify required fields are present
	for _, tmpl := range templates {
		if tmpl.Name == "" {
			t.Error("Template has empty name")
		}
		if tmpl.Template == "" {
			t.Error("Template has empty template string")
		}
		if tmpl.Description == "" {
			t.Error("Template has empty description")
		}
		if tmpl.Example == "" {
			t.Error("Template has empty example")
		}
	}
}

func TestGenerateNFOPath(t *testing.T) {
	tests := []struct {
		mkvPath  string
		expected string
	}{
		{"/path/to/video.mkv", "/path/to/video.nfo"},
		{"/path/to/Artist - Song.mkv", "/path/to/Artist - Song.nfo"},
	}

	for _, tt := range tests {
		t.Run(tt.mkvPath, func(t *testing.T) {
			result := GenerateNFOPath(tt.mkvPath)
			if result != tt.expected {
				t.Errorf("GenerateNFOPath(%q) = %q, want %q", tt.mkvPath, result, tt.expected)
			}
		})
	}
}

func TestGeneratePosterPath(t *testing.T) {
	mkvPath := "/path/to/Artist/Song/Song.mkv"
	expected := "/path/to/Artist/Song/poster.jpg"

	result := GeneratePosterPath(mkvPath)
	if result != expected {
		t.Errorf("GeneratePosterPath() = %q, want %q", result, expected)
	}
}

func TestGenerateFanartPath(t *testing.T) {
	mkvPath := "/path/to/Artist/Song/Song.mkv"
	expected := "/path/to/Artist/Song/fanart.jpg"

	result := GenerateFanartPath(mkvPath)
	if result != expected {
		t.Errorf("GenerateFanartPath() = %q, want %q", result, expected)
	}
}

// ===============================
// NFO Generation Tests
// ===============================

func TestGenerateNFO_Basic(t *testing.T) {
	metadata := &Metadata{
		Title:     "Never Gonna Give You Up",
		Artist:    "Rick Astley",
		Album:     "Whenever You Need Somebody",
		Year:      1987,
		Duration:  213.0,
		YouTubeID: "dQw4w9WgXcQ",
		ISRC:      "GBARL9300135",
	}

	content, err := GenerateNFO(metadata, nil)
	if err != nil {
		t.Fatalf("GenerateNFO failed: %v", err)
	}

	// Verify XML structure
	nfoStr := string(content)

	// Should have XML header
	if !strings.HasPrefix(nfoStr, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Error("NFO should start with XML header")
	}

	// Should contain expected elements
	if !strings.Contains(nfoStr, "<musicvideo>") {
		t.Error("NFO should contain <musicvideo> root element")
	}
	if !strings.Contains(nfoStr, "<title>Never Gonna Give You Up</title>") {
		t.Error("NFO should contain title")
	}
	if !strings.Contains(nfoStr, "<artist>Rick Astley</artist>") {
		t.Error("NFO should contain artist")
	}
	if !strings.Contains(nfoStr, "<album>Whenever You Need Somebody</album>") {
		t.Error("NFO should contain album")
	}
	if !strings.Contains(nfoStr, "<year>1987</year>") {
		t.Error("NFO should contain year")
	}
	if !strings.Contains(nfoStr, "<runtime>3</runtime>") {
		t.Error("NFO should contain runtime in minutes")
	}

	// Parse back to verify structure
	var nfo MusicVideoNFO
	err = xml.Unmarshal(content, &nfo)
	if err != nil {
		t.Fatalf("Failed to parse generated NFO: %v", err)
	}

	if nfo.Title != metadata.Title {
		t.Errorf("Parsed title = %q, want %q", nfo.Title, metadata.Title)
	}
}

func TestGenerateNFO_WithUniqueIDs(t *testing.T) {
	metadata := &Metadata{
		Title:     "Test Song",
		Artist:    "Test Artist",
		YouTubeID: "abc123xyz",
		ISRC:      "USRC11700001",
	}

	content, err := GenerateNFO(metadata, nil)
	if err != nil {
		t.Fatalf("GenerateNFO failed: %v", err)
	}

	var nfo MusicVideoNFO
	err = xml.Unmarshal(content, &nfo)
	if err != nil {
		t.Fatalf("Failed to parse NFO: %v", err)
	}

	// Should have 2 unique IDs
	if len(nfo.UniqueID) != 2 {
		t.Errorf("Expected 2 unique IDs, got %d", len(nfo.UniqueID))
	}

	// Find YouTube ID
	foundYT := false
	foundISRC := false
	for _, uid := range nfo.UniqueID {
		if uid.Type == "youtube" && uid.Value == "abc123xyz" {
			foundYT = true
		}
		if uid.Type == "isrc" && uid.Value == "USRC11700001" {
			foundISRC = true
		}
	}

	if !foundYT {
		t.Error("YouTube ID not found in unique IDs")
	}
	if !foundISRC {
		t.Error("ISRC not found in unique IDs")
	}
}

func TestGenerateNFO_WithThumbnail(t *testing.T) {
	metadata := &Metadata{
		Title:     "Test Song",
		Artist:    "Test Artist",
		Thumbnail: "https://i.ytimg.com/vi/abc123/maxresdefault.jpg",
	}

	opts := &NFOOptions{
		IncludeThumbnail: true,
	}

	content, err := GenerateNFO(metadata, opts)
	if err != nil {
		t.Fatalf("GenerateNFO failed: %v", err)
	}

	var nfo MusicVideoNFO
	err = xml.Unmarshal(content, &nfo)
	if err != nil {
		t.Fatalf("Failed to parse NFO: %v", err)
	}

	// Should have thumb
	if len(nfo.Thumb) == 0 {
		t.Error("Expected thumb element")
	}

	// Should have fanart
	if nfo.Fanart == nil || len(nfo.Fanart.Thumbs) == 0 {
		t.Error("Expected fanart element")
	}
}

func TestGenerateNFO_WithMediaInfo(t *testing.T) {
	metadata := &Metadata{
		Title:  "Test Song",
		Artist: "Test Artist",
	}

	opts := &NFOOptions{
		IncludeFileInfo: true,
		MediaInfo: &MediaInfo{
			Duration:   213.0,
			VideoCodec: "h264",
			AudioCodec: "flac",
			Width:      1920,
			Height:     1080,
			Channels:   2,
		},
	}

	content, err := GenerateNFO(metadata, opts)
	if err != nil {
		t.Fatalf("GenerateNFO failed: %v", err)
	}

	var nfo MusicVideoNFO
	err = xml.Unmarshal(content, &nfo)
	if err != nil {
		t.Fatalf("Failed to parse NFO: %v", err)
	}

	// Should have file info
	if nfo.FileInfo == nil {
		t.Fatal("Expected fileinfo element")
	}
	if nfo.FileInfo.StreamDetails == nil {
		t.Fatal("Expected streamdetails element")
	}
	if nfo.FileInfo.StreamDetails.Video == nil {
		t.Fatal("Expected video stream details")
	}
	if nfo.FileInfo.StreamDetails.Audio == nil {
		t.Fatal("Expected audio stream details")
	}

	video := nfo.FileInfo.StreamDetails.Video
	if video.Codec != "h264" {
		t.Errorf("Video codec = %q, want h264", video.Codec)
	}
	if video.Width != 1920 {
		t.Errorf("Video width = %d, want 1920", video.Width)
	}
	if video.Height != 1080 {
		t.Errorf("Video height = %d, want 1080", video.Height)
	}

	audio := nfo.FileInfo.StreamDetails.Audio
	if audio.Codec != "flac" {
		t.Errorf("Audio codec = %q, want flac", audio.Codec)
	}
	if audio.Channels != 2 {
		t.Errorf("Audio channels = %d, want 2", audio.Channels)
	}
}

func TestGenerateNFO_Error_NilMetadata(t *testing.T) {
	_, err := GenerateNFO(nil, nil)
	if err == nil {
		t.Error("Expected error for nil metadata")
	}
}

// ===============================
// File Operation Tests
// ===============================

func TestCreateDirectoryStructure(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "Artist", "Album", "Song.mkv")

	err := CreateDirectoryStructure(outputPath)
	if err != nil {
		t.Fatalf("CreateDirectoryStructure failed: %v", err)
	}

	// Check directory was created
	dir := filepath.Dir(outputPath)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Expected directory, got file")
	}
}

func TestOrganizeOutput(t *testing.T) {
	tmpDir := t.TempDir()
	metadata := &Metadata{
		Title:  "Test Song",
		Artist: "Test Artist",
	}

	result, err := OrganizeOutput(metadata, LayoutJellyfin, tmpDir, "")
	if err != nil {
		t.Fatalf("OrganizeOutput failed: %v", err)
	}

	if !result.Created {
		t.Error("Expected Created = true")
	}
	if !result.DirectoryCreated {
		t.Error("Expected DirectoryCreated = true")
	}
	if result.MKVPath == "" {
		t.Error("MKVPath should not be empty")
	}
	if result.NFOPath == "" {
		t.Error("NFOPath should not be empty")
	}
	if result.PosterPath == "" {
		t.Error("PosterPath should not be empty")
	}

	// Verify directory exists
	dir := filepath.Dir(result.MKVPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}
}

func TestWriteNFO(t *testing.T) {
	tmpDir := t.TempDir()
	metadata := &Metadata{
		Title:     "Test Song",
		Artist:    "Test Artist",
		Album:     "Test Album",
		Year:      2023,
		YouTubeID: "abc123",
	}

	nfoPath := filepath.Join(tmpDir, "Artist", "Song", "Song.nfo")

	err := WriteNFO(metadata, nfoPath, nil)
	if err != nil {
		t.Fatalf("WriteNFO failed: %v", err)
	}

	// Read and verify content
	content, err := os.ReadFile(nfoPath)
	if err != nil {
		t.Fatalf("Failed to read NFO file: %v", err)
	}

	if !strings.Contains(string(content), "<title>Test Song</title>") {
		t.Error("NFO file doesn't contain expected content")
	}
}

func TestCheckFileConflict(t *testing.T) {
	tmpDir := t.TempDir()

	// Test non-existent file
	nonExistent := filepath.Join(tmpDir, "nonexistent.mkv")
	conflict, err := CheckFileConflict(nonExistent)
	if err != nil {
		t.Fatalf("CheckFileConflict failed: %v", err)
	}
	if conflict {
		t.Error("Expected no conflict for non-existent file")
	}

	// Create a file
	existingFile := filepath.Join(tmpDir, "existing.mkv")
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test existing file
	conflict, err = CheckFileConflict(existingFile)
	if err != nil {
		t.Fatalf("CheckFileConflict failed: %v", err)
	}
	if !conflict {
		t.Error("Expected conflict for existing file")
	}
}

func TestResolveConflict(t *testing.T) {
	tmpDir := t.TempDir()

	// Create conflicting files
	basePath := filepath.Join(tmpDir, "Song.mkv")
	os.WriteFile(basePath, []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "Song (1).mkv"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "Song (2).mkv"), []byte("test"), 0644)

	result := ResolveConflict(basePath)
	expected := filepath.Join(tmpDir, "Song (3).mkv")

	if result != expected {
		t.Errorf("ResolveConflict() = %q, want %q", result, expected)
	}

	// Verify the resolved path doesn't exist
	if _, err := os.Stat(result); !os.IsNotExist(err) {
		t.Error("Resolved path should not exist")
	}
}

// ===============================
// Edge Cases
// ===============================

func TestSanitizeFileName_LongName(t *testing.T) {
	// Create a very long filename
	longName := strings.Repeat("a", 300)
	result := SanitizeFileName(longName)

	if len(result) > 200 {
		t.Errorf("SanitizeFileName should limit length to 200, got %d", len(result))
	}
}

func TestApplyTemplate_SpecialCharacters(t *testing.T) {
	metadata := &Metadata{
		Title:  "Song: The \"Best\" Version",
		Artist: "Artist/Name",
	}

	result := ApplyTemplate("{artist} - {title}", metadata)

	// Special characters should be sanitized
	if strings.Contains(result, "/") {
		t.Error("Result should not contain slashes")
	}
	if strings.Contains(result, ":") {
		t.Error("Result should not contain colons")
	}
	if strings.Contains(result, "\"") {
		t.Error("Result should not contain quotes")
	}
}

func TestGenerateFilePath_EmptyMetadata(t *testing.T) {
	metadata := &Metadata{}
	baseDir := "/music"
	template := "{artist}/{title}"

	result := GenerateFilePath(metadata, template, baseDir, ".mkv")

	// Empty fields are removed, leaving just the extension
	// The path cleanup removes empty segments
	expected := "/music/.mkv"
	if result != expected {
		t.Errorf("GenerateFilePath with empty metadata = %q, want %q", result, expected)
	}
}

func TestGenerateFilePath_PartialMetadata(t *testing.T) {
	metadata := &Metadata{
		Title: "Song Title",
		// Artist is empty
	}
	baseDir := "/music"
	template := "{artist}/{title}"

	result := GenerateFilePath(metadata, template, baseDir, ".mkv")

	// Empty artist is cleaned up, only title remains
	expected := "/music/Song Title.mkv"
	if result != expected {
		t.Errorf("GenerateFilePath with partial metadata = %q, want %q", result, expected)
	}
}
