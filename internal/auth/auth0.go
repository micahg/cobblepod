package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Auth0Config holds Auth0 configuration
type Auth0Config struct {
	Domain       string
	Audience     string
	ClientID     string
	ClientSecret string
}

// ManagementTokenCache holds a cached management token
type ManagementTokenCache struct {
	token     string
	expiresAt time.Time
	mu        sync.RWMutex
}

var mgmtTokenCache = &ManagementTokenCache{}

// GetAuth0Config returns Auth0 configuration from environment
func GetAuth0Config() *Auth0Config {
	return &Auth0Config{
		Domain:       os.Getenv("AUTH0_DOMAIN"),
		Audience:     os.Getenv("AUTH0_AUDIENCE"),
		ClientID:     os.Getenv("AUTH0_CLIENT_ID"),
		ClientSecret: os.Getenv("AUTH0_CLIENT_SECRET"),
	}
}

// GetCachedManagementToken returns a cached management token or fetches a new one
func GetCachedManagementToken(config *Auth0Config) (string, error) {
	mgmtTokenCache.mu.RLock()
	if mgmtTokenCache.token != "" && time.Now().Before(mgmtTokenCache.expiresAt) {
		token := mgmtTokenCache.token
		mgmtTokenCache.mu.RUnlock()
		slog.Debug("Using cached management token", "expiresAt", mgmtTokenCache.expiresAt)
		return token, nil
	}
	mgmtTokenCache.mu.RUnlock()

	// Need to fetch new token
	mgmtTokenCache.mu.Lock()
	defer mgmtTokenCache.mu.Unlock()

	// Double-check after acquiring write lock
	if mgmtTokenCache.token != "" && time.Now().Before(mgmtTokenCache.expiresAt) {
		slog.Debug("Using cached management token (after lock)", "expiresAt", mgmtTokenCache.expiresAt)
		return mgmtTokenCache.token, nil
	}

	slog.Info("Fetching new Auth0 management token")
	token, expiresIn, err := getManagementAPIToken(config)
	if err != nil {
		return "", err
	}

	// Cache with a small buffer before expiration (5 minutes early)
	mgmtTokenCache.token = token
	mgmtTokenCache.expiresAt = time.Now().Add(time.Duration(expiresIn)*time.Second - 5*time.Minute)

	slog.Info("Cached new management token", "expiresAt", mgmtTokenCache.expiresAt, "expiresInSeconds", expiresIn)
	return token, nil
}

// getManagementAPIToken gets an Auth0 Management API token
func getManagementAPIToken(config *Auth0Config) (string, int, error) {
	url := fmt.Sprintf("https://%s/oauth/token", config.Domain)

	payload := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     config.ClientID,
		"client_secret": config.ClientSecret,
		"audience":      fmt.Sprintf("https://%s/api/v2/", config.Domain),
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("auth0 returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, err
	}

	return result.AccessToken, result.ExpiresIn, nil
}
