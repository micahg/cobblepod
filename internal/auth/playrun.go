package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

const playrunBaseURL = "https://www.playrun.app"

// PlayrunAuthenticator is the interface for logging in to Playrun.
type PlayrunAuthenticator interface {
	Login(email, password string) (string, error)
}

// DefaultPlayrunAuthenticator is the production implementation of
// PlayrunAuthenticator. It talks to https://www.playrun.app/api/auth.
type DefaultPlayrunAuthenticator struct {
	BaseURL string // overrides playrunBaseURL when non-empty (used by tests)
}

// Login logs in to Playrun with email/password credentials and returns the
// Playrun JWT. The caller is responsible for obtaining the credentials and for
// storing the returned token.
func (a *DefaultPlayrunAuthenticator) Login(email, password string) (string, error) {
	baseURL := a.BaseURL
	if baseURL == "" {
		baseURL = playrunBaseURL
	}
	url := baseURL + "/api/auth"

	payload := map[string]string{
		"email":    email,
		"password": password,
	}
	payloadBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	slog.Info("Logging in to Playrun", "email", email)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("playrun auth returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token string `json:"token"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Token == "" {
		return "", fmt.Errorf("playrun auth response did not contain a token")
	}

	return result.Token, nil
}

// PlayrunLogin is a convenience wrapper that authenticates against the default
// Playrun endpoint using the package-level DefaultPlayrunAuthenticator.
func PlayrunLogin(email, password string) (string, error) {
	return (&DefaultPlayrunAuthenticator{}).Login(email, password)
}

// PlayrunTokenClaims holds the subset of JWT claims we surface as Playrun
// connection status. The `exp` claim is a standard NumericDate (seconds since
// epoch); the `email` claim is the Playrun account identifier.
type PlayrunTokenClaims struct {
	Exp   int64  `json:"exp"`
	Email string `json:"email"`
}

// ParsePlayrunToken decodes the payload of a Playrun JWT and returns the
// claims we care about (exp and email). It does NOT verify the signature:
// the token was obtained from Playrun on login and stored locally, so we
// trust its contents rather than maintaining a verification key. The caller
// is responsible for treating an unparseable token as "not logged in".
func ParsePlayrunToken(token string) (PlayrunTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return PlayrunTokenClaims{}, fmt.Errorf("invalid JWT: expected 3 parts, got %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return PlayrunTokenClaims{}, fmt.Errorf("invalid JWT payload: %w", err)
	}
	var claims PlayrunTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return PlayrunTokenClaims{}, fmt.Errorf("invalid JWT claims: %w", err)
	}
	return claims, nil
}
