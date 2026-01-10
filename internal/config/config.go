package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
)

type Config struct {
	// Agent Identity
	NodeID      string `json:"node_id"`
	Fingerprint string `json:"fingerprint"`

	// Enrollment
	EnrollToken string `json:"-"` // Never persist token
	CertPath    string `json:"cert_path"`
	KeyPath     string `json:"key_path"`
	CACertPath  string `json:"ca_cert_path"`

	// Backend Connection
	BackendURL string `json:"backend_url"`

	// Agent Settings
	HeartbeatInterval int    `json:"heartbeat_interval"` // seconds
	MetricInterval    int    `json:"metric_interval"`    // seconds
	LogLevel          string `json:"log_level"`

	// Paths
	DataDir   string `json:"data_dir"`
	LogDir    string `json:"log_dir"`
	BufferDir string `json:"buffer_dir"`
}

// DefaultConfig returns platform-specific defaults
func DefaultConfig() *Config {
	var dataDir, logDir string

	if runtime.GOOS == "windows" {
		dataDir = filepath.Join(os.Getenv("ProgramData"), "einfra", "agent")
		logDir = filepath.Join(dataDir, "logs")
	} else {
		dataDir = "/var/lib/einfra-agent"
		logDir = "/var/log/einfra-agent"
	}

	return &Config{
		BackendURL:        os.Getenv("EINFRA_BACKEND_URL"),
		EnrollToken:       os.Getenv("EINFRA_ENROLL_TOKEN"),
		HeartbeatInterval: 30,
		MetricInterval:    60,
		LogLevel:          "info",
		DataDir:           dataDir,
		LogDir:            logDir,
		BufferDir:         filepath.Join(dataDir, "buffer"),
		CertPath:          filepath.Join(dataDir, "certs", "agent.crt"),
		KeyPath:           filepath.Join(dataDir, "certs", "agent.key"),
		CACertPath:        filepath.Join(dataDir, "certs", "ca.crt"),
	}
}

// Load reads config from file, merges with env vars
func Load(configPath string) (*Config, error) {
	// Load .env if exists
	_ = godotenv.Load()

	cfg := DefaultConfig()

	// Try to load from file
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err == nil {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config: %w", err)
			}
		}
	}

	// Override with env vars
	if url := os.Getenv("EINFRA_BACKEND_URL"); url != "" {
		cfg.BackendURL = url
	}
	if token := os.Getenv("EINFRA_ENROLL_TOKEN"); token != "" {
		cfg.EnrollToken = token
	}

	// Ensure directories exist
	for _, dir := range []string{cfg.DataDir, cfg.LogDir, cfg.BufferDir, filepath.Dir(cfg.CertPath)} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return cfg, nil
}

// Save persists config to file
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
