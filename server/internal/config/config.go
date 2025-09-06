package config

import (
	"os"
	"strconv"

	"github.com/google/uuid"
)

var (
	// Google Drive and Cloud settings
	Scopes           = []string{"https://www.googleapis.com/auth/drive"}
	ProjectID        = os.Getenv("GOOGLE_CLOUD_PROJECT_ID")
	TopicName        = getEnvWithDefault("PUBSUB_TOPIC_NAME", "m3u8-processor")
	SubscriptionName = getEnvWithDefault("PUBSUB_SUBSCRIPTION_NAME", "m3u8-processor-sub")
	WebhookURL       = os.Getenv("WEBHOOK_URL")
	WebhookSecret    = getEnvWithDefault("WEBHOOK_SECRET", uuid.New().String())

	// Audio processing settings
	DefaultSpeed     = 1.5
	MaxFFMPEGWorkers = 4

	// Email settings
	SMTPServer        = getEnvWithDefault("SMTP_SERVER", "smtp.gmail.com")
	SMTPPort          = getEnvInt("SMTP_PORT", 587)
	EmailUsername     = os.Getenv("EMAIL_USERNAME")
	EmailPassword     = os.Getenv("EMAIL_PASSWORD")
	NotificationEmail = os.Getenv("NOTIFICATION_EMAIL")

	// Polling settings
	PollInterval = getEnvInt("POLL_INTERVAL", 300)

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
