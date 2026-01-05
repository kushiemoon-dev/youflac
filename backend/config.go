package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Application configuration and settings

type Config struct {
	OutputDirectory     string   `json:"outputDirectory"`
	VideoQuality        string   `json:"videoQuality"` // "best", "1080p", "720p"
	AudioSourcePriority []string `json:"audioSourcePriority"` // ["tidal", "qobuz", "amazon"]
	NamingTemplate      string   `json:"namingTemplate"`
	GenerateNFO         bool     `json:"generateNfo"`
	ConcurrentDownloads int      `json:"concurrentDownloads"`
	EmbedCoverArt       bool     `json:"embedCoverArt"`
	Theme               string   `json:"theme"`          // "dark", "light", "system"
	CookiesBrowser      string   `json:"cookiesBrowser"` // "firefox", "chrome", "chromium", "brave", "opera", "edge", ""
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
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "MusicVideos")
}
