package backend

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ============================================================================
// Tidal HiFi API Service - vogel.qqdl.site (SpotiFLAC pattern)
// Free Tidal FLAC proxy without credentials
// ============================================================================

const (
	tidalHifiAPIBase = "https://vogel.qqdl.site"
)

// TidalHifiService implements AudioDownloadService using the hifi-api
type TidalHifiService struct {
	client  *http.Client
	baseURL string
}

// TidalManifest represents the decoded manifest from hifi-api
type TidalManifest struct {
	MimeType       string   `json:"mimeType"`
	Codecs         string   `json:"codecs"`
	EncryptionType string   `json:"encryptionType"`
	URLs           []string `json:"urls"`
}

// TidalTrackResponse represents the track info response
type TidalTrackResponse struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Duration    int    `json:"duration"`
	TrackNumber int    `json:"trackNumber"`
	ISRC        string `json:"isrc"`
	Explicit    bool   `json:"explicit"`
	Artist      struct {
		Name string `json:"name"`
	} `json:"artist"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Title string `json:"title"`
		Cover string `json:"cover"`
	} `json:"album"`
}

// TidalStreamResponse represents the stream/manifest response
type TidalStreamResponse struct {
	TrackID      int    `json:"trackId"`
	AssetID      int    `json:"assetId,omitempty"`
	AudioMode    string `json:"audioMode"`
	AudioQuality string `json:"audioQuality"`
	Manifest     string `json:"manifest"`        // Base64 encoded
	ManifestType string `json:"manifestMimeType"`
}

// TidalSearchResponse represents search results
type TidalSearchResponse struct {
	Version string `json:"version,omitempty"`
	Data    struct {
		Items []TidalTrackResponse `json:"items"`
	} `json:"data,omitempty"`
	Tracks struct {
		Items []TidalTrackResponse `json:"items"`
	} `json:"tracks,omitempty"`
}

// TidalInfoResponse wraps track info with version
type TidalInfoResponse struct {
	Version string             `json:"version"`
	Data    TidalTrackResponse `json:"data"`
}

// TidalStreamDataResponse wraps stream response with version
type TidalStreamDataResponse struct {
	Version string              `json:"version"`
	Data    TidalStreamResponse `json:"data"`
}

// NewTidalHifiService creates a new Tidal HiFi download service.
// If client is nil, a default client is used (respects PROXY_URL env var).
func NewTidalHifiService(client *http.Client) *TidalHifiService {
	if client == nil {
		client, _ = NewHTTPClient(0, "")
	}
	return &TidalHifiService{
		client:  client,
		baseURL: tidalHifiAPIBase,
	}
}

func (t *TidalHifiService) Name() string {
	return "tidal-hifi"
}

func (t *TidalHifiService) IsAvailable() bool {
	resp, err := t.client.Head(t.baseURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

func (t *TidalHifiService) SupportsFormat(format string) bool {
	return strings.ToLower(format) == "flac"
}

// SearchTrack searches for a track on Tidal
func (t *TidalHifiService) SearchTrack(query string) (*TidalTrackResponse, error) {
	searchURL := fmt.Sprintf("%s/search/?s=%s", t.baseURL, url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read search response: %w", err)
	}

	var searchResp TidalSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	var items []TidalTrackResponse
	if len(searchResp.Data.Items) > 0 {
		items = searchResp.Data.Items
	} else if len(searchResp.Tracks.Items) > 0 {
		items = searchResp.Tracks.Items
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no tracks found for query: %s", query)
	}

	return &items[0], nil
}

// GetTrackByID fetches track info by Tidal ID
func (t *TidalHifiService) GetTrackByID(trackID int) (*TidalTrackResponse, error) {
	infoURL := fmt.Sprintf("%s/info/?id=%d", t.baseURL, trackID)

	req, err := http.NewRequest("GET", infoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("info request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read info response: %w", err)
	}

	// Try v2.0 wrapper format first
	var infoResp TidalInfoResponse
	if err := json.Unmarshal(body, &infoResp); err != nil {
		return nil, fmt.Errorf("failed to parse track info: %w", err)
	}

	if infoResp.Data.ID > 0 {
		return &infoResp.Data, nil
	}

	// Fallback: try direct format
	var trackInfo TidalTrackResponse
	if err := json.Unmarshal(body, &trackInfo); err != nil {
		return nil, fmt.Errorf("failed to parse track info (direct): %w", err)
	}

	return &trackInfo, nil
}

// GetStreamURL fetches the FLAC stream URL for a track
func (t *TidalHifiService) GetStreamURL(trackID int) (string, error) {
	streamURL := fmt.Sprintf("%s/track/?id=%d&quality=LOSSLESS", t.baseURL, trackID)

	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("stream request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read stream response: %w", err)
	}

	// Try v2.0 wrapper format first
	var streamDataResp TidalStreamDataResponse
	if err := json.Unmarshal(body, &streamDataResp); err != nil {
		return "", fmt.Errorf("failed to parse stream response: %w", err)
	}

	manifestBase64 := streamDataResp.Data.Manifest
	if manifestBase64 == "" {
		var streamResp TidalStreamResponse
		if err := json.Unmarshal(body, &streamResp); err != nil {
			return "", fmt.Errorf("failed to parse stream response (direct): %w", err)
		}
		manifestBase64 = streamResp.Manifest
	}

	if manifestBase64 == "" {
		return "", fmt.Errorf("no manifest in stream response")
	}

	manifestBytes, err := base64.StdEncoding.DecodeString(manifestBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode manifest: %w", err)
	}

	var manifest TidalManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return "", fmt.Errorf("failed to parse manifest: %w", err)
	}

	if len(manifest.URLs) == 0 {
		return "", fmt.Errorf("no download URLs in manifest")
	}

	return manifest.URLs[0], nil
}

// ExtractTidalID extracts the track ID from a Tidal URL
func ExtractTidalID(tidalURL string) (int, error) {
	patterns := []string{
		`tidal\.com/browse/track/(\d+)`,
		`listen\.tidal\.com/track/(\d+)`,
		`tidal:track:(\d+)`,
		`/track/(\d+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(tidalURL); len(matches) > 1 {
			var id int
			fmt.Sscanf(matches[1], "%d", &id)
			return id, nil
		}
	}

	return 0, fmt.Errorf("could not extract Tidal track ID from URL: %s", tidalURL)
}

func (t *TidalHifiService) GetTrackInfo(trackURL string) (*AudioTrackInfo, error) {
	trackID, err := ExtractTidalID(trackURL)
	if err != nil {
		return nil, err
	}

	track, err := t.GetTrackByID(trackID)
	if err != nil {
		return nil, err
	}

	artistName := track.Artist.Name
	if artistName == "" && len(track.Artists) > 0 {
		artistName = track.Artists[0].Name
	}

	return &AudioTrackInfo{
		ID:       fmt.Sprintf("%d", track.ID),
		Title:    track.Title,
		Artist:   artistName,
		Album:    track.Album.Title,
		ISRC:     track.ISRC,
		Duration: float64(track.Duration),
		Quality:  "FLAC 16-bit/44.1kHz",
		Platform: "tidal",
		CoverURL: fmt.Sprintf("https://resources.tidal.com/images/%s/640x640.jpg", strings.ReplaceAll(track.Album.Cover, "-", "/")),
	}, nil
}

func (t *TidalHifiService) Download(trackURL string, outputDir string, format string) (*AudioDownloadResult, error) {
	trackID, err := ExtractTidalID(trackURL)
	if err != nil {
		return nil, err
	}

	track, err := t.GetTrackByID(trackID)
	if err != nil {
		return nil, fmt.Errorf("failed to get track info: %w", err)
	}

	streamURL, err := t.GetStreamURL(trackID)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream URL: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	artistName := track.Artist.Name
	if artistName == "" && len(track.Artists) > 0 {
		artistName = track.Artists[0].Name
	}
	safeTitle := SanitizeFileName(fmt.Sprintf("%s - %s", artistName, track.Title))
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.flac", safeTitle))

	if err := t.downloadFile(streamURL, outputPath); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	stat, _ := os.Stat(outputPath)
	var fileSize int64
	if stat != nil {
		fileSize = stat.Size()
	}

	return &AudioDownloadResult{
		FilePath: outputPath,
		Track: &AudioTrackInfo{
			ID:       fmt.Sprintf("%d", track.ID),
			Title:    track.Title,
			Artist:   artistName,
			Album:    track.Album.Title,
			Duration: float64(track.Duration),
			ISRC:     track.ISRC,
			Platform: "tidal",
			Quality:  "FLAC LOSSLESS",
			CoverURL: fmt.Sprintf("https://resources.tidal.com/images/%s/640x640.jpg", strings.ReplaceAll(track.Album.Cover, "-", "/")),
		},
		Format: "flac",
		Size:   fileSize,
	}, nil
}

func (t *TidalHifiService) downloadFile(downloadURL, outputPath string) error {
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download server returned %d", resp.StatusCode)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	if _, err = io.Copy(outFile, resp.Body); err != nil {
		os.Remove(outputPath)
		return fmt.Errorf("download interrupted: %w", err)
	}

	return nil
}

// DownloadBySearch downloads FLAC by searching for artist + title
func (t *TidalHifiService) DownloadBySearch(artist, title, outputDir string) (*AudioDownloadResult, error) {
	query := fmt.Sprintf("%s %s", artist, title)

	track, err := t.SearchTrack(query)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	trackURL := fmt.Sprintf("https://tidal.com/browse/track/%d", track.ID)
	return t.Download(trackURL, outputDir, "flac")
}

// ============================================================================
// OrpheusDL Service - Python subprocess for high-quality FLAC
// Requires: Python 3 + orpheusdl installed (pip install orpheusdl)
// ============================================================================

// OrpheusDLService uses OrpheusDL Python tool as subprocess
type OrpheusDLService struct {
	pythonPath string
}

// NewOrpheusDLService creates a new OrpheusDL service
func NewOrpheusDLService() *OrpheusDLService {
	pythonPath := "python3"
	if _, err := exec.LookPath("python3"); err != nil {
		pythonPath = "python"
	}
	return &OrpheusDLService{
		pythonPath: pythonPath,
	}
}

func (o *OrpheusDLService) Name() string {
	return "orpheusdl"
}

func (o *OrpheusDLService) IsAvailable() bool {
	if err := exec.Command("rip", "--version").Run(); err == nil {
		return true
	}
	return exec.Command(o.pythonPath, "-m", "streamrip", "--version").Run() == nil
}

func (o *OrpheusDLService) SupportsFormat(format string) bool {
	return strings.ToLower(format) == "flac"
}

func (o *OrpheusDLService) GetTrackInfo(trackURL string) (*AudioTrackInfo, error) {
	return nil, fmt.Errorf("orpheusdl does not support metadata-only queries")
}

func (o *OrpheusDLService) Download(trackURL string, outputDir string, format string) (*AudioDownloadResult, error) {
	result, err := o.tryStreamrip(trackURL, outputDir)
	if err == nil {
		return result, nil
	}
	return o.tryOrpheusDL(trackURL, outputDir)
}

func (o *OrpheusDLService) tryStreamrip(trackURL string, outputDir string) (*AudioDownloadResult, error) {
	if err := ValidateTrackURL(trackURL); err != nil {
		return nil, fmt.Errorf("rejected track URL: %w", err)
	}

	cmd := exec.Command("rip", "url", trackURL)
	cmd.Dir = outputDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		cmd2 := exec.Command(o.pythonPath, "-m", "streamrip", "url", trackURL)
		cmd2.Dir = outputDir
		output, err = cmd2.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("streamrip failed: %w - %s", err, string(output))
		}
	}

	homeDir, _ := os.UserHomeDir()
	musicDir := filepath.Join(homeDir, "music")

	flacFile, err := o.findDownloadedFLAC(outputDir)
	if err != nil {
		flacFile, err = o.findDownloadedFLAC(musicDir)
		if err != nil {
			return nil, fmt.Errorf("FLAC file not found after download: %w", err)
		}
	}

	return &AudioDownloadResult{
		FilePath: flacFile,
		Format:   "flac",
		Track: &AudioTrackInfo{
			Platform: "streamrip",
			Quality:  "FLAC",
		},
	}, nil
}

func (o *OrpheusDLService) tryOrpheusDL(trackURL string, outputDir string) (*AudioDownloadResult, error) {
	if err := ValidateTrackURL(trackURL); err != nil {
		return nil, fmt.Errorf("rejected track URL: %w", err)
	}

	cmd := exec.Command(o.pythonPath, "-m", "orpheusdl", trackURL, "-o", outputDir, "-q", "flac")
	cmd.Dir = outputDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("orpheusdl failed: %w - %s", err, string(output))
	}

	flacFile, err := o.findDownloadedFLAC(outputDir)
	if err != nil {
		return nil, err
	}

	return &AudioDownloadResult{
		FilePath: flacFile,
		Format:   "flac",
		Track: &AudioTrackInfo{
			Platform: "orpheusdl",
			Quality:  "FLAC 24-bit",
		},
	}, nil
}

func (o *OrpheusDLService) findDownloadedFLAC(dir string) (string, error) {
	var flacFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".flac") {
			flacFiles = append(flacFiles, path)
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to search for FLAC: %w", err)
	}

	if len(flacFiles) == 0 {
		return "", fmt.Errorf("no FLAC file found in output directory")
	}

	// Return the most recently modified FLAC file
	var newestFile string
	var newestTime time.Time

	for _, f := range flacFiles {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		if info.ModTime().After(newestTime) {
			newestTime = info.ModTime()
			newestFile = f
		}
	}

	return newestFile, nil
}
