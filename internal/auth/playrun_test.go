package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestPlayrunLogin_Mock verifies the request/response shape against a stub
// server, without touching the network.
func TestPlayrunLogin_Mock(t *testing.T) {
	const wantToken = "mock-jwt-token"
	const wantEmail = "user@example.com"
	const wantPassword = "secret"

	var gotPath, gotMethod, gotContentType string
	var gotBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":        wantToken,
			"email":        wantEmail,
			"featureFlags": []string{},
		})
	}))
	defer srv.Close()

	a := &DefaultPlayrunAuthenticator{BaseURL: srv.URL}
	token, err := a.Login(wantEmail, wantPassword)
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if token != wantToken {
		t.Fatalf("token = %q, want %q", token, wantToken)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/api/auth" {
		t.Errorf("path = %q, want /api/auth", gotPath)
	}
	if !strings.Contains(gotContentType, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody["email"] != wantEmail || gotBody["password"] != wantPassword {
		t.Errorf("body = %v, want email=%q password=%q", gotBody, wantEmail, wantPassword)
	}
}

// TestPlayrunLogin_MockError ensures a non-201 response surfaces as an error
// containing the status code and body, rather than being misread as success.
func TestPlayrunLogin_MockError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid credentials"}`))
	}))
	defer srv.Close()

	a := &DefaultPlayrunAuthenticator{BaseURL: srv.URL}
	token, err := a.Login("x", "y")
	if err == nil {
		t.Fatalf("expected error, got token %q", token)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error %q does not mention status 401", err)
	}
	if !strings.Contains(err.Error(), "invalid credentials") {
		t.Errorf("error %q does not include response body", err)
	}
}

// TestPlayrunLogin_MockNoToken ensures a 201 response missing the token field
// is rejected rather than returning an empty token.
func TestPlayrunLogin_MockNoToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"email":"x@example.com"}`))
	}))
	defer srv.Close()

	a := &DefaultPlayrunAuthenticator{BaseURL: srv.URL}
	token, err := a.Login("x", "y")
	if err == nil {
		t.Fatalf("expected error for missing token, got token %q", token)
	}
	if !strings.Contains(err.Error(), "did not contain a token") {
		t.Errorf("error %q does not mention missing token", err)
	}
}

// TestPlayrunLogin_Integration performs a real login against the live Playrun
// API. It is skipped unless PLAYRUN_TEST_EMAIL and PLAYRUN_TEST_PASSWORD are
// both set in the environment. Run with:
//
//	PLAYRUN_TEST_EMAIL=... PLAYRUN_TEST_PASSWORD=... go test ./internal/auth/ -run Integration -v
func TestPlayrunLogin_Integration(t *testing.T) {
	email := os.Getenv("PLAYRUN_TEST_EMAIL")
	password := os.Getenv("PLAYRUN_TEST_PASSWORD")
	if email == "" || password == "" {
		t.Skip("set PLAYRUN_TEST_EMAIL and PLAYRUN_TEST_PASSWORD to run the live login test")
	}

	token, err := PlayrunLogin(email, password)
	if err != nil {
		t.Fatalf("PlayrunLogin failed: %v", err)
	}
	if token == "" {
		t.Fatal("PlayrunLogin returned an empty token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token does not look like a JWT (got %d parts): %q", len(parts), token)
	}
	t.Logf("login OK, got JWT (len=%d): %s...%s", len(token), token[:16], token[len(token)-8:])
}