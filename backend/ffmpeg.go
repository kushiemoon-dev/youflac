package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	OutputPath   string        `json:"outputPath"`
	Duration     float64       `json:"duration"`
	FileSize     int64         `json:"fileSize"`
	VideoCodec   string        `json:"videoCodec"`
	AudioCodec   string        `json:"audioCodec"`
	ElapsedTime  time.Duration `json:"elapsedTime"`
	HasCoverArt  bool          `json:"hasCoverArt"`
	HasMetadata  bool          `json:"hasMetadata"`
	HasChapters  bool          `json:"hasChapters"`
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

// MuxVideoAudioWithProgress combines video and audio with progress callback
func MuxVideoAudioWithProgress(videoPath, audioPath, outputPath string, opts MuxOptions, progress ProgressCallback) error {
	// Validate input files exist
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("video file not found: %s", videoPath)
	}
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return fmt.Errorf("audio file not found: %s", audioPath)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if progress != nil {
		progress(0, "Preparing mux")
	}

	// Build FFmpeg arguments
	ffmpegPath := GetFFmpegPath()
	args := []string{}

	// Overwrite flag
	if opts.Overwrite {
		args = append(args, "-y")
	} else {
		args = append(args, "-n")
	}

	// Input files
	args = append(args, "-i", videoPath)
	args = append(args, "-i", audioPath)

	// Add cover art as third input if provided
	hasCover := opts.CoverArtPath != "" && fileExists(opts.CoverArtPath)
	if hasCover {
		args = append(args, "-i", opts.CoverArtPath)
	}

	// Map streams
	args = append(args, "-map", "0:v:0") // First video stream from first input
	args = append(args, "-map", "1:a:0") // First audio stream from second input

	if hasCover {
		args = append(args, "-map", "2:0") // Cover art as attachment
	}

	// Codec settings - stream copy (no re-encoding)
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

	// Cover art settings
	if hasCover {
		args = append(args, "-c:v:1", "mjpeg") // Cover as MJPEG
		args = append(args, "-disposition:v:1", "attached_pic")
	}

	// Add metadata
	for key, value := range opts.Metadata {
		if value != "" {
			args = append(args, "-metadata", fmt.Sprintf("%s=%s", key, value))
		}
	}

	// MKV-specific options
	args = append(args, "-f", "matroska") // Force MKV format

	// Output file
	args = append(args, outputPath)

	if progress != nil {
		progress(10, "Starting FFmpeg")
	}

	// Execute FFmpeg
	cmd := exec.Command(ffmpegPath, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
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

	// Get input media info for validation
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

	// Build metadata map
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

	// Build mux options
	opts := MuxOptions{
		VideoCodec:   "copy",
		AudioCodec:   "copy",
		Metadata:     metadataMap,
		CoverArtPath: coverPath,
		Overwrite:    true,
	}

	// Execute muxing with progress wrapper
	muxProgress := func(p float64, stage string) {
		if progress != nil {
			// Scale to 20-90% range
			scaled := 20 + (p * 0.7)
			progress(scaled, stage)
		}
	}

	if err := MuxVideoAudioWithProgress(videoPath, audioPath, outputPath, opts, muxProgress); err != nil {
		return nil, err
	}

	if progress != nil {
		progress(95, "Finalizing")
	}

	// Get output file info
	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to verify output: %w", err)
	}

	// Get output media info
	outputMediaInfo, err := GetMediaInfo(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get output info: %w", err)
	}

	if progress != nil {
		progress(100, "Complete")
	}

	return &MuxResult{
		OutputPath:   outputPath,
		Duration:     outputMediaInfo.Duration,
		FileSize:     outputInfo.Size(),
		VideoCodec:   outputMediaInfo.VideoCodec,
		AudioCodec:   outputMediaInfo.AudioCodec,
		ElapsedTime:  time.Since(startTime),
		HasCoverArt:  coverPath != "" && fileExists(coverPath),
		HasMetadata:  len(metadataMap) > 0,
		HasChapters:  false,
	}, nil
}

// CreateFLACWithMetadata creates a FLAC file with embedded metadata and optional cover art
// Used for audio-only fallback when video is unavailable
func CreateFLACWithMetadata(audioPath, outputPath string, metadata *Metadata, coverPath string) (*MuxResult, error) {
	startTime := time.Now()

	// Get audio info for validation
	audioInfo, err := GetMediaInfo(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio info: %w", err)
	}

	if !audioInfo.HasAudio {
		return nil, fmt.Errorf("input file has no audio stream")
	}

	ffmpegPath := GetFFmpegPath()

	// Build ffmpeg command
	args := []string{"-y"} // Overwrite output

	// Input audio file
	args = append(args, "-i", audioPath)

	// Input cover art if available
	hasCover := coverPath != "" && fileExists(coverPath)
	if hasCover {
		args = append(args, "-i", coverPath)
	}

	// Map audio stream
	args = append(args, "-map", "0:a")

	// Map cover art if available
	if hasCover {
		args = append(args, "-map", "1:0")
	}

	// Audio codec: convert to FLAC if not already, otherwise copy
	isAlreadyFLAC := strings.EqualFold(audioInfo.AudioCodec, "flac")
	if isAlreadyFLAC {
		args = append(args, "-c:a", "copy")
	} else {
		// Convert to FLAC with high quality
		args = append(args, "-c:a", "flac")
		args = append(args, "-compression_level", "8")
	}

	// Cover art codec (MJPEG for embedded pictures)
	if hasCover {
		args = append(args, "-c:v", "mjpeg")
		args = append(args, "-disposition:v", "attached_pic")
	}

	// Add metadata
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

	// Output file
	args = append(args, outputPath)

	fmt.Printf("[FFmpeg] Creating FLAC: %s\n", strings.Join(args, " "))

	// Execute ffmpeg
	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %v - %s", err, stderr.String())
	}

	// Get output file info
	outputInfo, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to verify output: %w", err)
	}

	// Get output media info
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

	// Parse JSON output
	var probeData struct {
		Streams []struct {
			Index         int     `json:"index"`
			CodecName     string  `json:"codec_name"`
			CodecLongName string  `json:"codec_long_name"`
			CodecType     string  `json:"codec_type"`
			Profile       string  `json:"profile"`
			Width         int     `json:"width"`
			Height        int     `json:"height"`
			SampleRate    string  `json:"sample_rate"`
			Channels      int     `json:"channels"`
			BitRate       string  `json:"bit_rate"`
			Duration      string  `json:"duration"`
			RFrameRate    string  `json:"r_frame_rate"`
			AvgFrameRate  string  `json:"avg_frame_rate"`
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

	// Parse duration
	if d, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
		info.Duration = d
	}

	// Parse bitrate
	if br, err := strconv.ParseInt(probeData.Format.BitRate, 10, 64); err == nil {
		info.Bitrate = br
	}

	// Process streams
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

// EmbedMetadata adds/updates metadata in existing MKV using mkvpropedit or ffmpeg
func EmbedMetadata(mkvPath string, metadata map[string]string) error {
	if _, err := os.Stat(mkvPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", mkvPath)
	}

	// Try mkvpropedit first (more reliable for MKV)
	mkvpropeditPath, err := exec.LookPath("mkvpropedit")
	if err == nil {
		return embedMetadataMkvpropedit(mkvPath, metadata, mkvpropeditPath)
	}

	// Fall back to ffmpeg (requires re-mux)
	return embedMetadataFFmpeg(mkvPath, metadata)
}

func embedMetadataMkvpropedit(mkvPath string, metadata map[string]string, mkvpropeditPath string) error {
	// For full metadata, use --edit info
	args := []string{mkvPath, "--edit", "info"}
	for key, value := range metadata {
		if value != "" {
			switch strings.ToLower(key) {
			case "title":
				args = append(args, "--set", fmt.Sprintf("title=%s", value))
			case "artist":
				// MKV doesn't have a standard artist field in segment info
				// We'll skip it here as it's better handled via ffmpeg or tags
			case "album":
				// Same as artist
			}
		}
	}

	cmd := exec.Command(mkvpropeditPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkvpropedit failed: %v - %s", err, stderr.String())
	}

	return nil
}

func embedMetadataFFmpeg(mkvPath string, metadata map[string]string) error {
	// FFmpeg requires re-muxing to change metadata
	tempPath := mkvPath + ".tmp"

	args := []string{
		"-y",
		"-i", mkvPath,
		"-c", "copy",
	}

	for key, value := range metadata {
		if value != "" {
			args = append(args, "-metadata", fmt.Sprintf("%s=%s", key, value))
		}
	}

	args = append(args, tempPath)

	cmd := exec.Command(GetFFmpegPath(), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("ffmpeg metadata failed: %v - %s", err, stderr.String())
	}

	// Replace original with temp
	if err := os.Rename(tempPath, mkvPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace file: %w", err)
	}

	return nil
}

// EmbedCoverArt adds cover art to existing MKV
func EmbedCoverArt(mkvPath, coverPath string) error {
	if _, err := os.Stat(mkvPath); os.IsNotExist(err) {
		return fmt.Errorf("mkv file not found: %s", mkvPath)
	}
	if _, err := os.Stat(coverPath); os.IsNotExist(err) {
		return fmt.Errorf("cover file not found: %s", coverPath)
	}

	// Try mkvpropedit first
	mkvpropeditPath, err := exec.LookPath("mkvpropedit")
	if err == nil {
		return embedCoverMkvpropedit(mkvPath, coverPath, mkvpropeditPath)
	}

	// Fall back to ffmpeg
	return embedCoverFFmpeg(mkvPath, coverPath)
}

func embedCoverMkvpropedit(mkvPath, coverPath, mkvpropeditPath string) error {
	// mkvpropedit can add attachments
	args := []string{
		mkvPath,
		"--add-attachment", coverPath,
	}

	cmd := exec.Command(mkvpropeditPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkvpropedit cover failed: %v - %s", err, stderr.String())
	}

	return nil
}

func embedCoverFFmpeg(mkvPath, coverPath string) error {
	tempPath := mkvPath + ".tmp"

	args := []string{
		"-y",
		"-i", mkvPath,
		"-i", coverPath,
		"-map", "0",
		"-map", "1:0",
		"-c", "copy",
		"-c:v:1", "mjpeg",
		"-disposition:v:1", "attached_pic",
		tempPath,
	}

	cmd := exec.Command(GetFFmpegPath(), args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("ffmpeg cover failed: %v - %s", err, stderr.String())
	}

	if err := os.Rename(tempPath, mkvPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace file: %w", err)
	}

	return nil
}

// AddChapters adds chapter markers to MKV
func AddChapters(mkvPath string, chapters []Chapter) error {
	if len(chapters) == 0 {
		return nil
	}

	// Create chapters file in WebVTT-like format for mkvpropedit
	chaptersFile, err := os.CreateTemp("", "chapters-*.xml")
	if err != nil {
		return fmt.Errorf("failed to create chapters file: %w", err)
	}
	defer os.Remove(chaptersFile.Name())

	// Write XML chapters format
	var xml strings.Builder
	xml.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE Chapters SYSTEM "matroskachapters.dtd">
<Chapters>
  <EditionEntry>
`)
	for i, ch := range chapters {
		startNs := int64(ch.StartTime * 1e9)
		endNs := int64(ch.EndTime * 1e9)
		xml.WriteString(fmt.Sprintf(`    <ChapterAtom>
      <ChapterUID>%d</ChapterUID>
      <ChapterTimeStart>%d</ChapterTimeStart>
      <ChapterTimeEnd>%d</ChapterTimeEnd>
      <ChapterDisplay>
        <ChapterString>%s</ChapterString>
        <ChapterLanguage>eng</ChapterLanguage>
      </ChapterDisplay>
    </ChapterAtom>
`, i+1, startNs, endNs, ch.Title))
	}
	xml.WriteString(`  </EditionEntry>
</Chapters>
`)

	if _, err := chaptersFile.WriteString(xml.String()); err != nil {
		return err
	}
	chaptersFile.Close()

	// Try mkvpropedit
	mkvpropeditPath, err := exec.LookPath("mkvpropedit")
	if err != nil {
		return fmt.Errorf("mkvpropedit not found, chapters require mkvtoolnix")
	}

	args := []string{
		mkvPath,
		"--chapters", chaptersFile.Name(),
	}

	cmd := exec.Command(mkvpropeditPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkvpropedit chapters failed: %v - %s", err, stderr.String())
	}

	return nil
}

// ExtractAudioStream extracts audio from a video file
func ExtractAudioStream(videoPath, outputPath string) error {
	args := []string{
		"-y",
		"-i", videoPath,
		"-vn",       // No video
		"-c:a", "copy", // Copy audio codec
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
		"-an",       // No audio
		"-c:v", "copy", // Copy video codec
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

	// Use ffmpeg to download and convert to JPEG
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
	// Check bundled binary first
	bundledPaths := []string{
		filepath.Join(getAppDataDir(), "bin", "ffmpeg"),
		filepath.Join(getAppDataDir(), "bin", "ffmpeg.exe"),
	}

	for _, p := range bundledPaths {
		if fileExists(p) {
			return p
		}
	}

	// Check system PATH
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path
	}

	// Fallback
	return "ffmpeg"
}

// GetFFprobePath returns path to FFprobe binary
func GetFFprobePath() string {
	// Check bundled binary first
	bundledPaths := []string{
		filepath.Join(getAppDataDir(), "bin", "ffprobe"),
		filepath.Join(getAppDataDir(), "bin", "ffprobe.exe"),
	}

	for _, p := range bundledPaths {
		if fileExists(p) {
			return p
		}
	}

	// Check system PATH
	if path, err := exec.LookPath("ffprobe"); err == nil {
		return path
	}

	return "ffprobe"
}

// CheckFFmpegInstalled verifies FFmpeg is available
func CheckFFmpegInstalled() error {
	ffmpegPath := GetFFmpegPath()
	cmd := exec.Command(ffmpegPath, "-version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FFmpeg not found or not executable: %w", err)
	}
	return nil
}

// CheckFFprobeInstalled verifies FFprobe is available
func CheckFFprobeInstalled() error {
	ffprobePath := GetFFprobePath()
	cmd := exec.Command(ffprobePath, "-version")
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

	// Parse first line for version
	output := stdout.String()
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		// "ffmpeg version X.X.X ..."
		return strings.TrimSpace(lines[0]), nil
	}

	return "unknown", nil
}

// Helper functions

func parseFrameRate(fpsStr string) float64 {
	// Format: "30/1" or "30000/1001"
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
	// Cross-platform app data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Use ~/.youflac for all platforms
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

	// Check if directory exists or can be created
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}

	// Check if we can write to the directory
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
// This is useful for real-time progress tracking
func ReadProgressFromStderr(stderr io.Reader, totalDuration float64, callback ProgressCallback) {
	// FFmpeg outputs progress like: "time=00:01:23.45"
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
