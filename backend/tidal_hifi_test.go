package backend

import (
	"testing"
)

func TestTidalHifiService_Search(t *testing.T) {
	service := NewTidalHifiService(nil)

	if !service.IsAvailable() {
		t.Skip("TidalHifi service not available")
	}

	// Search for Thunderstruck by AC/DC
	track, err := service.SearchTrack("AC/DC Thunderstruck")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	t.Logf("Found track: %s - %s (ID: %d)", track.Artist.Name, track.Title, track.ID)
	t.Logf("Album: %s, ISRC: %s, Duration: %ds", track.Album.Title, track.ISRC, track.Duration)
}

func TestTidalHifiService_GetStreamURL(t *testing.T) {
	service := NewTidalHifiService(nil)

	if !service.IsAvailable() {
		t.Skip("TidalHifi service not available")
	}

	// Thunderstruck by AC/DC - Tidal ID
	trackID := 35986245

	streamURL, err := service.GetStreamURL(trackID)
	if err != nil {
		t.Fatalf("GetStreamURL failed: %v", err)
	}

	t.Logf("Stream URL: %s", streamURL[:100]+"...")
}

func TestTidalHifiService_DownloadBySearch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping download test in short mode")
	}

	service := NewTidalHifiService(nil)

	if !service.IsAvailable() {
		t.Skip("TidalHifi service not available")
	}

	// Download Thunderstruck to temp dir
	result, err := service.DownloadBySearch("AC/DC", "Thunderstruck", t.TempDir())
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	t.Logf("Downloaded: %s (%d bytes)", result.FilePath, result.Size)
	t.Logf("Track: %s - %s", result.Track.Artist, result.Track.Title)
}
