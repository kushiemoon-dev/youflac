package backend

import "testing"

func TestIsQualityDowngrade(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		actual    string
		want      bool
	}{
		// Downgrades
		{"hi_res to lossless", "hi_res", "lossless", true},
		{"highest to flac", "highest", "flac", true},
		{"24bit to 16bit", "24bit", "16bit", true},
		{"hires to mp3", "hires", "mp3", true},
		{"lossless to lossy", "lossless", "lossy", true},

		// Same quality (no downgrade)
		{"lossless to lossless", "lossless", "lossless", false},
		{"flac to flac", "flac", "flac", false},
		{"hi_res to hi_res", "hi_res", "hi_res", false},

		// Upgrade (no downgrade)
		{"mp3 to flac", "mp3", "flac", false},
		{"lossy to lossless", "lossy", "lossless", false},

		// Unknown quality strings (no classification)
		{"empty requested", "", "flac", false},
		{"empty actual", "flac", "", false},
		{"unknown both", "premium", "standard", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isQualityDowngrade(tt.requested, tt.actual)
			if got != tt.want {
				t.Errorf("isQualityDowngrade(%q, %q) = %v, want %v", tt.requested, tt.actual, got, tt.want)
			}
		})
	}
}

func TestQualityRankOf(t *testing.T) {
	tests := []struct {
		quality string
		wantMin int // minimum expected rank
	}{
		{"hi_res 24-bit/96kHz", 3},
		{"24bit", 3},
		{"HIRES", 3},
		{"lossless 16-bit/44.1kHz", 2},
		{"FLAC", 2},
		{"16bit", 2},
		{"mp3 320kbps", 1},
		{"lossy", 1},
		{"unknown codec", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.quality, func(t *testing.T) {
			got := qualityRankOf(tt.quality)
			if got != tt.wantMin {
				t.Errorf("qualityRankOf(%q) = %d, want %d", tt.quality, got, tt.wantMin)
			}
		})
	}
}
