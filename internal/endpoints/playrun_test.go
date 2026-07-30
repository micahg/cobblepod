package endpoints

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"cobblepod/internal/auth"
	"cobblepod/internal/state"
	statemock "cobblepod/internal/state/mock"

	"github.com/gin-gonic/gin"
)

func newPlayrunRouter(a auth.PlayrunAuthenticator, store state.CobblepodStateManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Mimic Auth0Middleware having populated the user_id in context.
	r.POST("/api/playrun/login", func(c *gin.Context) {
		c.Set("user_id", "test-user")
		HandlePlayrunLogin(a, store)(c)
	})
	r.POST("/api/playrun/logout", func(c *gin.Context) {
		c.Set("user_id", "test-user")
		HandlePlayrunLogout(store)(c)
	})
	r.GET("/api/playrun", func(c *gin.Context) {
		c.Set("user_id", "test-user")
		HandlePlayrunStatus(store)(c)
	})
	return r
}

// buildPlayrunJWT constructs an unsigned JWT with the given email and exp
// claims, purely for testing Playrun status parsing. It is not a valid
// token cryptographically, but ParsePlayrunToken does not verify the
// signature.
func buildPlayrunJWT(email string, exp int64) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(
		`{"email":"` + email + `","exp":` + strconv.FormatInt(exp, 10) + `}`))
	return header + "." + payload + ".signature"
}

func TestHandlePlayrunLogin_Success(t *testing.T) {
	const wantToken = "jwt-from-playrun"
	mock := &auth.MockPlayrunAuthenticator{Token: wantToken}
	store := statemock.NewMockCobblepodStateManager()
	router := newPlayrunRouter(mock, store)

	body, _ := json.Marshal(PlayrunLoginRequest{
		Email:    "user@example.com",
		Password: "secret",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/playrun/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp PlayrunLoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Errorf("response = %+v, want success=true", resp)
	}
	saved := store.GetSavedState("test-user")
	if saved == nil {
		t.Fatal("expected state to be saved for user")
	}
	if saved.PlayrunJWT != wantToken {
		t.Errorf("saved JWT = %q, want %q", saved.PlayrunJWT, wantToken)
	}
}

func TestHandlePlayrunLogin_PreservesLastRun(t *testing.T) {
	const wantToken = "jwt-from-playrun"
	mock := &auth.MockPlayrunAuthenticator{Token: wantToken}
	wantLastRun := time.Unix(1700000000, 0)
	store := statemock.NewMockCobblepodStateManager()
	store.SetState("test-user", &state.CobblepodState{LastRun: wantLastRun})
	router := newPlayrunRouter(mock, store)

	body, _ := json.Marshal(PlayrunLoginRequest{
		Email:    "user@example.com",
		Password: "secret",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/playrun/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	saved := store.GetSavedState("test-user")
	if saved == nil {
		t.Fatal("expected state to be saved for user")
	}
	if !saved.LastRun.Equal(wantLastRun) {
		t.Errorf("saved LastRun = %v, want %v (must be preserved)", saved.LastRun, wantLastRun)
	}
	if saved.PlayrunJWT != wantToken {
		t.Errorf("saved JWT = %q, want %q", saved.PlayrunJWT, wantToken)
	}
}

func TestHandlePlayrunLogin_AuthError(t *testing.T) {
	mock := &auth.MockPlayrunAuthenticator{Err: errors.New("invalid credentials")}
	store := statemock.NewMockCobblepodStateManager()
	router := newPlayrunRouter(mock, store)

	body, _ := json.Marshal(PlayrunLoginRequest{
		Email:    "user@example.com",
		Password: "wrong",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/playrun/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	var resp PlayrunLoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Success {
		t.Errorf("response = %+v, want success=false", resp)
	}
	if got := store.GetSavedState("test-user"); got != nil {
		t.Errorf("expected no state saved on auth failure, got %+v", got)
	}
}

func TestHandlePlayrunLogin_BadRequest(t *testing.T) {
	mock := &auth.MockPlayrunAuthenticator{}
	store := statemock.NewMockCobblepodStateManager()
	router := newPlayrunRouter(mock, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/playrun/login",
		bytes.NewReader([]byte(`{"email":"not-an-email"}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestHandlePlayrunLogout_Success(t *testing.T) {
	store := statemock.NewMockCobblepodStateManager()
	store.SetState("test-user", &state.CobblepodState{
		PlayrunJWT: "jwt-to-clear",
		LastRun:    time.Unix(1700000000, 0),
	})
	router := newPlayrunRouter(nil, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/playrun/logout", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp PlayrunLogoutResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Errorf("response = %+v, want success=true", resp)
	}
	saved := store.GetSavedState("test-user")
	if saved == nil {
		t.Fatal("expected state to be saved for user")
	}
	if saved.PlayrunJWT != "" {
		t.Errorf("saved JWT = %q, want empty", saved.PlayrunJWT)
	}
	wantLastRun := time.Unix(1700000000, 0)
	if !saved.LastRun.Equal(wantLastRun) {
		t.Errorf("saved LastRun = %v, want %v (must be preserved)", saved.LastRun, wantLastRun)
	}
}

func TestHandlePlayrunLogout_NoExistingState(t *testing.T) {
	store := statemock.NewMockCobblepodStateManager()
	router := newPlayrunRouter(nil, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/playrun/logout", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	saved := store.GetSavedState("test-user")
	if saved == nil {
		t.Fatal("expected state to be saved for user")
	}
	if saved.PlayrunJWT != "" {
		t.Errorf("saved JWT = %q, want empty", saved.PlayrunJWT)
	}
}

func TestHandlePlayrunStatus_LoggedIn(t *testing.T) {
	const wantEmail = "user@example.com"
	const wantExp = int64(1700000000)
	store := statemock.NewMockCobblepodStateManager()
	store.SetState("test-user", &state.CobblepodState{
		PlayrunJWT: buildPlayrunJWT(wantEmail, wantExp),
	})
	router := newPlayrunRouter(nil, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/playrun", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp PlayrunStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.LoggedIn {
		t.Errorf("loggedIn = false, want true")
	}
	if resp.Email != wantEmail {
		t.Errorf("email = %q, want %q", resp.Email, wantEmail)
	}
	if resp.ExpiresAt != wantExp {
		t.Errorf("expiresAt = %d, want %d", resp.ExpiresAt, wantExp)
	}
}

func TestHandlePlayrunStatus_NotLoggedIn(t *testing.T) {
	store := statemock.NewMockCobblepodStateManager()
	router := newPlayrunRouter(nil, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/playrun", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp PlayrunStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.LoggedIn {
		t.Errorf("loggedIn = true, want false (no JWT in state)")
	}
	if resp.Email != "" {
		t.Errorf("email = %q, want empty", resp.Email)
	}
	if resp.ExpiresAt != 0 {
		t.Errorf("expiresAt = %d, want 0", resp.ExpiresAt)
	}
}

func TestHandlePlayrunStatus_MalformedJWT(t *testing.T) {
	store := statemock.NewMockCobblepodStateManager()
	store.SetState("test-user", &state.CobblepodState{
		PlayrunJWT: "not-a-jwt",
	})
	router := newPlayrunRouter(nil, store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/playrun", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp PlayrunStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// A JWT is present so LoggedIn is true, but parsing fails so the
	// email/expiry fields are left empty.
	if !resp.LoggedIn {
		t.Errorf("loggedIn = false, want true (JWT present in state)")
	}
	if resp.Email != "" {
		t.Errorf("email = %q, want empty for unparseable JWT", resp.Email)
	}
	if resp.ExpiresAt != 0 {
		t.Errorf("expiresAt = %d, want 0 for unparseable JWT", resp.ExpiresAt)
	}
}
