package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateM3U8_Empty(t *testing.T) {
	dir := t.TempDir()
	err := GenerateM3U8(nil, dir, "playlist")
	if err != nil {
		t.Fatalf("expected nil error for empty items, got %v", err)
	}
	// No file should be created for empty items
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected no files created for empty items, got %d", len(entries))
	}
}

func TestGenerateM3U8_Basic(t *testing.T) {
	dir := t.TempDir()

	items := []QueueItem{
		{
			ID:         "1",
			Title:      "Never Gonna Give You Up",
			Artist:     "Rick Astley",
			Duration:   213,
			OutputPath: filepath.Join(dir, "Rick Astley - Never Gonna Give You Up.mkv"),
		},
		{
			ID:         "2",
			Title:      "Take On Me",
			Artist:     "a-ha",
			Duration:   225,
			OutputPath: filepath.Join(dir, "a-ha - Take On Me.mkv"),
		},
	}

	if err := GenerateM3U8(items, dir, "My Playlist"); err != nil {
		t.Fatalf("GenerateM3U8 failed: %v", err)
	}

	m3u8Path := filepath.Join(dir, "My Playlist.m3u8")
	data, err := os.ReadFile(m3u8Path)
	if err != nil {
		t.Fatalf("could not read m3u8 file: %v", err)
	}

	content := string(data)
	if !strings.HasPrefix(content, "#EXTM3U\n") {
		t.Error("m3u8 should start with #EXTM3U")
	}
	if !strings.Contains(content, "#EXTINF:213,Rick Astley - Never Gonna Give You Up") {
		t.Errorf("missing EXTINF for first track, got:\n%s", content)
	}
	if !strings.Contains(content, "#EXTINF:225,a-ha - Take On Me") {
		t.Errorf("missing EXTINF for second track, got:\n%s", content)
	}
	if !strings.Contains(content, "Rick Astley - Never Gonna Give You Up.mkv") {
		t.Error("missing file path for first track")
	}
}

func TestGenerateM3U8_SkipsEmptyOutputPath(t *testing.T) {
	dir := t.TempDir()

	items := []QueueItem{
		{ID: "1", Title: "Track Without Path", Artist: "Artist", Duration: 100},
		{ID: "2", Title: "Track With Path", Artist: "Artist2", Duration: 200, OutputPath: filepath.Join(dir, "track.mkv")},
	}

	if err := GenerateM3U8(items, dir, "test"); err != nil {
		t.Fatalf("GenerateM3U8 failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "test.m3u8"))
	content := string(data)

	if strings.Contains(content, "Track Without Path") {
		t.Error("should skip items without OutputPath")
	}
	if !strings.Contains(content, "Track With Path") {
		t.Error("should include items with OutputPath")
	}
}

func TestGenerateM3U8_RelativePaths(t *testing.T) {
	dir := t.TempDir()

	subdir := filepath.Join(dir, "subdir")
	items := []QueueItem{
		{ID: "1", Title: "Track", Artist: "Artist", Duration: 100, OutputPath: filepath.Join(subdir, "track.mkv")},
	}

	if err := GenerateM3U8(items, dir, "test"); err != nil {
		t.Fatalf("GenerateM3U8 failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "test.m3u8"))
	content := string(data)

	// Path should be relative ("subdir/track.mkv"), not absolute
	if strings.Contains(content, dir) {
		t.Errorf("path should be relative, got absolute in:\n%s", content)
	}
	if !strings.Contains(content, "subdir/track.mkv") {
		t.Errorf("expected relative path subdir/track.mkv in:\n%s", content)
	}
}

func TestGenerateM3U8_SanitizesPlaylistName(t *testing.T) {
	dir := t.TempDir()
	items := []QueueItem{
		{ID: "1", Title: "T", Artist: "A", Duration: 60, OutputPath: filepath.Join(dir, "t.mkv")},
	}

	// Name with slashes should be sanitized
	if err := GenerateM3U8(items, dir, "My/Playlist\\Test"); err != nil {
		t.Fatalf("GenerateM3U8 failed: %v", err)
	}

	// The sanitized name should result in a valid file
	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".m3u8") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected .m3u8 file to be created")
	}
}
