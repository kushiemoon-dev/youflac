package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// FFmpeg muxing operations

// MuxOptions configures the muxing operation
type MuxOptions struct {
	VideoCodec   string            `json:"videoCodec"`   // "copy" for stream copy
	AudioCodec   string            `json:"audioCodec"`   // "copy" for FLAC passthrough
	Metadata     map[string]string `json:"metadata"`
	CoverArtPath string            `json:"coverArtPath,omitempty"`
	Chapters     []Chapter         `json:"chapters,omitempty"`
	Overwrite    bool              `json:"overwrite"` // Overwrite output if exists
}

// Chapter represents a chapter marker
type Chapter struct {
	Title     string  `json:"title"`
	StartTime float64 `json:"startTime"`
	EndTime   float64 `json:"endTime"`
}

// MediaInfo contains media file information from ffprobe
type MediaInfo struct {
	Duration    float64     `json:"duration"`
	VideoCodec  string      `json:"videoCodec"`
	AudioCodec  string      `json:"audioCodec"`
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	Bitrate     int64       `json:"bitrate"`
	FrameRate   float64     `json:"frameRate"`
	SampleRate  int         `json:"sampleRate"`
	Channels    int         `json:"channels"`
	Format      string      `json:"format"`
	HasVideo    bool        `json:"hasVideo"`
	HasAudio    bool        `json:"hasAudio"`
	VideoStream *StreamInfo `json:"videoStream,omitempty"`
	AudioStream *StreamInfo `json:"audioStream,omitempty"`
}

// StreamInfo contains detailed stream information
type StreamInfo struct {
	Index      int     `json:"index"`
	CodecName  string  `json:"codecName"`
	CodecLong  string  `json:"codecLong"`
	Profile    string  `json:"profile,omitempty"`
	BitRate    int64   `json:"bitRate,omitempty"`
	Duration   float64 `json:"duration,omitempty"`
	Width      int     `json:"width,omitempty"`
	Height     int     `json:"height,omitempty"`
	FrameRate  float64 `json:"frameRate,omitempty"`
	SampleRate int     `json:"sampleRate,omitempty"`
	Channels   int     `json:"channels,omitempty"`
}

// MuxResult contains the result of a muxing operation
type MuxResult struct {
	OutputPath  string        `json:"outputPath"`
	Duration    float64       `json:"duration"`
	FileSize    int64         `json:"fileSize"`
	VideoCodec  string        `json:"videoCodec"`
	AudioCodec  string        `json:"audioCodec"`
	ElapsedTime time.Duration `json:"elapsedTime"`
	HasCoverArt bool          `json:"hasCoverArt"`
	HasMetadata bool          `json:"hasMetadata"`
	HasChapters bool          `json:"hasChapters"`
}

// ProgressCallback is called during muxing with progress updates
type ProgressCallback func(percent float64, stage string)

// MuxError represents an FFmpeg error with details
type MuxError struct {
	Command string
	Args    []string
	Stderr  string
	Err     error
}

func (e *MuxError) Error() string {
	return fmt.Sprintf("ffmpeg error: %v\nstderr: %s", e.Err, e.Stderr)
}

// DefaultMuxOptions returns sensible defaults
func DefaultMuxOptions() MuxOptions {
	return MuxOptions{
		VideoCodec: "copy",
		AudioCodec: "copy",
		Metadata:   make(map[string]string),
		Overwrite:  true,
	}
}

// MuxVideoAudio combines video and audio into MKV without re-encoding
func MuxVideoAudio(videoPath, audioPath, outputPath string, opts MuxOptions) error {
	return MuxVideoAudioWithProgress(videoPath, audioPath, outputPath, opts, nil)
}

// DetectLeadingSilence returns the duration of silence at the very start of an audio file.
// Returns 0 if no leading silence is detected or if detection fails.
func DetectLeadingSilence(filePath string) (float64, error) {
	ffmpegPath := GetFFmpegPath()
	args := []string{
		"-i", filePath,
		"-af", "silencedetect=noise=-50dB:d=0.05",
		"-f", "null",
		"-",
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Run() // Ignore exit code; silencedetect output is in stderr

	output := stderr.String()

	silenceStartRe := regexp.MustCompile(`silence_start: ([\d.]+)`)
	silenceEndRe := regexp.MustCompile(`silence_end: ([\d.]+)`)

	startMatches := silenceStartRe.FindAllStringSubmatch(output, -1)
	endMatches := silenceEndRe.FindAllStringSubmatch(output, -1)

	if len(startMatches) > 0 && len(endMatches) > 0 {
		firstStart, err1 := strconv.ParseFloat(startMatches[0][1], 64)
		firstEnd, err2 := strconv.ParseFloat(endMatches[0][1], 64)
		// Only trim if silence begins at the very start of the file
		if err1 == nil && err2 == nil && firstStart < 0.01 {
			return firstEnd, nil
		}
	}

	return 0, nil
}

// TrimAudioStart removes the first `duration` seconds from an audio file using
// sample-accurate audio filters. Output is re-encoded to FLAC (lossless).
func TrimAudioStart(inputPath, outputPath string, duration float64) error {
	ffmpegPath := GetFFmpegPath()
	args := []string{
		"-y",
		"-i", inputPath,
		"-af", fmt.Sprintf("atrim=start=%.6f,asetpts=PTS-STARTPTS", duration),
		"-c:a", "flac",
		"-compression_level", "5",
		outputPath,
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("audio trim failed: %v - %s", err, stderr.String())
	}
	return nil
}

// MuxVideoAudioWithProgress combines video and audio with progress callback
func MuxVideoAudioWithProgress(videoPath, audioPath, outputPath string, opts MuxOptions, progress ProgressCallback) error {
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("video file not found: %s", videoPath)
	}
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return fmt.Errorf("audio file not found: %s", audioPath)
	}

	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if progress != nil {
		progress(0, "Preparing mux")
	}

	// Detect and trim leading silence in audio to keep A/V in sync.
	// Tidal/Qobuz FLACs often have a short digital silence at the start
	// that YouTube's audio track doesn't, causing audible drift.
	const minSilenceTrimSec = 0.1 // ignore silences shorter than 100 ms
	effectiveAudioPath := audioPath
	if leadingSilence, err := DetectLeadingSilence(audioPath); err == nil && leadingSilence > minSilenceTrimSec {
		trimPath := audioPath + ".sync_trimmed.flac"
		if trimErr := TrimAudioStart(audioPath, trimPath, leadingSilence); trimErr == nil {
			slog.Info("trimmed leading silence for A/V sync", "duration_sec", leadingSilence, "path", trimPath)
			defer os.Remove(trimPath)
			effectiveAudioPath = trimPath
		} else {
			slog.Warn("could not trim leading silence, sync may be off", "err", trimErr)
		}
	}

	ffmpegPath := GetFFmpegPath()
	args := []string{}

	if opts.Overwrite {
		args = append(args, "-y")
	} else {
		args = append(args, "-n")
	}

	args = append(args, "-i", videoPath)
	args = append(args, "-i", effectiveAudioPath)

	hasCover := opts.CoverArtPath != "" && fileExists(opts.CoverArtPath)
	if hasCover {
		args = append(args, "-i", opts.CoverArtPath)
	}

	args = append(args, "-map", "0:v:0")
	args = append(args, "-map", "1:a:0")

	if hasCover {
		args = append(args, "-map", "2:0")
	}

	videoCodec := opts.VideoCodec
	if videoCodec == "" {
		videoCodec = "copy"
	}
	audioCodec := opts.AudioCodec
	if audioCodec == "" {
		audioCodec = "copy"
	}

	args = append(args, "-c:v", videoCodec)
	args = append(args, "-c:a", audioCodec)

	if hasCover {
		args = append(args, "-c:v:1", "mjpeg")
		args = append(args, "-disposition:v:1", "attached_pic")
	}

	for key, value := range opts.Metadata {
		if value != "" {
			args = append(args, "-metadata", fmt.Sprintf("%s=%s", key, value))
		}
	}

	args = append(args, "-f", "matroska")
	args = append(args, outputPath)

	if progress != nil {
		progress(10, "Starting FFmpeg")
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &MuxError{
			Command: ffmpegPath,
			Args:    args,
			Stderr:  stderr.String(),
			Err:     err,
		}
	}

	if progress != nil {
		progress(100, "Muxing complete")
	}

	return nil
}

// MuxVideoWithFLAC is a high-level function that handles the complete muxing workflow
func MuxVideoWithFLAC(videoPath, audioPath, outputPath string, metadata *Metadata, coverPath string, progress ProgressCallback) (*MuxResult, error) {
	startTime := time.Now()

	if progress != nil {
		progress(0, "Initializing")
	}

	videoInfo, err := GetMediaInfo(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	audioInfo, err := GetMediaInfo(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio info: %w", err)
	}

	if !videoInfo.HasVideo {
		return nil, fmt.Errorf("input video file has no video stream")
	}
	if !audioInfo.HasAudio {
		return nil, fmt.Errorf("input audio file has no audio stream")
	}

	if progress != nil {
		progress(10, "Validating inputs")
	}

	metadataMap := make(map[string]string)
	if metadata != nil {
		if metadata.Title != "" {
			metadataMap["title"] = metadata.Title
		}
		if metadata.Artist != "" {
			metadataMap["artist"] = metadata.Artist
		}
		if metadata.Album != "" {
			metadataMap["album"] = metadata.Album
		}
		if metadata.Year > 0 {
			metadataMap["date"] = strconv.Itoa(metadata.Year)
		}
		if metadata.ISRC != "" {
			metadataMap["ISRC"] = metadata.ISRC
		}
	}

	opts := MuxOptions{
		VideoCodec:   "copy",
		AudioCodec:   "copy",
		Metadata:     metadataMap,
		CoverArtPath: coverPath,
		Overwrite:    true,
	}

	muxProgress := func(p float64, stage string) {
		if progress != nil {
			progress(20+(p*0.7), stage)
		}
	}

	if err := MuxVideoAudioWithProgress(videoPath, audioPath, outputPath, opts, muxProgress); err != nil {
		return nil, err
	}

	if progress != nil {
		progress(95, "Finalizing")
	}

	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to verify output: %w", err)
	}

	outputMediaInfo, err := GetMediaInfo(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get output info: %w", err)
	}

	if progress != nil {
		progress(100, "Complete")
	}

	return &MuxResult{
		OutputPath:  outputPath,
		Duration:    outputMediaInfo.Duration,
		FileSize:    outputInfo.Size(),
		VideoCodec:  outputMediaInfo.VideoCodec,
		AudioCodec:  outputMediaInfo.AudioCodec,
		ElapsedTime: time.Since(startTime),
		HasCoverArt: coverPath != "" && fileExists(coverPath),
		HasMetadata: len(metadataMap) > 0,
		HasChapters: false,
	}, nil
}

// CreateFLACWithMetadata creates a FLAC file with embedded metadata and optional cover art.
// Used for audio-only fallback when video is unavailable.
func CreateFLACWithMetadata(audioPath, outputPath string, metadata *Metadata, coverPath string) (*MuxResult, error) {
	startTime := time.Now()

	audioInfo, err := GetMediaInfo(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio info: %w", err)
	}

	if !audioInfo.HasAudio {
		return nil, fmt.Errorf("input file has no audio stream")
	}

	ffmpegPath := GetFFmpegPath()
	args := []string{"-y"}
	args = append(args, "-i", audioPath)

	hasCover := coverPath != "" && fileExists(coverPath)
	if hasCover {
		args = append(args, "-i", coverPath)
	}

	args = append(args, "-map", "0:a")
	if hasCover {
		args = append(args, "-map", "1:0")
	}

	if strings.EqualFold(audioInfo.AudioCodec, "flac") {
		args = append(args, "-c:a", "copy")
	} else {
		args = append(args, "-c:a", "flac", "-compression_level", "8")
	}

	if hasCover {
		args = append(args, "-c:v", "mjpeg", "-disposition:v", "attached_pic")
	}

	if metadata != nil {
		if metadata.Title != "" {
			args = append(args, "-metadata", fmt.Sprintf("TITLE=%s", metadata.Title))
		}
		if metadata.Artist != "" {
			args = append(args, "-metadata", fmt.Sprintf("ARTIST=%s", metadata.Artist))
		}
		if metadata.Album != "" {
			args = append(args, "-metadata", fmt.Sprintf("ALBUM=%s", metadata.Album))
		}
		if metadata.Year > 0 {
			args = append(args, "-metadata", fmt.Sprintf("DATE=%d", metadata.Year))
		}
		if metadata.ISRC != "" {
			args = append(args, "-metadata", fmt.Sprintf("ISRC=%s", metadata.ISRC))
		}
	}

	args = append(args, outputPath)

	slog.Debug("creating FLAC", "args", strings.Join(args, " "))

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %v - %s", err, stderr.String())
	}

	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to verify output: %w", err)
	}

	outputMediaInfo, err := GetMediaInfo(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get output info: %w", err)
	}

	return &MuxResult{
		OutputPath:  outputPath,
		Duration:    outputMediaInfo.Duration,
		FileSize:    outputInfo.Size(),
		AudioCodec:  outputMediaInfo.AudioCodec,
		ElapsedTime: time.Since(startTime),
		HasCoverArt: hasCover,
		HasMetadata: metadata != nil,
	}, nil
}

// GetMediaInfo extracts media information using ffprobe
func GetMediaInfo(filePath string) (*MediaInfo, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	ffprobePath := GetFFprobePath()

	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	}

	cmd := exec.Command(ffprobePath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed: %v - %s", err, stderr.String())
	}

	var probeData struct {
		Streams []struct {
			Index         int    `json:"index"`
			CodecName     string `json:"codec_name"`
			CodecLongName string `json:"codec_long_name"`
			CodecType     string `json:"codec_type"`
			Profile       string `json:"profile"`
			Width         int    `json:"width"`
			Height        int    `json:"height"`
			SampleRate    string `json:"sample_rate"`
			Channels      int    `json:"channels"`
			BitRate       string `json:"bit_rate"`
			Duration      string `json:"duration"`
			RFrameRate    string `json:"r_frame_rate"`
			AvgFrameRate  string `json:"avg_frame_rate"`
		} `json:"streams"`
		Format struct {
			Filename   string `json:"filename"`
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
			BitRate    string `json:"bit_rate"`
			Size       string `json:"size"`
		} `json:"format"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &probeData); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	info := &MediaInfo{
		Format: probeData.Format.FormatName,
	}

	if d, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
		info.Duration = d
	}
	if br, err := strconv.ParseInt(probeData.Format.BitRate, 10, 64); err == nil {
		info.Bitrate = br
	}

	for _, stream := range probeData.Streams {
		switch stream.CodecType {
		case "video":
			info.HasVideo = true
			info.VideoCodec = stream.CodecName
			info.Width = stream.Width
			info.Height = stream.Height
			info.FrameRate = parseFrameRate(stream.AvgFrameRate)

			info.VideoStream = &StreamInfo{
				Index:     stream.Index,
				CodecName: stream.CodecName,
				CodecLong: stream.CodecLongName,
				Profile:   stream.Profile,
				Width:     stream.Width,
				Height:    stream.Height,
				FrameRate: info.FrameRate,
			}
			if br, err := strconv.ParseInt(stream.BitRate, 10, 64); err == nil {
				info.VideoStream.BitRate = br
			}
			if d, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
				info.VideoStream.Duration = d
			}

		case "audio":
			info.HasAudio = true
			info.AudioCodec = stream.CodecName
			if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
				info.SampleRate = sr
			}
			info.Channels = stream.Channels

			info.AudioStream = &StreamInfo{
				Index:      stream.Index,
				CodecName:  stream.CodecName,
				CodecLong:  stream.CodecLongName,
				Profile:    stream.Profile,
				SampleRate: info.SampleRate,
				Channels:   stream.Channels,
			}
			if br, err := strconv.ParseInt(stream.BitRate, 10, 64); err == nil {
				info.AudioStream.BitRate = br
			}
			if d, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
				info.AudioStream.Duration = d
			}
		}
	}

	return info, nil
}

// ExtractAudioStream extracts audio from a video file
func ExtractAudioStream(videoPath, outputPath string) error {
	args := []string{
		"-y",
		"-i", videoPath,
		"-vn",
		"-c:a", "copy",
		outputPath,
	}

	cmd := exec.Command(GetFFmpegPath(), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("audio extraction failed: %v - %s", err, stderr.String())
	}

	return nil
}

// ExtractVideoStream extracts video (no audio) from a file
func ExtractVideoStream(videoPath, outputPath string) error {
	args := []string{
		"-y",
		"-i", videoPath,
		"-an",
		"-c:v", "copy",
		outputPath,
	}

	cmd := exec.Command(GetFFmpegPath(), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("video extraction failed: %v - %s", err, stderr.String())
	}

	return nil
}

// ConvertToMKV converts any video format to MKV (stream copy)
func ConvertToMKV(inputPath, outputPath string) error {
	args := []string{
		"-y",
		"-i", inputPath,
		"-c", "copy",
		"-f", "matroska",
		outputPath,
	}

	cmd := exec.Command(GetFFmpegPath(), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkv conversion failed: %v - %s", err, stderr.String())
	}

	return nil
}

// DownloadThumbnail downloads thumbnail from URL to local file
func DownloadThumbnail(url, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{
		"-y",
		"-i", url,
		"-vframes", "1",
		"-f", "image2",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, GetFFmpegPath(), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("thumbnail download failed: %v - %s", err, stderr.String())
	}

	return nil
}

// GetFFmpegPath returns path to FFmpeg binary
func GetFFmpegPath() string {
	bundledPaths := []string{
		filepath.Join(getAppDataDir(), "bin", "ffmpeg"),
		filepath.Join(getAppDataDir(), "bin", "ffmpeg.exe"),
	}

	for _, p := range bundledPaths {
		if fileExists(p) {
			return p
		}
	}

	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path
	}

	return "ffmpeg"
}

// GetFFprobePath returns path to FFprobe binary
func GetFFprobePath() string {
	bundledPaths := []string{
		filepath.Join(getAppDataDir(), "bin", "ffprobe"),
		filepath.Join(getAppDataDir(), "bin", "ffprobe.exe"),
	}

	for _, p := range bundledPaths {
		if fileExists(p) {
			return p
		}
	}

	if path, err := exec.LookPath("ffprobe"); err == nil {
		return path
	}

	return "ffprobe"
}

// CheckFFmpegInstalled verifies FFmpeg is available
func CheckFFmpegInstalled() error {
	cmd := exec.Command(GetFFmpegPath(), "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FFmpeg not found or not executable: %w", err)
	}
	return nil
}

// CheckFFprobeInstalled verifies FFprobe is available
func CheckFFprobeInstalled() error {
	cmd := exec.Command(GetFFprobePath(), "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FFprobe not found or not executable: %w", err)
	}
	return nil
}

// GetFFmpegVersion returns FFmpeg version string
func GetFFmpegVersion() (string, error) {
	cmd := exec.Command(GetFFmpegPath(), "-version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return "", err
	}

	output := stdout.String()
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0]), nil
	}

	return "unknown", nil
}

// Helper functions

func parseFrameRate(fpsStr string) float64 {
	parts := strings.Split(fpsStr, "/")
	if len(parts) != 2 {
		return 0
	}

	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}

	return num / den
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func getAppDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".youflac")
}

// FormatDuration formats seconds into HH:MM:SS
func FormatDuration(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// FormatFileSize formats bytes into human readable format
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ValidateOutputPath ensures output path is valid and writable
func ValidateOutputPath(outputPath string) error {
	dir := filepath.Dir(outputPath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}

	testFile := filepath.Join(dir, ".youflac-test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("cannot write to output directory: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// ReadProgressFromStderr parses FFmpeg progress from stderr
func ReadProgressFromStderr(stderr io.Reader, totalDuration float64, callback ProgressCallback) {
	timeRegex := regexp.MustCompile(`time=(\d+):(\d+):(\d+)\.(\d+)`)

	buf := make([]byte, 1024)
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			text := string(buf[:n])
			matches := timeRegex.FindStringSubmatch(text)
			if len(matches) == 5 {
				hours, _ := strconv.Atoi(matches[1])
				mins, _ := strconv.Atoi(matches[2])
				secs, _ := strconv.Atoi(matches[3])
				currentTime := float64(hours*3600+mins*60+secs)

				if totalDuration > 0 && callback != nil {
					percent := (currentTime / totalDuration) * 100
					if percent > 100 {
						percent = 100
					}
					callback(percent, "Processing")
				}
			}
		}
		if err != nil {
			break
		}
	}
}
