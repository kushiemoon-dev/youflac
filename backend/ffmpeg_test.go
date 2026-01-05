package backend

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetFFmpegPath(t *testing.T) {
	path := GetFFmpegPath()
	if path == "" {
		t.Error("GetFFmpegPath returned empty string")
	}
	t.Logf("FFmpeg path: %s", path)
}

func TestGetFFprobePath(t *testing.T) {
	path := GetFFprobePath()
	if path == "" {
		t.Error("GetFFprobePath returned empty string")
	}
	t.Logf("FFprobe path: %s", path)
}

func TestCheckFFmpegInstalled(t *testing.T) {
	err := CheckFFmpegInstalled()
	if err != nil {
		t.Skipf("FFmpeg not installed: %v", err)
	}
	t.Log("FFmpeg is installed and working")
}

func TestCheckFFprobeInstalled(t *testing.T) {
	err := CheckFFprobeInstalled()
	if err != nil {
		t.Skipf("FFprobe not installed: %v", err)
	}
	t.Log("FFprobe is installed and working")
}

func TestGetFFmpegVersion(t *testing.T) {
	if err := CheckFFmpegInstalled(); err != nil {
		t.Skip("FFmpeg not installed")
	}

	version, err := GetFFmpegVersion()
	if err != nil {
		t.Fatalf("GetFFmpegVersion failed: %v", err)
	}

	if version == "" || version == "unknown" {
		t.Error("Failed to get FFmpeg version")
	}

	t.Logf("FFmpeg version: %s", version)
}

func TestDefaultMuxOptions(t *testing.T) {
	opts := DefaultMuxOptions()

	if opts.VideoCodec != "copy" {
		t.Errorf("Expected VideoCodec 'copy', got %s", opts.VideoCodec)
	}
	if opts.AudioCodec != "copy" {
		t.Errorf("Expected AudioCodec 'copy', got %s", opts.AudioCodec)
	}
	if opts.Metadata == nil {
		t.Error("Metadata map should not be nil")
	}
	if !opts.Overwrite {
		t.Error("Overwrite should be true by default")
	}
}

func TestParseFrameRate(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"30/1", 30.0},
		{"60/1", 60.0},
		{"24000/1001", 23.976},
		{"30000/1001", 29.97},
		{"0/0", 0.0},
		{"invalid", 0.0},
		{"", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseFrameRate(tt.input)

			// Allow small epsilon for floating point comparison
			if tt.expected > 0 {
				if result < tt.expected*0.99 || result > tt.expected*1.01 {
					t.Errorf("parseFrameRate(%q) = %v, want ~%v", tt.input, result, tt.expected)
				}
			} else if result != 0 {
				t.Errorf("parseFrameRate(%q) = %v, want 0", tt.input, result)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0, "0:00"},
		{30, "0:30"},
		{60, "1:00"},
		{90, "1:30"},
		{3600, "1:00:00"},
		{3661, "1:01:01"},
		{213, "3:33"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.0fs", tt.seconds), func(t *testing.T) {
			result := FormatDuration(tt.seconds)
			if result != tt.expected {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.seconds, result, tt.expected)
			}
		})
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatFileSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatFileSize(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestValidateOutputPath(t *testing.T) {
	// Create temp directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		path      string
		shouldErr bool
	}{
		{
			name:      "valid path in temp dir",
			path:      filepath.Join(tmpDir, "test.mkv"),
			shouldErr: false,
		},
		{
			name:      "nested directory",
			path:      filepath.Join(tmpDir, "sub", "dir", "test.mkv"),
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutputPath(tt.path)
			if tt.shouldErr && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestMuxError(t *testing.T) {
	err := &MuxError{
		Command: "ffmpeg",
		Args:    []string{"-i", "input.mp4", "output.mkv"},
		Stderr:  "Error message",
		Err:     fmt.Errorf("exit status 1"),
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("MuxError.Error() returned empty string")
	}

	if !contains(errStr, "ffmpeg") {
		t.Error("Error message should contain 'ffmpeg'")
	}
	if !contains(errStr, "Error message") {
		t.Error("Error message should contain stderr")
	}
}

func TestGetMediaInfo(t *testing.T) {
	if err := CheckFFprobeInstalled(); err != nil {
		t.Skip("FFprobe not installed")
	}

	// Create a minimal test video file using ffmpeg
	tmpDir := t.TempDir()
	testVideoPath := filepath.Join(tmpDir, "test.mp4")

	// Generate test video (1 second, 320x240, with audio)
	cmd := fmt.Sprintf(
		"%s -f lavfi -i testsrc=duration=1:size=320x240:rate=30 -f lavfi -i sine=frequency=1000:duration=1 -c:v libx264 -c:a aac -y %s",
		GetFFmpegPath(),
		testVideoPath,
	)
	if err := runCommand(cmd); err != nil {
		t.Skipf("Could not create test video: %v", err)
	}

	// Test GetMediaInfo
	info, err := GetMediaInfo(testVideoPath)
	if err != nil {
		t.Fatalf("GetMediaInfo failed: %v", err)
	}

	// Verify video info
	if !info.HasVideo {
		t.Error("Expected HasVideo = true")
	}
	if !info.HasAudio {
		t.Error("Expected HasAudio = true")
	}
	if info.Width != 320 {
		t.Errorf("Expected Width = 320, got %d", info.Width)
	}
	if info.Height != 240 {
		t.Errorf("Expected Height = 240, got %d", info.Height)
	}
	if info.Duration < 0.9 || info.Duration > 1.1 {
		t.Errorf("Expected Duration ~1.0s, got %.2f", info.Duration)
	}

	t.Logf("MediaInfo: %+v", info)
}

func TestGetMediaInfo_NotFound(t *testing.T) {
	_, err := GetMediaInfo("/nonexistent/file.mp4")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestMuxVideoAudio(t *testing.T) {
	if err := CheckFFmpegInstalled(); err != nil {
		t.Skip("FFmpeg not installed")
	}

	tmpDir := t.TempDir()

	// Create test video (video only)
	videoPath := filepath.Join(tmpDir, "video.mp4")
	videoCmd := fmt.Sprintf(
		"%s -f lavfi -i testsrc=duration=2:size=640x480:rate=30 -an -c:v libx264 -y %s",
		GetFFmpegPath(),
		videoPath,
	)
	if err := runCommand(videoCmd); err != nil {
		t.Fatalf("Could not create test video: %v", err)
	}

	// Create test audio (FLAC)
	audioPath := filepath.Join(tmpDir, "audio.flac")
	audioCmd := fmt.Sprintf(
		"%s -f lavfi -i sine=frequency=440:duration=2 -c:a flac -y %s",
		GetFFmpegPath(),
		audioPath,
	)
	if err := runCommand(audioCmd); err != nil {
		t.Fatalf("Could not create test audio: %v", err)
	}

	// Mux video + audio
	outputPath := filepath.Join(tmpDir, "output.mkv")
	opts := MuxOptions{
		VideoCodec: "copy",
		AudioCodec: "copy",
		Metadata: map[string]string{
			"title":  "Test Video",
			"artist": "Test Artist",
		},
		Overwrite: true,
	}

	err := MuxVideoAudio(videoPath, audioPath, outputPath, opts)
	if err != nil {
		t.Fatalf("MuxVideoAudio failed: %v", err)
	}

	// Verify output exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output file was not created")
	}

	// Verify output has both video and audio
	info, err := GetMediaInfo(outputPath)
	if err != nil {
		t.Fatalf("Could not get output info: %v", err)
	}

	if !info.HasVideo {
		t.Error("Output should have video stream")
	}
	if !info.HasAudio {
		t.Error("Output should have audio stream")
	}
	if info.AudioCodec != "flac" {
		t.Errorf("Expected audio codec 'flac', got %s", info.AudioCodec)
	}

	t.Logf("Output MKV: %s", outputPath)
	t.Logf("Duration: %.2fs", info.Duration)
	t.Logf("Video: %s %dx%d", info.VideoCodec, info.Width, info.Height)
	t.Logf("Audio: %s %dHz %dch", info.AudioCodec, info.SampleRate, info.Channels)
}

func TestMuxVideoWithFLAC(t *testing.T) {
	if err := CheckFFmpegInstalled(); err != nil {
		t.Skip("FFmpeg not installed")
	}

	tmpDir := t.TempDir()

	// Create test video
	videoPath := filepath.Join(tmpDir, "video.mp4")
	videoCmd := fmt.Sprintf(
		"%s -f lavfi -i testsrc=duration=2:size=1280x720:rate=30 -an -c:v libx264 -y %s",
		GetFFmpegPath(),
		videoPath,
	)
	if err := runCommand(videoCmd); err != nil {
		t.Fatalf("Could not create test video: %v", err)
	}

	// Create test FLAC audio
	audioPath := filepath.Join(tmpDir, "audio.flac")
	audioCmd := fmt.Sprintf(
		"%s -f lavfi -i sine=frequency=440:duration=2 -c:a flac -y %s",
		GetFFmpegPath(),
		audioPath,
	)
	if err := runCommand(audioCmd); err != nil {
		t.Fatalf("Could not create test audio: %v", err)
	}

	// Create test cover image
	coverPath := filepath.Join(tmpDir, "cover.jpg")
	coverCmd := fmt.Sprintf(
		"%s -f lavfi -i color=red:size=500x500:duration=1 -frames:v 1 -y %s",
		GetFFmpegPath(),
		coverPath,
	)
	if err := runCommand(coverCmd); err != nil {
		t.Fatalf("Could not create test cover: %v", err)
	}

	// Test MuxVideoWithFLAC
	outputPath := filepath.Join(tmpDir, "output.mkv")
	metadata := &Metadata{
		Title:  "Never Gonna Give You Up",
		Artist: "Rick Astley",
		Album:  "Whenever You Need Somebody",
		Year:   1987,
		ISRC:   "GBARL9300135",
	}

	var progressCalls int
	progress := func(percent float64, stage string) {
		progressCalls++
		t.Logf("Progress: %.0f%% - %s", percent, stage)
	}

	result, err := MuxVideoWithFLAC(videoPath, audioPath, outputPath, metadata, coverPath, progress)
	if err != nil {
		t.Fatalf("MuxVideoWithFLAC failed: %v", err)
	}

	// Verify result
	if result.OutputPath != outputPath {
		t.Errorf("Expected OutputPath %s, got %s", outputPath, result.OutputPath)
	}
	if result.Duration < 1.9 || result.Duration > 2.1 {
		t.Errorf("Expected Duration ~2.0s, got %.2f", result.Duration)
	}
	if result.FileSize == 0 {
		t.Error("FileSize should not be 0")
	}
	if result.VideoCodec == "" {
		t.Error("VideoCodec should not be empty")
	}
	if result.AudioCodec != "flac" {
		t.Errorf("Expected AudioCodec 'flac', got %s", result.AudioCodec)
	}
	if !result.HasMetadata {
		t.Error("HasMetadata should be true")
	}
	if !result.HasCoverArt {
		t.Error("HasCoverArt should be true")
	}
	if progressCalls == 0 {
		t.Error("Progress callback should have been called")
	}

	fmt.Println()
	fmt.Println("=== MuxVideoWithFLAC Result ===")
	fmt.Printf("Output: %s\n", result.OutputPath)
	fmt.Printf("Duration: %s\n", FormatDuration(result.Duration))
	fmt.Printf("File Size: %s\n", FormatFileSize(result.FileSize))
	fmt.Printf("Video Codec: %s\n", result.VideoCodec)
	fmt.Printf("Audio Codec: %s\n", result.AudioCodec)
	fmt.Printf("Elapsed: %s\n", result.ElapsedTime)
	fmt.Printf("Has Cover: %v\n", result.HasCoverArt)
	fmt.Printf("Has Metadata: %v\n", result.HasMetadata)
}

func TestMuxVideoAudio_MissingInputs(t *testing.T) {
	tmpDir := t.TempDir()

	opts := DefaultMuxOptions()
	outputPath := filepath.Join(tmpDir, "output.mkv")

	// Test missing video
	err := MuxVideoAudio("/nonexistent/video.mp4", "/nonexistent/audio.flac", outputPath, opts)
	if err == nil {
		t.Error("Expected error for missing video file")
	}

	// Create a dummy video to test missing audio
	videoPath := filepath.Join(tmpDir, "exists.mp4")
	os.WriteFile(videoPath, []byte("dummy"), 0644)

	err = MuxVideoAudio(videoPath, "/nonexistent/audio.flac", outputPath, opts)
	if err == nil {
		t.Error("Expected error for missing audio file")
	}
}

func TestExtractAudioStream(t *testing.T) {
	if err := CheckFFmpegInstalled(); err != nil {
		t.Skip("FFmpeg not installed")
	}

	tmpDir := t.TempDir()

	// Create test video with audio
	videoPath := filepath.Join(tmpDir, "video.mp4")
	videoCmd := fmt.Sprintf(
		"%s -f lavfi -i testsrc=duration=1:size=320x240:rate=30 -f lavfi -i sine=frequency=1000:duration=1 -c:v libx264 -c:a aac -y %s",
		GetFFmpegPath(),
		videoPath,
	)
	if err := runCommand(videoCmd); err != nil {
		t.Skipf("Could not create test video: %v", err)
	}

	// Extract audio
	audioPath := filepath.Join(tmpDir, "audio.aac")
	err := ExtractAudioStream(videoPath, audioPath)
	if err != nil {
		t.Fatalf("ExtractAudioStream failed: %v", err)
	}

	// Verify audio file exists
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		t.Error("Audio file was not created")
	}

	// Verify it has audio but no video
	info, err := GetMediaInfo(audioPath)
	if err != nil {
		t.Fatalf("Could not get audio info: %v", err)
	}

	if !info.HasAudio {
		t.Error("Extracted file should have audio")
	}
	if info.HasVideo {
		t.Error("Extracted file should not have video")
	}
}

func TestExtractVideoStream(t *testing.T) {
	if err := CheckFFmpegInstalled(); err != nil {
		t.Skip("FFmpeg not installed")
	}

	tmpDir := t.TempDir()

	// Create test video with audio
	videoPath := filepath.Join(tmpDir, "video.mp4")
	videoCmd := fmt.Sprintf(
		"%s -f lavfi -i testsrc=duration=1:size=320x240:rate=30 -f lavfi -i sine=frequency=1000:duration=1 -c:v libx264 -c:a aac -y %s",
		GetFFmpegPath(),
		videoPath,
	)
	if err := runCommand(videoCmd); err != nil {
		t.Skipf("Could not create test video: %v", err)
	}

	// Extract video
	videoOnlyPath := filepath.Join(tmpDir, "video_only.mp4")
	err := ExtractVideoStream(videoPath, videoOnlyPath)
	if err != nil {
		t.Fatalf("ExtractVideoStream failed: %v", err)
	}

	// Verify video file exists
	if _, err := os.Stat(videoOnlyPath); os.IsNotExist(err) {
		t.Error("Video file was not created")
	}

	// Verify it has video but no audio
	info, err := GetMediaInfo(videoOnlyPath)
	if err != nil {
		t.Fatalf("Could not get video info: %v", err)
	}

	if !info.HasVideo {
		t.Error("Extracted file should have video")
	}
	if info.HasAudio {
		t.Error("Extracted file should not have audio")
	}
}

func TestConvertToMKV(t *testing.T) {
	if err := CheckFFmpegInstalled(); err != nil {
		t.Skip("FFmpeg not installed")
	}

	tmpDir := t.TempDir()

	// Create test MP4
	inputPath := filepath.Join(tmpDir, "input.mp4")
	cmd := fmt.Sprintf(
		"%s -f lavfi -i testsrc=duration=1:size=320x240:rate=30 -c:v libx264 -y %s",
		GetFFmpegPath(),
		inputPath,
	)
	if err := runCommand(cmd); err != nil {
		t.Skipf("Could not create test video: %v", err)
	}

	// Convert to MKV
	outputPath := filepath.Join(tmpDir, "output.mkv")
	err := ConvertToMKV(inputPath, outputPath)
	if err != nil {
		t.Fatalf("ConvertToMKV failed: %v", err)
	}

	// Verify output exists and is MKV
	info, err := GetMediaInfo(outputPath)
	if err != nil {
		t.Fatalf("Could not get output info: %v", err)
	}

	if !info.HasVideo {
		t.Error("Output should have video")
	}
	if info.Format == "" {
		t.Error("Format should not be empty")
	}

	t.Logf("Converted to MKV: format=%s", info.Format)
}

// Helper function to run shell commands
func runCommand(cmd string) error {
	parts := splitCommand(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	command := exec.Command(parts[0], parts[1:]...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, string(output))
	}
	return nil
}

func splitCommand(cmd string) []string {
	var parts []string
	var current string
	inQuotes := false

	for _, r := range cmd {
		switch {
		case r == '"' || r == '\'':
			inQuotes = !inQuotes
		case r == ' ' && !inQuotes:
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		default:
			current += string(r)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

