package backend

import (
	"fmt"
	"testing"
)

func TestMatchByISRC(t *testing.T) {
	tests := []struct {
		name     string
		videoISRC string
		audioISRC string
		expected bool
	}{
		{"exact match", "USRC11700001", "USRC11700001", true},
		{"case insensitive", "usrc11700001", "USRC11700001", true},
		{"with hyphens", "US-RC1-17-00001", "USRC11700001", true},
		{"both empty", "", "", false},
		{"video empty", "", "USRC11700001", false},
		{"audio empty", "USRC11700001", "", false},
		{"mismatch", "USRC11700001", "USRC11700002", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchByISRC(tt.videoISRC, tt.audioISRC)
			if result != tt.expected {
				t.Errorf("MatchByISRC(%q, %q) = %v, want %v", tt.videoISRC, tt.audioISRC, result, tt.expected)
			}
		})
	}
}

func TestMatchByDuration(t *testing.T) {
	tests := []struct {
		name          string
		videoDuration float64
		audioDuration float64
		expected      bool
	}{
		{"exact match", 213.0, 213.0, true},
		{"within tolerance +", 213.0, 214.5, true},
		{"within tolerance -", 213.0, 211.5, true},
		{"at tolerance boundary", 213.0, 215.0, true},
		{"exceeds tolerance", 213.0, 216.0, false},
		{"large difference", 213.0, 300.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchByDuration(tt.videoDuration, tt.audioDuration)
			if result != tt.expected {
				t.Errorf("MatchByDuration(%v, %v) = %v, want %v", tt.videoDuration, tt.audioDuration, result, tt.expected)
			}
		})
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Never Gonna Give You Up", "never gonna give you up"},
		{"Never Gonna Give You Up (Official Video)", "never gonna give you up"},
		{"Never Gonna Give You Up [Official Music Video]", "never gonna give you up"},
		{"Never Gonna Give You Up - Official Video", "never gonna give you up"},
		{"Never Gonna Give You Up (Remastered 2021)", "never gonna give you up"},
		{"Song Title (HD)", "song title"},
		{"Song Title [4K]", "song title"},
		{"Song Title - Lyric Video", "song title"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeTitle(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeTitle(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeArtist(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Rick Astley", "rick astley"},
		{"RickAstleyVEVO", "rickastley"},
		{"Rick Astley - Topic", "rick astley"},
		{"Rick Astley VEVO", "rick astley"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeArtist(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeArtist(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractPrimaryArtist(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"rick astley", "rick astley"},
		{"rick astley feat. john doe", "rick astley"},
		{"rick astley ft. john doe", "rick astley"},
		{"rick astley featuring john doe", "rick astley"},
		{"rick astley x john doe", "rick astley"},
		{"rick astley & john doe", "rick astley"},
		{"rick astley, john doe", "rick astley"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractPrimaryArtist(tt.input)
			if result != tt.expected {
				t.Errorf("extractPrimaryArtist(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStringSimilarity(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		minScore float64
	}{
		{"never gonna give you up", "never gonna give you up", 1.0},
		{"never gonna give you up", "Never Gonna Give You Up", 1.0},
		{"never gonna give you up", "never gonna give u up", 0.9},
		{"rick astley", "rick astley", 1.0},
		{"rick astley", "Rick Astley", 1.0},
		{"completely different", "something else", 0.0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s vs %s", tt.a, tt.b), func(t *testing.T) {
			result := stringSimilarity(tt.a, tt.b)
			if result < tt.minScore {
				t.Errorf("stringSimilarity(%q, %q) = %v, want >= %v", tt.a, tt.b, result, tt.minScore)
			}
		})
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "ab", 1},
		{"abc", "abcd", 1},
		{"kitten", "sitting", 3},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s to %s", tt.a, tt.b), func(t *testing.T) {
			result := levenshteinDistance(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestComputeTitleSimilarity(t *testing.T) {
	tests := []struct {
		videoTitle string
		audioTitle string
		minScore   float64
	}{
		{"Never Gonna Give You Up", "Never Gonna Give You Up", 1.0},
		{"Never Gonna Give You Up (Official Video)", "Never Gonna Give You Up", 0.95},
		{"Never Gonna Give You Up [HD]", "Never Gonna Give You Up", 0.95},
		{"Never Gonna Give You Up - Remastered 2021", "Never Gonna Give You Up", 0.9},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s vs %s", tt.videoTitle, tt.audioTitle), func(t *testing.T) {
			result := ComputeTitleSimilarity(tt.videoTitle, tt.audioTitle)
			if result < tt.minScore {
				t.Errorf("ComputeTitleSimilarity(%q, %q) = %v, want >= %v", tt.videoTitle, tt.audioTitle, result, tt.minScore)
			}
		})
	}
}

func TestComputeArtistSimilarity(t *testing.T) {
	tests := []struct {
		videoArtist string
		audioArtist string
		minScore    float64
	}{
		{"Rick Astley", "Rick Astley", 1.0},
		{"RickAstleyVEVO", "Rick Astley", 0.8},
		{"Rick Astley - Topic", "Rick Astley", 0.95},
		{"Rick Astley feat. Someone", "Rick Astley", 0.9},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s vs %s", tt.videoArtist, tt.audioArtist), func(t *testing.T) {
			result := ComputeArtistSimilarity(tt.videoArtist, tt.audioArtist)
			if result < tt.minScore {
				t.Errorf("ComputeArtistSimilarity(%q, %q) = %v, want >= %v", tt.videoArtist, tt.audioArtist, result, tt.minScore)
			}
		})
	}
}

func TestMatchVideoToAudio(t *testing.T) {
	video := &VideoInfo{
		ID:       "dQw4w9WgXcQ",
		Title:    "Never Gonna Give You Up",
		Artist:   "Rick Astley",
		Duration: 213.0,
		ISRC:     "GBARL9300135",
	}

	candidates := []AudioCandidate{
		{
			Platform: "tidal",
			URL:      "https://tidal.com/track/12345",
			Title:    "Never Gonna Give You Up",
			Artist:   "Rick Astley",
			Duration: 213.0,
			ISRC:     "GBARL9300135",
			Priority: 1,
		},
		{
			Platform: "qobuz",
			URL:      "https://qobuz.com/track/67890",
			Title:    "Never Gonna Give You Up",
			Artist:   "Rick Astley",
			Duration: 213.0,
			ISRC:     "GBARL9300135",
			Priority: 2,
		},
	}

	result, err := MatchVideoToAudio(video, candidates, nil)
	if err != nil {
		t.Fatalf("MatchVideoToAudio failed: %v", err)
	}

	if !result.IsValid {
		t.Error("Expected valid match")
	}

	if result.MatchMethod != MatchMethodISRC {
		t.Errorf("Expected ISRC match, got %s", result.MatchMethod)
	}

	if result.Confidence != 1.0 {
		t.Errorf("Expected 100%% confidence for ISRC match, got %v", result.Confidence)
	}

	if result.Audio.Platform != "tidal" {
		t.Errorf("Expected tidal (highest priority), got %s", result.Audio.Platform)
	}

	fmt.Printf("Match Result:\n")
	fmt.Printf("  Method: %s\n", GetMatchMethodLabel(result.MatchMethod))
	fmt.Printf("  Confidence: %.0f%% (%s)\n", result.Confidence*100, GetMatchConfidenceLabel(result.Confidence))
	fmt.Printf("  Platform: %s\n", result.Audio.Platform)
}

func TestMatchVideoToAudio_DurationMatch(t *testing.T) {
	// Video without ISRC, so must match by duration + metadata
	video := &VideoInfo{
		ID:       "dQw4w9WgXcQ",
		Title:    "Never Gonna Give You Up (Official Video)",
		Artist:   "Rick Astley",
		Duration: 213.0,
	}

	candidates := []AudioCandidate{
		{
			Platform: "tidal",
			URL:      "https://tidal.com/track/12345",
			Title:    "Never Gonna Give You Up",
			Artist:   "Rick Astley",
			Duration: 214.0, // Within tolerance
			Priority: 1,
		},
	}

	result, err := MatchVideoToAudio(video, candidates, nil)
	if err != nil {
		t.Fatalf("MatchVideoToAudio failed: %v", err)
	}

	if !result.IsValid {
		t.Error("Expected valid match")
	}

	if result.MatchMethod != MatchMethodDuration {
		t.Errorf("Expected duration match, got %s", result.MatchMethod)
	}

	if result.Confidence < 0.8 {
		t.Errorf("Expected confidence >= 80%%, got %v", result.Confidence)
	}

	fmt.Printf("Duration Match Result:\n")
	fmt.Printf("  Method: %s\n", GetMatchMethodLabel(result.MatchMethod))
	fmt.Printf("  Confidence: %.0f%% (%s)\n", result.Confidence*100, GetMatchConfidenceLabel(result.Confidence))
	fmt.Printf("  Title Score: %.0f%%\n", result.TitleScore*100)
	fmt.Printf("  Artist Score: %.0f%%\n", result.ArtistScore*100)
	fmt.Printf("  Duration Diff: %.1fs\n", result.DurationDiff)
}

func TestMatchVideoToAudio_NoMatch(t *testing.T) {
	video := &VideoInfo{
		ID:       "xyz123",
		Title:    "Completely Different Song",
		Artist:   "Unknown Artist",
		Duration: 180.0,
	}

	candidates := []AudioCandidate{
		{
			Platform: "tidal",
			URL:      "https://tidal.com/track/12345",
			Title:    "Never Gonna Give You Up",
			Artist:   "Rick Astley",
			Duration: 213.0,
			Priority: 1,
		},
	}

	result, err := MatchVideoToAudio(video, candidates, nil)
	if err != nil {
		t.Fatalf("MatchVideoToAudio failed: %v", err)
	}

	if result.IsValid {
		t.Error("Expected no valid match for completely different content")
	}

	fmt.Printf("No Match Result:\n")
	fmt.Printf("  Valid: %v\n", result.IsValid)
	fmt.Printf("  Confidence: %.0f%%\n", result.Confidence*100)
	if len(result.Warnings) > 0 {
		fmt.Printf("  Warning: %s\n", result.Warnings[0])
	}
}

func TestGetMatchConfidenceLabel(t *testing.T) {
	tests := []struct {
		confidence float64
		expected   string
	}{
		{1.0, "Excellent"},
		{0.95, "Excellent"},
		{0.90, "Very Good"},
		{0.80, "Good"},
		{0.70, "Fair"},
		{0.60, "Acceptable"},
		{0.50, "Poor"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.0f%%", tt.confidence*100), func(t *testing.T) {
			result := GetMatchConfidenceLabel(tt.confidence)
			if result != tt.expected {
				t.Errorf("GetMatchConfidenceLabel(%v) = %q, want %q", tt.confidence, result, tt.expected)
			}
		})
	}
}

// Integration test - requires network access
func TestMatchYouTubeToFLAC_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Rick Astley - Never Gonna Give You Up
	youtubeURL := "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

	fmt.Println("=== Integration Test: YouTube to FLAC Matching ===")
	fmt.Println("URL:", youtubeURL)
	fmt.Println()

	result, err := MatchYouTubeToFLAC(youtubeURL)
	if err != nil {
		t.Fatalf("MatchYouTubeToFLAC failed: %v", err)
	}

	fmt.Println("Video Info:")
	if result.Video != nil {
		fmt.Printf("  ID: %s\n", result.Video.ID)
		fmt.Printf("  Title: %s\n", result.Video.Title)
		fmt.Printf("  Artist: %s\n", result.Video.Artist)
		fmt.Printf("  Duration: %.0fs\n", result.Video.Duration)
		fmt.Printf("  ISRC: %s\n", result.Video.ISRC)
	}
	fmt.Println()

	fmt.Println("Match Result:")
	fmt.Printf("  Valid: %v\n", result.IsValid)
	fmt.Printf("  Method: %s\n", GetMatchMethodLabel(result.MatchMethod))
	fmt.Printf("  Confidence: %.0f%% (%s)\n", result.Confidence*100, GetMatchConfidenceLabel(result.Confidence))

	if result.Audio != nil {
		fmt.Println()
		fmt.Println("Best Audio Source:")
		fmt.Printf("  Platform: %s\n", result.Audio.Platform)
		fmt.Printf("  URL: %s\n", result.Audio.URL)
		fmt.Printf("  Quality: %s\n", result.Audio.Quality)
	}

	if len(result.Warnings) > 0 {
		fmt.Println()
		fmt.Println("Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}
}
