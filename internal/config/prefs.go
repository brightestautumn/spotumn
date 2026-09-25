// User preferences and configuration management - loads and saves spotumn.json with default fallbacks.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SpotifyClientID          = "d420a117a32841c2b3474932e49fb54b"
	SpotifyLibrespotClientID = "65b708073fc0480ea92a077233ca87bd"
)

// Log and LogError are set in cmd/spotumn to route output to ~/.cache/spotumn/spotumn.log.
var (
	Log      func(component, msg, file string)
	LogError func(component, msg, file string)
)

// LogMsg logs an event or change if logger is initialized.
func LogMsg(component, msg, file string) {
	if Log != nil {
		Log(component, msg, file)
	}
}

// LogErr logs an error (tagged [ERROR]) if err != nil and logger is initialized.
func LogErr(component string, err error, file string) {
	if err != nil && LogError != nil {
		LogError(component, err.Error(), file)
	}
}

type Config struct {
	Port               int    `yaml:"port"`
	RedirectURI        string `yaml:"redirect_uri"`
	ArtRenderer        string `yaml:"art_renderer"`
	Theme              string `yaml:"theme"`
	AppearanceMode     string `yaml:"appearance_mode"`
	AutoShrinkSidebars bool   `yaml:"auto_shrink_sidebars"`
	AudioBackend       string `yaml:"audio_backend"`
	CrossfadeSec       int    `yaml:"crossfade"`
	Bitrate            int    `yaml:"bitrate"`
	Normalisation      bool   `yaml:"normalisation"`
	AutoplayOnStartup  bool   `yaml:"autoplay_on_startup"`
}

const DefaultPort = 8989

func GetDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".spotumn"
	}
	dir := filepath.Join(home, ".config", "spotumn")
	if err := os.MkdirAll(dir, 0700); err != nil && LogError != nil {
		LogError("config", "mkdir config dir: "+err.Error(), "prefs.go")
	}
	return dir
}

func GetCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(GetDir(), "cache")
	}
	dir := filepath.Join(home, ".cache", "spotumn")
	if err := os.MkdirAll(dir, 0700); err != nil && LogError != nil {
		LogError("config", "mkdir cache dir: "+err.Error(), "prefs.go")
	}
	return dir
}

func GetThemesDir() string {
	dir := filepath.Join(GetDir(), "themes")
	if err := os.MkdirAll(dir, 0700); err != nil && LogError != nil {
		LogError("config", "mkdir themes dir: "+err.Error(), "prefs.go")
	}
	return dir
}

func defaults() *Config {
	return &Config{
		Port:               DefaultPort,
		ArtRenderer:        "auto",
		Theme:              "spotify",
		AppearanceMode:     "dark",
		AutoShrinkSidebars: true,
		AudioBackend:       "auto",
		Bitrate:            320,
		Normalisation:      true,
		AutoplayOnStartup:  false,
	}
}

func Get() *Config {
	cfg, err := Load()
	if err != nil {
		return defaults()
	}
	return cfg
}

func Load() (*Config, error) {
	cfg := defaults()

	// load config from disk or write default template if not found
	configPath := filepath.Join(GetDir(), "config.yml")
	data, err := os.ReadFile(configPath)
	if err != nil && os.IsNotExist(err) {
		if saveErr := Save(cfg); saveErr != nil && LogError != nil {
			LogError("config", "save default config: "+saveErr.Error(), "prefs.go")
		}
	} else if err == nil {
		if unmErr := yaml.Unmarshal(data, cfg); unmErr != nil && LogError != nil {
			LogError("config", "unmarshal config: "+unmErr.Error(), "prefs.go")
		}
	}

	// environment variables take precedence over config file
	if envURI := strings.TrimSpace(os.Getenv("SPOTUMN_REDIRECT_URI")); envURI != "" {
		cfg.RedirectURI = envURI
	}
	if envArt := strings.TrimSpace(os.Getenv("SPOTUMN_ART_RENDERER")); envArt != "" {
		cfg.ArtRenderer = strings.ToLower(envArt)
	}

	// sanitize fields and fallback to safe defaults
	cfg.RedirectURI = strings.TrimSpace(cfg.RedirectURI)
	if cfg.Port <= 0 {
		cfg.Port = DefaultPort
	}
	cfg.ArtRenderer = strings.ToLower(strings.TrimSpace(cfg.ArtRenderer))
	if cfg.ArtRenderer != "image" && cfg.ArtRenderer != "ansi" {
		cfg.ArtRenderer = "auto"
	}
	if cfg.Theme == "" {
		cfg.Theme = "spotify"
	}
	cfg.AppearanceMode = strings.ToLower(strings.TrimSpace(cfg.AppearanceMode))
	if cfg.AppearanceMode != "light" && cfg.AppearanceMode != "dark" {
		cfg.AppearanceMode = "dark"
	}
	if cfg.AudioBackend == "" {
		cfg.AudioBackend = "auto"
	}
	if cfg.Bitrate != 96 && cfg.Bitrate != 160 && cfg.Bitrate != 320 {
		cfg.Bitrate = 320
	}
	if cfg.CrossfadeSec < 0 {
		cfg.CrossfadeSec = 0
	}
	if cfg.CrossfadeSec > 12 {
		cfg.CrossfadeSec = 12
	}

	return cfg, nil
}

func Save(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(GetDir(), "config.yml"), data, 0600)
}
