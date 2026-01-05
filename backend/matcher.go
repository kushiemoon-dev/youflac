package backend

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Audio/Video matching logic
// Priority:
// 1. ISRC exact match = 100% confidence
// 2. Duration match (±2s) = 90% confidence
// 3. Metadata fuzzy match = variable confidence (based on title/artist similarity)

// Matching thresholds
const (
	DurationTolerance     = 2.0  // seconds
	MinConfidenceThreshold = 0.6 // Minimum confidence to consider a match valid
	ISRCConfidence        = 1.0  // 100% confidence for ISRC match
	DurationConfidence    = 0.9  // 90% confidence for duration match
)

// MatchMethod indicates how a match was found
type MatchMethod string

const (
	MatchMethodISRC     MatchMethod = "isrc"
	MatchMethodDuration MatchMethod = "duration"
	MatchMethodMetadata MatchMethod = "metadata"
	MatchMethodNone     MatchMethod = "none"
)

// AudioCandidate represents a potential audio source for matching
type AudioCandidate struct {
	Platform    string  `json:"platform"`    // tidal, qobuz, amazon, deezer
	URL         string  `json:"url"`         // Direct URL to the track
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album,omitempty"`
	ISRC        string  `json:"isrc,omitempty"`
	Duration    float64 `json:"duration"`    // in seconds
	Quality     string  `json:"quality,omitempty"`
	Priority    int     `json:"priority"`    // Lower = higher priority (1 = Tidal, 2 = Qobuz, etc.)
}

// MatchResult contains the result of matching a video to audio
type MatchResult struct {
	Video         *VideoInfo       `json:"video"`
	Audio         *AudioCandidate  `json:"audio"`
	Confidence    float64          `json:"confidence"`    // 0.0 to 1.0
	MatchMethod   MatchMethod      `json:"matchMethod"`
	DurationDiff  float64          `json:"durationDiff"`  // Difference in seconds
	TitleScore    float64          `json:"titleScore"`    // 0.0 to 1.0
	ArtistScore   float64          `json:"artistScore"`   // 0.0 to 1.0
	IsValid       bool             `json:"isValid"`       // True if confidence >= threshold
	Warnings      []string         `json:"warnings,omitempty"`
}

// MatchOptions configures the matching behavior
type MatchOptions struct {
	RequireISRC           bool    // Only accept ISRC matches
	MaxDurationDiff       float64 // Override default duration tolerance
	MinMetadataConfidence float64 // Minimum confidence for metadata-only matches
	PreferredPlatform     string  // Prefer a specific platform if multiple matches
}

// DefaultMatchOptions returns sensible defaults
func DefaultMatchOptions() *MatchOptions {
	return &MatchOptions{
		RequireISRC:           false,
		MaxDurationDiff:       DurationTolerance,
		MinMetadataConfidence: 0.7,
		PreferredPlatform:     "", // No preference
	}
}

// MatchVideoToAudio finds the best audio source for a YouTube video
func MatchVideoToAudio(video *VideoInfo, candidates []AudioCandidate, opts *MatchOptions) (*MatchResult, error) {
	if video == nil {
		return nil, fmt.Errorf("video info is nil")
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no audio candidates provided")
	}
	if opts == nil {
		opts = DefaultMatchOptions()
	}

	var results []MatchResult

	for _, candidate := range candidates {
		result := matchSingle(video, &candidate, opts)
		if result.Confidence >= MinConfidenceThreshold {
			results = append(results, result)
		}
	}

	if len(results) == 0 {
		return &MatchResult{
			Video:       video,
			MatchMethod: MatchMethodNone,
			IsValid:     false,
			Warnings:    []string{"No matching audio found above confidence threshold"},
		}, nil
	}

	// Sort by confidence (highest first), then by platform priority (lowest first)
	sort.Slice(results, func(i, j int) bool {
		if results[i].Confidence != results[j].Confidence {
			return results[i].Confidence > results[j].Confidence
		}
		return results[i].Audio.Priority < results[j].Audio.Priority
	})

	// If preferred platform specified, try to find it among high-confidence matches
	if opts.PreferredPlatform != "" {
		for _, r := range results {
			if r.Audio.Platform == opts.PreferredPlatform && r.Confidence >= 0.9 {
				return &r, nil
			}
		}
	}

	return &results[0], nil
}

// matchSingle computes match result for a single video-audio pair
func matchSingle(video *VideoInfo, audio *AudioCandidate, opts *MatchOptions) MatchResult {
	result := MatchResult{
		Video:       video,
		Audio:       audio,
		MatchMethod: MatchMethodNone,
		IsValid:     false,
	}

	// Priority 1: ISRC exact match
	if MatchByISRC(video.ISRC, audio.ISRC) {
		result.MatchMethod = MatchMethodISRC
		result.Confidence = ISRCConfidence
		result.DurationDiff = math.Abs(video.Duration - audio.Duration)
		result.IsValid = true

		// Add warning if durations differ significantly despite ISRC match
		if result.DurationDiff > 5.0 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("ISRC match but duration differs by %.1fs - may be different version", result.DurationDiff))
		}
		return result
	}

	// If ISRC required but not matched, return low confidence
	if opts.RequireISRC {
		result.Warnings = append(result.Warnings, "ISRC match required but not found")
		return result
	}

	// Priority 2: Duration match
	durationDiff := math.Abs(video.Duration - audio.Duration)
	result.DurationDiff = durationDiff

	durationMatches := durationDiff <= opts.MaxDurationDiff

	// Priority 3: Metadata fuzzy match
	result.TitleScore = ComputeTitleSimilarity(video.Title, audio.Title)
	result.ArtistScore = ComputeArtistSimilarity(video.Artist, audio.Artist)
	metadataScore := (result.TitleScore*0.6 + result.ArtistScore*0.4) // Title weighted higher

	// Combine scores based on what matches
	if durationMatches && metadataScore >= opts.MinMetadataConfidence {
		// Both duration and metadata match - high confidence
		result.MatchMethod = MatchMethodDuration
		result.Confidence = DurationConfidence * metadataScore
		result.IsValid = true
	} else if durationMatches && metadataScore >= 0.5 {
		// Duration matches, metadata is partial
		result.MatchMethod = MatchMethodDuration
		result.Confidence = 0.8 * metadataScore
		result.IsValid = metadataScore >= MinConfidenceThreshold
		result.Warnings = append(result.Warnings, "Partial metadata match")
	} else if metadataScore >= 0.85 {
		// Strong metadata match without duration
		result.MatchMethod = MatchMethodMetadata
		result.Confidence = metadataScore * 0.85 // Penalize lack of duration match
		result.IsValid = result.Confidence >= MinConfidenceThreshold
		if durationDiff > opts.MaxDurationDiff {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Duration difference: %.1fs", durationDiff))
		}
	} else {
		// Weak match
		result.Confidence = metadataScore * 0.5
		result.IsValid = false
	}

	return result
}

// MatchByISRC checks if two ISRCs match exactly (case-insensitive)
func MatchByISRC(videoISRC, audioISRC string) bool {
	if videoISRC == "" || audioISRC == "" {
		return false
	}
	// Normalize ISRCs (remove hyphens, uppercase)
	v := normalizeISRC(videoISRC)
	a := normalizeISRC(audioISRC)
	return v == a
}

func normalizeISRC(isrc string) string {
	isrc = strings.ToUpper(isrc)
	isrc = strings.ReplaceAll(isrc, "-", "")
	isrc = strings.ReplaceAll(isrc, " ", "")
	return isrc
}

// MatchByDuration checks if durations are within tolerance
func MatchByDuration(videoDuration, audioDuration float64) bool {
	diff := math.Abs(videoDuration - audioDuration)
	return diff <= DurationTolerance
}

// ComputeTitleSimilarity computes similarity between video and audio titles
func ComputeTitleSimilarity(videoTitle, audioTitle string) float64 {
	v := normalizeTitle(videoTitle)
	a := normalizeTitle(audioTitle)

	if v == a {
		return 1.0
	}

	// Try without common suffixes
	vClean := removeCommonSuffixes(v)
	aClean := removeCommonSuffixes(a)

	if vClean == aClean {
		return 0.95
	}

	// Levenshtein-based similarity
	return stringSimilarity(vClean, aClean)
}

// ComputeArtistSimilarity computes similarity between artists
func ComputeArtistSimilarity(videoArtist, audioArtist string) float64 {
	v := normalizeArtist(videoArtist)
	a := normalizeArtist(audioArtist)

	if v == a {
		return 1.0
	}

	// Check if one contains the other (for "Artist ft. Other" cases)
	if strings.Contains(v, a) || strings.Contains(a, v) {
		return 0.9
	}

	// Check primary artist (before "feat", ",", "&")
	vPrimary := extractPrimaryArtist(v)
	aPrimary := extractPrimaryArtist(a)

	if vPrimary == aPrimary {
		return 0.95
	}

	return stringSimilarity(vPrimary, aPrimary)
}

// normalizeTitle prepares a title for comparison
func normalizeTitle(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))

	// Remove content in parentheses/brackets that's usually metadata
	// e.g., "(Official Video)", "[HD]", "(Remastered 2021)"
	title = regexp.MustCompile(`\([^)]*(?:video|audio|lyric|official|hd|hq|4k|remaster|remix|version|edit|live|acoustic)\w*[^)]*\)`).ReplaceAllString(title, "")
	title = regexp.MustCompile(`\[[^\]]*(?:video|audio|lyric|official|hd|hq|4k|remaster|remix|version|edit|live|acoustic)\w*[^\]]*\]`).ReplaceAllString(title, "")

	// Remove remaster/version patterns with years (handles both orders)
	// "- Remastered 2021", "- 2021 Remaster", "Remastered 2021", etc.
	title = regexp.MustCompile(`\s*-?\s*(remaster(ed)?|version|edition|mix)\s*\d{4}\s*$`).ReplaceAllString(title, "")
	title = regexp.MustCompile(`\s*-?\s*\d{4}\s*(remaster(ed)?|version|edition|mix)?\s*$`).ReplaceAllString(title, "")

	// Remove common video-specific suffixes (both " - suffix" and " suffix" patterns)
	// Run this AFTER year patterns so "- remastered" remaining gets cleaned
	suffixes := []string{
		"official video", "official music video", "music video",
		"official audio", "audio", "lyric video", "lyrics",
		"hd", "hq", "4k", "uhd",
		"visualizer", "visualiser",
		"remastered", "remaster",
	}
	for _, suffix := range suffixes {
		// Remove " - suffix" pattern
		title = regexp.MustCompile(`\s*-\s*`+regexp.QuoteMeta(suffix)+`\s*$`).ReplaceAllString(title, "")
		// Remove " suffix" pattern
		title = regexp.MustCompile(`\s+`+regexp.QuoteMeta(suffix)+`\s*$`).ReplaceAllString(title, "")
	}

	// Clean up any remaining trailing dashes
	title = regexp.MustCompile(`\s*-\s*$`).ReplaceAllString(title, "")

	// Normalize whitespace
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)

	return title
}

// normalizeArtist prepares an artist name for comparison
func normalizeArtist(artist string) string {
	artist = strings.ToLower(strings.TrimSpace(artist))

	// Common variations
	artist = strings.ReplaceAll(artist, " vevo", "")
	artist = strings.ReplaceAll(artist, "vevo", "")
	artist = strings.ReplaceAll(artist, " - topic", "")

	// Normalize whitespace and punctuation
	artist = regexp.MustCompile(`\s+`).ReplaceAllString(artist, " ")
	artist = strings.TrimSpace(artist)

	return artist
}

// extractPrimaryArtist gets the main artist before featuring artists
func extractPrimaryArtist(artist string) string {
	// Split on common featuring patterns
	separators := []string{" feat.", " feat ", " ft.", " ft ", " featuring ", " x ", " & ", ", "}
	result := artist
	for _, sep := range separators {
		if idx := strings.Index(strings.ToLower(result), sep); idx > 0 {
			result = result[:idx]
		}
	}
	return strings.TrimSpace(result)
}

// removeCommonSuffixes removes version indicators from titles
func removeCommonSuffixes(title string) string {
	// Remove year suffixes like "- 2021 Remaster"
	title = regexp.MustCompile(`\s*-?\s*\d{4}\s*(remaster|version|edition|mix)?`).ReplaceAllString(title, "")

	// Remove common version indicators
	patterns := []string{
		`\s*\(remaster(ed)?\)`,
		`\s*\(deluxe\s*(edition)?\)`,
		`\s*\(expanded\s*(edition)?\)`,
		`\s*\(anniversary\s*(edition)?\)`,
		`\s*\(single\s*(version)?\)`,
		`\s*\(album\s*(version)?\)`,
		`\s*\(radio\s*(edit)?\)`,
		`\s*\(extended\s*(version|mix)?\)`,
	}
	for _, pattern := range patterns {
		title = regexp.MustCompile(pattern).ReplaceAllString(title, "")
	}

	return strings.TrimSpace(title)
}

// stringSimilarity computes normalized similarity between two strings (0.0 to 1.0)
// Uses Levenshtein distance normalized by the longer string length
func stringSimilarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))

	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	distance := levenshteinDistance(a, b)
	maxLen := max(len(a), len(b))

	return 1.0 - float64(distance)/float64(maxLen)
}

// levenshteinDistance computes the edit distance between two strings
func levenshteinDistance(a, b string) int {
	aRunes := []rune(a)
	bRunes := []rune(b)
	aLen := len(aRunes)
	bLen := len(bRunes)

	if aLen == 0 {
		return bLen
	}
	if bLen == 0 {
		return aLen
	}

	// Create distance matrix
	matrix := make([][]int, aLen+1)
	for i := range matrix {
		matrix[i] = make([]int, bLen+1)
		matrix[i][0] = i
	}
	for j := 0; j <= bLen; j++ {
		matrix[0][j] = j
	}

	// Fill in the matrix
	for i := 1; i <= aLen; i++ {
		for j := 1; j <= bLen; j++ {
			cost := 1
			if unicode.ToLower(aRunes[i-1]) == unicode.ToLower(bRunes[j-1]) {
				cost = 0
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[aLen][bLen]
}

// ============================================================================
// High-level matching functions for the app
// ============================================================================

// MatchYouTubeToFLAC matches a YouTube video to the best FLAC source
// Uses song.link to resolve and find matching audio on streaming platforms
func MatchYouTubeToFLAC(youtubeURL string) (*MatchResult, error) {
	// 1. Get YouTube video metadata
	videoID, err := ParseYouTubeURL(youtubeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YouTube URL: %w", err)
	}

	video, err := GetVideoMetadata(videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get video metadata: %w", err)
	}

	// 2. Try to resolve via song.link
	// First, try using the YouTube Music URL if available
	ytMusicURL := fmt.Sprintf("https://music.youtube.com/watch?v=%s", videoID)
	songInfo, err := ResolveMusicURL(ytMusicURL)
	if err != nil {
		// Fallback to regular YouTube URL
		songInfo, err = ResolveMusicURL(youtubeURL)
	}

	if err != nil {
		// If song.link fails, we can't cross-reference
		return &MatchResult{
			Video:       video,
			MatchMethod: MatchMethodNone,
			IsValid:     false,
			Warnings:    []string{fmt.Sprintf("Could not resolve on song.link: %v", err)},
		}, nil
	}

	// 3. Build audio candidates from song.link results
	candidates := buildCandidatesFromSongLink(songInfo)
	if len(candidates) == 0 {
		return &MatchResult{
			Video:       video,
			MatchMethod: MatchMethodNone,
			IsValid:     false,
			Warnings:    []string{"No FLAC sources found via song.link"},
		}, nil
	}

	// 4. Add ISRC from song.link to video if missing
	if video.ISRC == "" && songInfo.ISRC != "" {
		video.ISRC = songInfo.ISRC
	}

	// 5. Match
	return MatchVideoToAudio(video, candidates, nil)
}

// MatchSpotifyToFLAC matches a Spotify track to the best FLAC source
func MatchSpotifyToFLAC(spotifyURL string) (*MatchResult, error) {
	// 1. Resolve Spotify URL via song.link
	songInfo, err := ResolveMusicURL(spotifyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Spotify URL: %w", err)
	}

	// 2. Create a "pseudo-video" info from Spotify metadata
	video := &VideoInfo{
		ID:        songInfo.SpotifyID,
		Title:     songInfo.Title,
		Artist:    songInfo.Artist,
		ISRC:      songInfo.ISRC,
		Thumbnail: songInfo.Thumbnail,
		URL:       spotifyURL,
	}

	// 3. Build audio candidates
	candidates := buildCandidatesFromSongLink(songInfo)
	if len(candidates) == 0 {
		return &MatchResult{
			Video:       video,
			MatchMethod: MatchMethodNone,
			IsValid:     false,
			Warnings:    []string{"No FLAC sources found"},
		}, nil
	}

	// 4. Match (ISRC should always match for same track)
	return MatchVideoToAudio(video, candidates, nil)
}

// buildCandidatesFromSongLink creates AudioCandidate list from song.link response
func buildCandidatesFromSongLink(info *SongLinkTrackInfo) []AudioCandidate {
	var candidates []AudioCandidate

	// Add platforms in priority order
	if info.URLs.TidalURL != "" {
		candidates = append(candidates, AudioCandidate{
			Platform: "tidal",
			URL:      info.URLs.TidalURL,
			Title:    info.Title,
			Artist:   info.Artist,
			ISRC:     info.ISRC,
			Priority: 1,
			Quality:  "FLAC (up to 24-bit/192kHz)",
		})
	}

	if info.URLs.QobuzURL != "" {
		candidates = append(candidates, AudioCandidate{
			Platform: "qobuz",
			URL:      info.URLs.QobuzURL,
			Title:    info.Title,
			Artist:   info.Artist,
			ISRC:     info.ISRC,
			Priority: 2,
			Quality:  "FLAC (up to 24-bit/192kHz)",
		})
	}

	if info.URLs.AmazonURL != "" {
		candidates = append(candidates, AudioCandidate{
			Platform: "amazon",
			URL:      info.URLs.AmazonURL,
			Title:    info.Title,
			Artist:   info.Artist,
			ISRC:     info.ISRC,
			Priority: 3,
			Quality:  "FLAC (up to 24-bit/192kHz)",
		})
	}

	if info.URLs.DeezerURL != "" {
		candidates = append(candidates, AudioCandidate{
			Platform: "deezer",
			URL:      info.URLs.DeezerURL,
			Title:    info.Title,
			Artist:   info.Artist,
			ISRC:     info.ISRC,
			Priority: 4,
			Quality:  "FLAC (16-bit/44.1kHz)",
		})
	}

	return candidates
}

// GetMatchConfidenceLabel returns a human-readable confidence label
func GetMatchConfidenceLabel(confidence float64) string {
	switch {
	case confidence >= 0.95:
		return "Excellent"
	case confidence >= 0.85:
		return "Very Good"
	case confidence >= 0.75:
		return "Good"
	case confidence >= 0.65:
		return "Fair"
	case confidence >= MinConfidenceThreshold:
		return "Acceptable"
	default:
		return "Poor"
	}
}

// GetMatchMethodLabel returns a human-readable match method label
func GetMatchMethodLabel(method MatchMethod) string {
	switch method {
	case MatchMethodISRC:
		return "ISRC Match (Exact)"
	case MatchMethodDuration:
		return "Duration + Metadata Match"
	case MatchMethodMetadata:
		return "Metadata Match"
	case MatchMethodNone:
		return "No Match"
	default:
		return string(method)
	}
}
