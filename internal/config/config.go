// Package config handles configuration loading from environment variables and Kubernetes secrets.
package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the thermia exporter.
type Config struct {
	// Authentication credentials
	Username string
	Password string

	// Server configuration
	ListenAddr     string
	RequestTimeout time.Duration

	// Logging configuration
	LogLevel  string // debug, info, warn, error
	LogFormat string // text, json
}

// LoadConfig loads configuration from environment variables and Kubernetes secrets.
// It tries Kubernetes secrets first, then falls back to environment variables.
func LoadConfig() (*Config, error) {
	cfg := &Config{
		// Set defaults
		ListenAddr:     ":9808",
		RequestTimeout: 2 * time.Minute,
		LogLevel:       "info",
		LogFormat:      "text",
	}

	// Try to load from Kubernetes secrets first
	username, password, err := tryLoadFromSecrets()
	if err == nil && username != "" && password != "" {
		cfg.Username = username
		cfg.Password = password
	} else {
		// Fallback to environment variables
		cfg.Username = os.Getenv("THERMIA_USERNAME")
		cfg.Password = os.Getenv("THERMIA_PASSWORD")
	}

	// Override defaults from environment variables
	if addr := os.Getenv("THERMIA_ADDR"); addr != "" {
		cfg.ListenAddr = addr
	}

	if level := os.Getenv("THERMIA_LOG_LEVEL"); level != "" {
		cfg.LogLevel = level
	}

	if format := os.Getenv("THERMIA_LOG_FORMAT"); format != "" {
		cfg.LogFormat = format
	}

	if timeout := os.Getenv("THERMIA_REQUEST_TIMEOUT"); timeout != "" {
		if seconds, err := strconv.Atoi(timeout); err == nil && seconds > 0 {
			cfg.RequestTimeout = time.Duration(seconds) * time.Second
		}
	}

	return cfg, nil
}

// Validate checks that all required configuration fields are set.
func (c *Config) Validate() error {
	if c.Username == "" {
		return errors.New("username is required (set THERMIA_USERNAME or mount K8s secret)")
	}
	if c.Password == "" {
		return errors.New("password is required (set THERMIA_PASSWORD or mount K8s secret)")
	}
	if c.RequestTimeout < 10*time.Second {
		return errors.New("request timeout must be at least 10 seconds")
	}
	return nil
}
