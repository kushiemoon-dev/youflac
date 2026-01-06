package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// AudioAnalysis contains detailed audio quality analysis
type AudioAnalysis struct {
	FilePath      string   `json:"filePath"`
	FileName      string   `json:"fileName"`
	Codec         string   `json:"codec"`
	CodecLong     string   `json:"codecLong"`
	Bitrate       int      `json:"bitrate"`       // bits per second
	SampleRate    int      `json:"sampleRate"`    // Hz
	BitsPerSample int      `json:"bitsPerSample"` // 16, 24, 32
	Channels      int      `json:"channels"`
	Duration      float64  `json:"duration"`
	FileSize      int64    `json:"fileSize"`

	// Quality analysis
	IsTrueLossless bool     `json:"isTrueLossless"`
	FakeLossless   bool     `json:"fakeLossless"`
	QualityScore   int      `json:"qualityScore"`  // 0-100
	QualityRating  string   `json:"qualityRating"` // Excellent/Good/Fair/Poor
	Issues         []string `json:"issues,omitempty"`

	// Spectrogram
	SpectrogramPath string `json:"spectrogramPath,omitempty"`

	// Additional metadata
	Format      string `json:"format"`
	Profile     string `json:"profile,omitempty"`
	MaxFreq     int    `json:"maxFreq,omitempty"` // Estimated max frequency content
}

// AnalyzeAudio performs a comprehensive audio quality analysis
func AnalyzeAudio(filePath string) (*AudioAnalysis, error) {
	// Check file exists
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	analysis := &AudioAnalysis{
		FilePath: filePath,
		FileName: filepath.Base(filePath),
		FileSize: stat.Size(),
	}

	// Get detailed audio info via ffprobe
	if err := analysis.fetchProbeData(); err != nil {
		return nil, fmt.Errorf("failed to probe file: %w", err)
	}

	// Analyze quality
	analysis.analyzeQuality()

	// Detect fake lossless
	analysis.detectFakeLossless()

	// Calculate quality score
	analysis.calculateQualityScore()

	return analysis, nil
}

// fetchProbeData uses ffprobe to extract detailed audio information
func (a *AudioAnalysis) fetchProbeData() error {
	ffprobePath := GetFFprobePath()

	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-select_streams", "a:0", // First audio stream
		a.FilePath,
	}

	cmd := exec.Command(ffprobePath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffprobe failed: %v - %s", err, stderr.String())
	}

	var probeData struct {
		Streams []struct {
			CodecName     string `json:"codec_name"`
			CodecLongName string `json:"codec_long_name"`
			Profile       string `json:"profile"`
			SampleRate    string `json:"sample_rate"`
			Channels      int    `json:"channels"`
			BitsPerSample int    `json:"bits_per_raw_sample"`
			BitRate       string `json:"bit_rate"`
			Duration      string `json:"duration"`
		} `json:"streams"`
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
			BitRate    string `json:"bit_rate"`
		} `json:"format"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &probeData); err != nil {
		return fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	if len(probeData.Streams) == 0 {
		return fmt.Errorf("no audio stream found")
	}

	stream := probeData.Streams[0]

	a.Codec = stream.CodecName
	a.CodecLong = stream.CodecLongName
	a.Profile = stream.Profile
	a.Format = probeData.Format.FormatName
	a.Channels = stream.Channels
	a.BitsPerSample = stream.BitsPerSample

	// Parse sample rate
	if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
		a.SampleRate = sr
	}

	// Parse bitrate (stream or format level)
	if br, err := strconv.Atoi(stream.BitRate); err == nil {
		a.Bitrate = br
	} else if br, err := strconv.Atoi(probeData.Format.BitRate); err == nil {
		a.Bitrate = br
	}

	// Parse duration
	if d, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
		a.Duration = d
	} else if d, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
		a.Duration = d
	}

	// For FLAC, bits_per_raw_sample might be 0, try to detect from codec
	if a.BitsPerSample == 0 && strings.Contains(strings.ToLower(a.Codec), "flac") {
		// FLAC files typically are 16 or 24 bit
		// Check file size vs duration to estimate
		if a.Duration > 0 && a.FileSize > 0 {
			bps := float64(a.FileSize*8) / a.Duration / float64(a.Channels) / float64(a.SampleRate)
			if bps > 20 {
				a.BitsPerSample = 24
			} else {
				a.BitsPerSample = 16
			}
		} else {
			a.BitsPerSample = 16 // Default assumption
		}
	}

	return nil
}

// analyzeQuality checks for common quality issues
func (a *AudioAnalysis) analyzeQuality() {
	a.Issues = []string{}

	// Check codec
	codecLower := strings.ToLower(a.Codec)
	isLossless := codecLower == "flac" || codecLower == "alac" || codecLower == "wav" || codecLower == "pcm_s16le" || codecLower == "pcm_s24le"

	if !isLossless {
		a.Issues = append(a.Issues, fmt.Sprintf("Lossy codec detected: %s", a.Codec))
	} else {
		a.IsTrueLossless = true // Will be refined by detectFakeLossless
	}

	// Check sample rate
	if a.SampleRate < 44100 {
		a.Issues = append(a.Issues, fmt.Sprintf("Low sample rate: %d Hz (standard is 44.1kHz+)", a.SampleRate))
	}

	// Check bit depth for lossless
	if isLossless && a.BitsPerSample < 16 {
		a.Issues = append(a.Issues, fmt.Sprintf("Unusual bit depth: %d-bit", a.BitsPerSample))
	}

	// Check channels
	if a.Channels < 2 {
		a.Issues = append(a.Issues, "Mono audio")
	} else if a.Channels > 2 {
		// Not an issue, just info
	}

	// Check for very low bitrate (lossy)
	if !isLossless && a.Bitrate > 0 && a.Bitrate < 128000 {
		a.Issues = append(a.Issues, fmt.Sprintf("Low bitrate: %d kbps", a.Bitrate/1000))
	}
}

// detectFakeLossless attempts to detect if a lossless file was upscaled from lossy
func (a *AudioAnalysis) detectFakeLossless() {
	if !a.IsTrueLossless {
		return
	}

	// Use ffmpeg to analyze frequency content
	// Generate a frequency analysis using astats/loudnorm filter
	ffmpegPath := GetFFmpegPath()

	// Method: Check if there's content above 16kHz (lossy usually cuts off there)
	// Use showfreqs or astats filter
	args := []string{
		"-i", a.FilePath,
		"-af", "aformat=sample_fmts=flt,astats=metadata=1:reset=1",
		"-f", "null",
		"-t", "30", // Analyze first 30 seconds
		"-",
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	cmd.Run() // Ignore error, we parse stderr

	output := stderr.String()

	// Look for signs of frequency cutoff
	// This is a heuristic - spectral analysis would be more accurate
	// For now, check if the file claims high sample rate but has suspicious characteristics

	if a.SampleRate >= 44100 {
		// Calculate expected bitrate for true lossless
		expectedMinBitrate := a.SampleRate * a.BitsPerSample * a.Channels / 2 // Conservative estimate with compression

		// If actual bitrate is suspiciously low for "lossless", might be fake
		if a.Bitrate > 0 && a.Bitrate < expectedMinBitrate/3 {
			a.FakeLossless = true
			a.IsTrueLossless = false
			a.Issues = append(a.Issues, "Possible fake lossless: unusually high compression ratio")
		}
	}

	// Check for clues in the output
	if strings.Contains(output, "Flat_factor") || strings.Contains(output, "Peak level") {
		// Could parse these for more sophisticated analysis
	}

	// Estimate max frequency content
	// In a true lossless file at 44.1kHz, content can go up to ~22kHz
	// MP3 typically cuts off around 16-18kHz
	a.MaxFreq = a.SampleRate / 2 // Nyquist frequency
}

// calculateQualityScore computes an overall quality score (0-100)
func (a *AudioAnalysis) calculateQualityScore() {
	score := 100

	// Codec scoring
	codecLower := strings.ToLower(a.Codec)
	switch {
	case codecLower == "flac" || codecLower == "alac":
		// Best case
	case codecLower == "wav" || strings.HasPrefix(codecLower, "pcm"):
		// Also lossless
	case codecLower == "aac":
		score -= 20
	case codecLower == "mp3":
		score -= 30
	case codecLower == "opus":
		score -= 15 // Opus is very efficient
	case codecLower == "vorbis":
		score -= 25
	default:
		score -= 10
	}

	// Sample rate scoring
	switch {
	case a.SampleRate >= 96000:
		score += 5 // Hi-res bonus
	case a.SampleRate >= 48000:
		// Good
	case a.SampleRate >= 44100:
		// Standard CD quality
	case a.SampleRate >= 22050:
		score -= 20
	default:
		score -= 40
	}

	// Bit depth scoring (for lossless)
	if a.IsTrueLossless {
		switch {
		case a.BitsPerSample >= 24:
			score += 5 // Hi-res bonus
		case a.BitsPerSample >= 16:
			// Standard
		default:
			score -= 10
		}
	}

	// Bitrate scoring (for lossy)
	if !a.IsTrueLossless && a.Bitrate > 0 {
		switch {
		case a.Bitrate >= 320000:
			// Best lossy quality
		case a.Bitrate >= 256000:
			score -= 5
		case a.Bitrate >= 192000:
			score -= 10
		case a.Bitrate >= 128000:
			score -= 20
		default:
			score -= 30
		}
	}

	// Fake lossless penalty
	if a.FakeLossless {
		score -= 30
	}

	// Issue penalties
	score -= len(a.Issues) * 5

	// Clamp score
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	a.QualityScore = score

	// Set rating
	switch {
	case score >= 90:
		a.QualityRating = "Excellent"
	case score >= 75:
		a.QualityRating = "Good"
	case score >= 50:
		a.QualityRating = "Fair"
	default:
		a.QualityRating = "Poor"
	}
}

// GenerateSpectrogram creates a spectrogram image for the audio file
func GenerateSpectrogram(inputPath, outputPath string) error {
	ffmpegPath := GetFFmpegPath()

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate spectrogram using showspectrumpic filter
	// This creates a visual representation of the frequency content over time
	args := []string{
		"-y",
		"-i", inputPath,
		"-lavfi", "showspectrumpic=s=1024x512:mode=combined:color=intensity:scale=log:fscale=lin:saturation=1:win_func=hann",
		"-frames:v", "1",
		outputPath,
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("spectrogram generation failed: %v - %s", err, stderr.String())
	}

	return nil
}

// GenerateWaveform creates a waveform image for the audio file
func GenerateWaveform(inputPath, outputPath string) error {
	ffmpegPath := GetFFmpegPath()

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate waveform using showwavespic filter
	args := []string{
		"-y",
		"-i", inputPath,
		"-lavfi", "showwavespic=s=1024x256:colors=0x00ff00",
		"-frames:v", "1",
		outputPath,
	}

	cmd := exec.Command(ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("waveform generation failed: %v - %s", err, stderr.String())
	}

	return nil
}

// GetAudioFingerprint generates an acoustic fingerprint for the audio
// This can be used for duplicate detection or audio matching
func GetAudioFingerprint(filePath string) (string, error) {
	// Check if chromaprint (fpcalc) is available
	fpcalcPath, err := exec.LookPath("fpcalc")
	if err != nil {
		return "", fmt.Errorf("chromaprint (fpcalc) not installed")
	}

	args := []string{"-json", filePath}

	cmd := exec.Command(fpcalcPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("fingerprint generation failed: %v - %s", err, stderr.String())
	}

	var result struct {
		Fingerprint string  `json:"fingerprint"`
		Duration    float64 `json:"duration"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return "", fmt.Errorf("failed to parse fingerprint: %w", err)
	}

	return result.Fingerprint, nil
}

// FormatBitDepth returns a human-readable bit depth string
func FormatBitDepth(bits int) string {
	if bits <= 0 {
		return "Unknown"
	}
	return fmt.Sprintf("%d-bit", bits)
}

// FormatSampleRate returns a human-readable sample rate string
func FormatSampleRate(hz int) string {
	if hz <= 0 {
		return "Unknown"
	}
	if hz >= 1000 {
		return fmt.Sprintf("%.1f kHz", float64(hz)/1000)
	}
	return fmt.Sprintf("%d Hz", hz)
}

// FormatBitrate returns a human-readable bitrate string
func FormatBitrate(bps int) string {
	if bps <= 0 {
		return "Unknown"
	}
	if bps >= 1000000 {
		return fmt.Sprintf("%.1f Mbps", float64(bps)/1000000)
	}
	if bps >= 1000 {
		return fmt.Sprintf("%d kbps", bps/1000)
	}
	return fmt.Sprintf("%d bps", bps)
}

// IsHiRes checks if the audio qualifies as hi-res audio
func (a *AudioAnalysis) IsHiRes() bool {
	// Hi-res is typically defined as better than CD quality (16-bit/44.1kHz)
	return a.IsTrueLossless && (a.SampleRate > 44100 || a.BitsPerSample > 16)
}

// GetQualityBadge returns a short badge text for the quality
func (a *AudioAnalysis) GetQualityBadge() string {
	if a.FakeLossless {
		return "Fake Lossless"
	}
	if a.IsHiRes() {
		return fmt.Sprintf("Hi-Res %d/%d", a.BitsPerSample, a.SampleRate/1000)
	}
	if a.IsTrueLossless {
		return "Lossless"
	}

	// For lossy, show codec and bitrate
	if a.Bitrate > 0 {
		return fmt.Sprintf("%s %d", strings.ToUpper(a.Codec), a.Bitrate/1000)
	}
	return strings.ToUpper(a.Codec)
}
