package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	APIKey         string
	BaseURL        string
	Timeout        time.Duration
	RateLimitDelay time.Duration
	MaxRetries     int
	Verbose        bool
	Debug          bool
	Sync           SyncConfig
}

// SyncConfig represents sync-specific configuration
type SyncConfig struct {
	Direction       string
	Mode            string
	BatchSize       int
	MaxWorkers      int
	DryRun          bool
	Force           bool
	StateFile       string
	Extensions      []string
	ExcludePatterns []string
	IncludePatterns []string
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		BaseURL:        "https://api.mistral.ai/v1",
		Timeout:        120 * time.Second,
		RateLimitDelay: 1 * time.Second,
		MaxRetries:     3,
		Verbose:        false,
		Debug:          false,
		Sync: SyncConfig{
			Direction:       "both",
			Mode:            "safe",
			BatchSize:       10,
			MaxWorkers:      4,
			DryRun:          false,
			Force:           false,
			StateFile:       ".mrlib_sync_state.json",
			Extensions:      []string{},
			ExcludePatterns: []string{},
			IncludePatterns: []string{},
		},
	}
}

// expandPath expands a leading $HOME or ~ in a path
func expandPath(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	if strings.HasPrefix(path, "$HOME") {
		if home := os.Getenv("HOME"); home != "" {
			return filepath.Join(home, strings.TrimPrefix(path, "$HOME"))
		}
	}
	return path
}

// LoadConfig loads configuration from file and environment
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	configPath = expandPath(configPath)

	// Set up viper
	v := viper.New()
	v.SetConfigName("mrlib")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	if configPath != "" {
		v.AddConfigPath(filepath.Dir(configPath))
	}
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(home)
		v.AddConfigPath(filepath.Join(home, ".config", "mrlib"))
	}
	v.AddConfigPath("/etc/mrlib/")

	// Environment variables
	v.AutomaticEnv()
	v.SetEnvPrefix("MISTRAL")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Bind environment variables
	v.BindEnv("api_key", "API_KEY")
	v.BindEnv("base_url", "BASE_URL")
	v.BindEnv("timeout", "TIMEOUT")
	v.BindEnv("rate_limit_delay", "RATE_LIMIT_DELAY")
	v.BindEnv("max_retries", "MAX_RETRIES")
	v.BindEnv("verbose", "VERBOSE")
	v.BindEnv("debug", "DEBUG")

	// Read config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	}

	if err := v.ReadInConfig(); err != nil {
		// Ignore error if config file doesn't exist
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("failed to read config: %v", err)
		}
	}

	// Unmarshal config
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	// Override with environment variables
	if apiKey := os.Getenv("MISTRAL_API_KEY"); apiKey != "" {
		config.APIKey = apiKey
	}
	if baseURL := os.Getenv("MISTRAL_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	}

	return config, nil
}

// SaveConfig saves configuration to a file with owner-only permissions,
// since it may contain the API key
func SaveConfig(config *Config, path string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}

	return os.WriteFile(path, data, 0600)
}

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	// Check if config exists in current directory
	if _, err := os.Stat("mrlib.yaml"); err == nil {
		return "mrlib.yaml"
	}
	if _, err := os.Stat(".mrlib.yaml"); err == nil {
		return ".mrlib.yaml"
	}

	// Check home directory
	if home, err := os.UserHomeDir(); err == nil {
		for _, p := range []string{
			filepath.Join(home, "mrlib.yaml"),
			filepath.Join(home, ".mrlib.yaml"),
			filepath.Join(home, ".config", "mrlib", "mrlib.yaml"),
		} {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	return ""
}
