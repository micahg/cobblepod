package endpoints

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
	return r
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
