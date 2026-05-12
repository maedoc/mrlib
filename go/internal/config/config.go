package config

import (
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
	APIKey          string
	BaseURL         string
	Timeout         time.Duration
	RateLimitDelay  time.Duration
	MaxRetries      int
	Verbose         bool
	Debug           bool
	Sync            SyncConfig
}

// SyncConfig represents sync-specific configuration
type SyncConfig struct {
	Direction       string
	Mode           string
	BatchSize      int
	MaxWorkers     int
	DryRun         bool
	Force          bool
	StateFile      string
	Extensions     []string
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
			Direction:      "both",
			Mode:          "safe",
			BatchSize:     10,
			MaxWorkers:    4,
			DryRun:        false,
			Force:         false,
			StateFile:     ".mistral_sync_state.json",
			Extensions:    []string{},
			ExcludePatterns: []string{},
			IncludePatterns: []string{},
		},
	}
}

// LoadConfig loads configuration from file and environment
func LoadConfig(configPath string) (*Config, error) {
	config := DefaultConfig()

	// Set up viper
	v := viper.New()
	v.SetConfigName("mistral-file-sync")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath(filepath.Dir(configPath))
	v.AddConfigPath("$HOME")
	v.AddConfigPath("/etc/mistral-file-sync/")

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
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
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

	return config, nil
}

// SaveConfig saves configuration to a file
func SaveConfig(config *Config, path string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	// Check if config exists in current directory
	if _, err := os.Stat(".mistral-file-sync.yaml"); err == nil {
		return ".mistral-file-sync.yaml"
	}

	// Check home directory
	home := os.Getenv("HOME")
	if home != "" {
		path := filepath.Join(home, ".mistral-file-sync.yaml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
