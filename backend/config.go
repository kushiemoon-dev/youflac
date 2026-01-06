package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Application configuration and settings

type Config struct {
	OutputDirectory      string   `json:"outputDirectory"`
	VideoQuality         string   `json:"videoQuality"` // "best", "1080p", "720p"
	AudioSourcePriority  []string `json:"audioSourcePriority"` // ["tidal", "qobuz", "amazon"]
	NamingTemplate       string   `json:"namingTemplate"`
	GenerateNFO          bool     `json:"generateNfo"`
	ConcurrentDownloads  int      `json:"concurrentDownloads"`
	EmbedCoverArt        bool     `json:"embedCoverArt"`
	Theme                string   `json:"theme"`               // "dark", "light", "system"
	CookiesBrowser       string   `json:"cookiesBrowser"`      // "firefox", "chrome", "chromium", "brave", "opera", "edge", ""
	AccentColor          string   `json:"accentColor"`         // "pink", "blue", "green", "purple", "orange", "teal", "red", "yellow"
	SoundEffectsEnabled  bool     `json:"soundEffectsEnabled"` // Play sounds on download complete, error, etc.
	LyricsEnabled        bool     `json:"lyricsEnabled"`       // Fetch lyrics automatically
	LyricsEmbedMode      string   `json:"lyricsEmbedMode"`     // "embed", "lrc", "both"
}

var defaultConfig = Config{
	OutputDirectory:     "",
	VideoQuality:        "best",
	AudioSourcePriority: []string{"tidal", "qobuz", "amazon"},
	NamingTemplate:      "{artist}/{title}/{title}",
	GenerateNFO:         true,
	ConcurrentDownloads: 2,
	EmbedCoverArt:       true,
	Theme:               "system",
	AccentColor:         "pink",
	SoundEffectsEnabled: true,
	LyricsEnabled:       false,
	LyricsEmbedMode:     "lrc",
}

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	configDir, _ := os.UserConfigDir()
	return filepath.Join(configDir, "youflac", "config.json")
}

// GetDataPath returns the path to app data directory
func GetDataPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".youflac")
}

// GetBinPath returns the path to bundled binaries
func GetBinPath() string {
	return filepath.Join(GetDataPath(), "bin")
}

// LoadConfig loads configuration from file
func LoadConfig() (*Config, error) {
	configPath := GetConfigPath()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return &defaultConfig, nil
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config) error {
	configPath := GetConfigPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// GetDefaultOutputDirectory returns default output path
func GetDefaultOutputDirectory() string {
	// Check env var first (for Docker)
	if dir := os.Getenv("OUTPUT_DIR"); dir != "" {
		return dir
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "MusicVideos")
}

// GetDefaultConfig returns a copy of the default config
func GetDefaultConfig() *Config {
	config := defaultConfig
	return &config
}

// LoadConfigWithEnv loads config from file, then overrides with environment variables
func LoadConfigWithEnv() (*Config, error) {
	// Start with file config or defaults
	config, err := LoadConfig()
	if err != nil {
		config = GetDefaultConfig()
	}

	// Override with environment variables
	if v := os.Getenv("OUTPUT_DIR"); v != "" {
		config.OutputDirectory = v
	}
	if v := os.Getenv("VIDEO_QUALITY"); v != "" {
		config.VideoQuality = v
	}
	if v := os.Getenv("CONCURRENT_DOWNLOADS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 10 {
			config.ConcurrentDownloads = n
		}
	}
	if v := os.Getenv("NAMING_TEMPLATE"); v != "" {
		config.NamingTemplate = resolveNamingTemplate(v)
	}
	if v := os.Getenv("GENERATE_NFO"); v != "" {
		config.GenerateNFO = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("EMBED_COVER_ART"); v != "" {
		config.EmbedCoverArt = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("THEME"); v != "" {
		config.Theme = v
	}
	if v := os.Getenv("ACCENT_COLOR"); v != "" {
		config.AccentColor = v
	}
	if v := os.Getenv("SOUND_EFFECTS_ENABLED"); v != "" {
		config.SoundEffectsEnabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("LYRICS_ENABLED"); v != "" {
		config.LyricsEnabled = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("LYRICS_EMBED_MODE"); v != "" {
		config.LyricsEmbedMode = v
	}
	if v := os.Getenv("COOKIES_BROWSER"); v != "" {
		config.CookiesBrowser = v
	}
	if v := os.Getenv("AUDIO_SOURCE_PRIORITY"); v != "" {
		sources := strings.Split(v, ",")
		for i := range sources {
			sources[i] = strings.TrimSpace(sources[i])
		}
		if len(sources) > 0 {
			config.AudioSourcePriority = sources
		}
	}

	return config, nil
}

// resolveNamingTemplate converts template names to actual template strings
func resolveNamingTemplate(template string) string {
	// Map of template names to actual templates
	templates := map[string]string{
		"jellyfin": "{artist}/{title}/{title}",
		"plex":     "{artist}/{title}",
		"flat":     "{artist} - {title}",
		"album":    "{artist}/{album}/{title}",
		"year":     "{year}/{artist} - {title}",
	}

	// Check if it's a template name
	if t, ok := templates[strings.ToLower(template)]; ok {
		return t
	}

	// Otherwise assume it's already a template string
	return template
}

// GetConfigPathWithEnv returns config path, checking CONFIG_DIR env first
func GetConfigPathWithEnv() string {
	if dir := os.Getenv("CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "config.json")
	}
	return GetConfigPath()
}

// GetDataPathWithEnv returns data path, checking CONFIG_DIR env first
func GetDataPathWithEnv() string {
	if dir := os.Getenv("CONFIG_DIR"); dir != "" {
		return dir
	}
	return GetDataPath()
}
