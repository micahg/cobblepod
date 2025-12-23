package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// TokenProvider interface for dependency injection
type TokenProvider interface {
	GetGoogleAccessToken(ctx context.Context, userID string) (string, error)
}

// DefaultTokenProvider implementation
type DefaultTokenProvider struct{}

func (p *DefaultTokenProvider) GetGoogleAccessToken(ctx context.Context, userID string) (string, error) {
	return GetGoogleAccessToken(ctx, userID)
}

// GetGoogleAccessToken exchanges Auth0 token for Google access token using the user ID from context
func GetGoogleAccessToken(ctx context.Context, userID string) (string, error) {
	config := GetAuth0Config()

	// Get cached or new management token
	mgmtToken, err := GetCachedManagementToken(config)
	if err != nil {
		return "", fmt.Errorf("failed to get management token: %w", err)
	}

	slog.Info("Fetching Google access token for user", "sub", userID)

	// Use management token to fetch user's identity provider tokens
	googleToken, err := getUserGoogleToken(userID, mgmtToken, config)
	if err != nil {
		return "", fmt.Errorf("failed to get Google token: %w", err)
	}

	return googleToken, nil
}

// getUserGoogleToken fetches the Google access token for a user
func getUserGoogleToken(userID, mgmtToken string, config *Auth0Config) (string, error) {
	// Extract the connection from user ID (e.g., "google-oauth2|123456")
	parts := strings.Split(userID, "|")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid user ID format: %s", userID)
	}

	url := fmt.Sprintf("https://%s/api/v2/users/%s", config.Domain, userID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", mgmtToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get user info, status %d: %s", resp.StatusCode, string(body))
	}

	var user struct {
		Identities []struct {
			Provider    string `json:"provider"`
			AccessToken string `json:"access_token"`
		} `json:"identities"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}

	// Find Google identity
	for _, identity := range user.Identities {
		if identity.Provider == "google-oauth2" {
			if identity.AccessToken == "" {
				return "", fmt.Errorf("google access token not available for user")
			}
			return identity.AccessToken, nil
		}
	}

	return "", fmt.Errorf("no google identity found for user")
}
