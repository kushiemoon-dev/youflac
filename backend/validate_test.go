package backend

import (
	"testing"
)

// ============================================================================
// ValidateYouTubeURL
// ============================================================================

func TestValidateYouTubeURL_Valid(t *testing.T) {
	cases := []string{
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ",
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://www.youtube.com/playlist?list=PLxxx",
	}
	for _, u := range cases {
		t.Run(u, func(t *testing.T) {
			if err := ValidateYouTubeURL(u); err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}
}

func TestValidateYouTubeURL_Invalid(t *testing.T) {
	cases := []struct {
		url    string
		reason string
	}{
		{"javascript:alert(1)", "javascript scheme"},
		{"http://youtube.com/watch?v=xxx", "http not https"},
		{"https://evil.com/watch?v=xxx", "wrong domain"},
		{"https://notyoutube.com/", "wrong domain"},
		{"ftp://youtube.com/watch?v=xxx", "ftp scheme"},
		{string(make([]byte, 2049)), "too long"},
		{"", "empty"},
		{"just-text", "no scheme"},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			if err := ValidateYouTubeURL(tc.url); err == nil {
				t.Errorf("expected error for %q (%s), got nil", tc.url, tc.reason)
			}
		})
	}
}

// ============================================================================
// SanitizePlaylistName
// ============================================================================

func TestSanitizePlaylistName_Valid(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"My Playlist", "My Playlist"},
		{"../evil", "evil"},      // ".." removed, "/" removed
		{"/etc/passwd", "etcpasswd"},
		{"back\\slash", "backslash"},
		{"  trim me  ", "trim me"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := SanitizePlaylistName(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestSanitizePlaylistName_NullByte(t *testing.T) {
	_, err := SanitizePlaylistName("bad\x00name")
	if err == nil {
		t.Error("expected error for null byte, got nil")
	}
}

func TestSanitizePlaylistName_EmptyAfterSanitize(t *testing.T) {
	_, err := SanitizePlaylistName("../")
	if err == nil {
		t.Error("expected error for name that becomes empty, got nil")
	}
}

// ============================================================================
// ValidateOutputDirectory
// ============================================================================

func TestValidateOutputDirectory_Valid(t *testing.T) {
	cases := []string{
		"",
		"/home/user/Music",
		"/mnt/nas/downloads",
		"/data/music",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			if err := ValidateOutputDirectory(p); err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}
}

func TestValidateOutputDirectory_SystemPaths(t *testing.T) {
	cases := []string{
		"/etc",
		"/etc/passwd",
		"/root",
		"/proc/self",
		"/sys/kernel",
		"/bin",
		"/dev/null",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			if err := ValidateOutputDirectory(p); err == nil {
				t.Errorf("expected error for system path %q, got nil", p)
			}
		})
	}
}

// ============================================================================
// ValidateAudioSources
// ============================================================================

func TestValidateAudioSources_Valid(t *testing.T) {
	if err := ValidateAudioSources([]string{"tidal", "qobuz", "amazon", "deezer"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateAudioSources([]string{"tidal"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateAudioSources([]string{}); err != nil {
		t.Errorf("empty slice should be valid: %v", err)
	}
}

func TestValidateAudioSources_Invalid(t *testing.T) {
	cases := [][]string{
		{"spotify"},
		{"tidal", "napster"},
		{"TIDAL"}, // case matters — lowercase enforced
	}
	for _, sources := range cases {
		if err := ValidateAudioSources(sources); err == nil {
			t.Errorf("expected error for %v, got nil", sources)
		}
	}
}

// ============================================================================
// ValidateTrackURL
// ============================================================================

func TestValidateTrackURL_Valid(t *testing.T) {
	cases := []string{
		"https://tidal.com/browse/track/12345",
		"https://play.qobuz.com/track/67890",
		"https://lucida.to/track/abc",
	}
	for _, u := range cases {
		t.Run(u, func(t *testing.T) {
			if err := ValidateTrackURL(u); err != nil {
				t.Errorf("expected valid, got: %v", err)
			}
		})
	}
}

func TestValidateTrackURL_Invalid(t *testing.T) {
	cases := []struct {
		url    string
		reason string
	}{
		{"http://tidal.com/track/123", "http not https"},
		{"ftp://tidal.com/track/123", "ftp scheme"},
		{"javascript:alert(1)", "javascript scheme"},
		{"not-a-url", "no scheme"},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			if err := ValidateTrackURL(tc.url); err == nil {
				t.Errorf("expected error for %q (%s)", tc.url, tc.reason)
			}
		})
	}
}

// Note: "https://valid.com/track; rm -rf /" is actually safe with exec.Command
// because Go passes args directly to the OS without shell interpretation.
// ValidateTrackURL intentionally accepts such URLs — the protection is the
// https-only scheme check, which prevents protocol-level injection.
