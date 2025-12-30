package config

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultSecretsPath = "/var/run/secrets/thermia"
	usernameFile       = "username"
	passwordFile       = "password"
)

// tryLoadFromSecrets attempts to read credentials from mounted Kubernetes secret files.
// Returns empty strings if the secrets path doesn't exist (not an error - allows fallback to env vars).
func tryLoadFromSecrets() (username, password string, err error) {
	secretsPath := os.Getenv("THERMIA_SECRETS_PATH")
	if secretsPath == "" {
		secretsPath = defaultSecretsPath
	}

	// Check if secrets directory exists
	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		return "", "", nil // Not an error, just not using secrets
	}

	// Read username
	usernameData, err := os.ReadFile(filepath.Join(secretsPath, usernameFile))
	if err != nil {
		// Don't fail if file doesn't exist, just return empty
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", "", err
	}
	username = strings.TrimSpace(string(usernameData))

	// Read password
	passwordData, err := os.ReadFile(filepath.Join(secretsPath, passwordFile))
	if err != nil {
		// Don't fail if file doesn't exist, just return empty
		if os.IsNotExist(err) {
			return username, "", nil
		}
		return "", "", err
	}
	password = strings.TrimSpace(string(passwordData))

	return username, password, nil
}
