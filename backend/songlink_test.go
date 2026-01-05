package backend

import (
	"fmt"
	"testing"
)

func TestResolveMusicURL(t *testing.T) {
	// Test avec "Never Gonna Give You Up" de Spotify
	spotifyURL := "https://open.spotify.com/track/4PTG3Z6ehGkBFwjybzWkR8"

	fmt.Println("=== Test song.link API ===")
	fmt.Println("URL Spotify:", spotifyURL)
	fmt.Println()

	// Résolution cross-platform
	info, err := ResolveMusicURL(spotifyURL)
	if err != nil {
		t.Fatalf("Erreur: %v", err)
	}

	fmt.Println("✓ Métadonnées:")
	fmt.Printf("  Titre: %s\n", info.Title)
	fmt.Printf("  Artiste: %s\n", info.Artist)
	fmt.Printf("  Type: %s\n", info.Type)
	fmt.Println()

	if info.Title == "" {
		t.Error("Titre vide")
	}
	if info.Artist == "" {
		t.Error("Artiste vide")
	}

	fmt.Println("✓ URLs cross-platform:")
	if info.URLs.SpotifyURL != "" {
		fmt.Printf("  Spotify: %s\n", info.URLs.SpotifyURL)
	}
	if info.URLs.TidalURL != "" {
		fmt.Printf("  Tidal: %s\n", info.URLs.TidalURL)
	}
	if info.URLs.QobuzURL != "" {
		fmt.Printf("  Qobuz: %s\n", info.URLs.QobuzURL)
	}
	if info.URLs.AmazonURL != "" {
		fmt.Printf("  Amazon: %s\n", info.URLs.AmazonURL)
	}
	if info.URLs.DeezerURL != "" {
		fmt.Printf("  Deezer: %s\n", info.URLs.DeezerURL)
	}
	if info.URLs.YouTubeURL != "" {
		fmt.Printf("  YouTube: %s\n", info.URLs.YouTubeURL)
	}
	if info.URLs.YouTubeMusicURL != "" {
		fmt.Printf("  YouTube Music: %s\n", info.URLs.YouTubeMusicURL)
	}
	fmt.Println()

	// Test meilleure source FLAC
	platform, url := GetBestFLACSource(info)
	if platform != "" {
		fmt.Printf("✓ Meilleure source FLAC: %s\n", platform)
		fmt.Printf("  URL: %s\n", url)
	} else {
		fmt.Println("⚠ Aucune source FLAC disponible")
	}

	// Vérifier qu'au moins une plateforme est disponible
	sources := GetAllFLACSources(info)
	fmt.Printf("\n✓ Sources FLAC disponibles: %d\n", len(sources))
	for _, s := range sources {
		fmt.Printf("  [%d] %s: %s\n", s.Priority, s.Platform, s.URL)
	}
}

func TestParseSpotifyURL(t *testing.T) {
	tests := []struct {
		url         string
		expectedID  string
		expectedType string
		shouldError bool
	}{
		{
			url:         "https://open.spotify.com/track/4PTG3Z6ehGkBFwjybzWkR8",
			expectedID:  "4PTG3Z6ehGkBFwjybzWkR8",
			expectedType: "track",
		},
		{
			url:         "https://open.spotify.com/intl-fr/track/4PTG3Z6ehGkBFwjybzWkR8",
			expectedID:  "4PTG3Z6ehGkBFwjybzWkR8",
			expectedType: "track",
		},
		{
			url:         "https://open.spotify.com/album/6DEjYFkNZh67HP7BOsfBOG",
			expectedID:  "6DEjYFkNZh67HP7BOsfBOG",
			expectedType: "album",
		},
		{
			url:         "https://example.com/something",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			id, contentType, err := ParseSpotifyURL(tt.url)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for URL: %s", tt.url)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if id != tt.expectedID {
				t.Errorf("Expected ID %s, got %s", tt.expectedID, id)
			}
			if contentType != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, contentType)
			}
		})
	}
}

func TestIsSpotifyURL(t *testing.T) {
	validURLs := []string{
		"https://open.spotify.com/track/4PTG3Z6ehGkBFwjybzWkR8",
		"https://open.spotify.com/intl-fr/track/4PTG3Z6ehGkBFwjybzWkR8",
		"https://open.spotify.com/album/6DEjYFkNZh67HP7BOsfBOG",
		"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M",
	}

	invalidURLs := []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://tidal.com/track/123456",
		"https://example.com/track/abc",
	}

	for _, url := range validURLs {
		if !IsSpotifyURL(url) {
			t.Errorf("Expected %s to be recognized as Spotify URL", url)
		}
	}

	for _, url := range invalidURLs {
		if IsSpotifyURL(url) {
			t.Errorf("Expected %s to NOT be recognized as Spotify URL", url)
		}
	}
}
