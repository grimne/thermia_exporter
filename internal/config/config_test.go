package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig_EnvVars(t *testing.T) {
	// Set test environment variables
	os.Setenv("THERMIA_USERNAME", "test@example.com")
	os.Setenv("THERMIA_PASSWORD", "testpass123")
	os.Setenv("THERMIA_ADDR", ":9999")
	os.Setenv("THERMIA_LOG_LEVEL", "debug")
	os.Setenv("THERMIA_LOG_FORMAT", "json")
	defer func() {
		os.Unsetenv("THERMIA_USERNAME")
		os.Unsetenv("THERMIA_PASSWORD")
		os.Unsetenv("THERMIA_ADDR")
		os.Unsetenv("THERMIA_LOG_LEVEL")
		os.Unsetenv("THERMIA_LOG_FORMAT")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Username != "test@example.com" {
		t.Errorf("Username = %v, want test@example.com", cfg.Username)
	}
	if cfg.Password != "testpass123" {
		t.Errorf("Password = %v, want testpass123", cfg.Password)
	}
	if cfg.ListenAddr != ":9999" {
		t.Errorf("ListenAddr = %v, want :9999", cfg.ListenAddr)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %v, want json", cfg.LogFormat)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("THERMIA_ADDR")
	os.Unsetenv("THERMIA_LOG_LEVEL")
	os.Unsetenv("THERMIA_LOG_FORMAT")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.ListenAddr != ":9808" {
		t.Errorf("ListenAddr = %v, want :9808", cfg.ListenAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %v, want text", cfg.LogFormat)
	}
	if cfg.RequestTimeout != 2*time.Minute {
		t.Errorf("RequestTimeout = %v, want 2m", cfg.RequestTimeout)
	}
}

func TestValidate_MissingUsername(t *testing.T) {
	cfg := &Config{
		Password: "password",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for missing username, got nil")
	}
}

func TestValidate_MissingPassword(t *testing.T) {
	cfg := &Config{
		Username: "user@example.com",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for missing password, got nil")
	}
}

func TestValidate_InvalidTimeout(t *testing.T) {
	cfg := &Config{
		Username:       "user@example.com",
		Password:       "password",
		RequestTimeout: 5 * time.Second,
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for timeout < 10s, got nil")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{
		Username:       "user@example.com",
		Password:       "password",
		RequestTimeout: 30 * time.Second,
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}
