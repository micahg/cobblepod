package config

import (
	"os"
	"strconv"
)

var (
	// Google Drive and Cloud settings (legacy)
	Scopes = []string{"https://www.googleapis.com/auth/drive"}

	// Audio processing settings
	DefaultSpeed     = 1.5
	MaxFFMPEGWorkers = 4

	// State
	ValkeyHost = getEnvWithDefault("VALKEY_HOST", "localhost")
	ValkeyPort = getEnvInt("VALKEY_PORT", 6379)
)

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// M3UQuery is the query used to search for M3U files in Google Drive
const M3UQuery = "name contains '.m3u' and trashed=false"

// RSSQuery is the query used to search for RSS files in Google Drive
const RSSQuery = "name = 'playrun_addict.xml' and trashed=false"
