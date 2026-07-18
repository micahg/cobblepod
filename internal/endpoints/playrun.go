package endpoints

import (
	"errors"
	"log/slog"
	"net/http"

	"cobblepod/internal/auth"
	"cobblepod/internal/state"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// defaultPlayrunAuthenticator is the package-level authenticator used when the
// caller does not inject one. It points at the production Playrun endpoint.
var defaultPlayrunAuthenticator auth.PlayrunAuthenticator = &auth.DefaultPlayrunAuthenticator{}

// PlayrunLoginRequest is the body for POST /api/playrun/login.
type PlayrunLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// PlayrunLoginResponse is the response for POST /api/playrun/login.
type PlayrunLoginResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HandlePlayrunLogin logs a user in to Playrun and returns the Playrun JWT.
// @Summary      Playrun login
// @Description  Exchanges Playrun credentials for a Playrun JWT
// @Tags         playrun
// @Accept       json
// @Produce      json
// @Param        body  body      PlayrunLoginRequest  true  "Playrun credentials"
// @Success      200   {object}  PlayrunLoginResponse
// @Failure      400   {object}  PlayrunLoginResponse
// @Failure      401   {object}  PlayrunLoginResponse
// @Router       /playrun/login [post]
func HandlePlayrunLogin(authenticator auth.PlayrunAuthenticator, stateManager state.CobblepodStateManager) gin.HandlerFunc {
	if authenticator == nil {
		authenticator = defaultPlayrunAuthenticator
	}
	return func(c *gin.Context) {
		var req PlayrunLoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.Warn("Invalid playrun login request", "error", err)
			c.JSON(http.StatusBadRequest, PlayrunLoginResponse{
				Success: false,
				Error:   "Invalid request: email and password are required",
			})
			return
		}

		userID, err := GetUserID(c)
		if err != nil {
			slog.Error("Failed to get user ID from context", "error", err)
			c.JSON(http.StatusUnauthorized, PlayrunLoginResponse{
				Success: false,
				Error:   "Unauthorized",
			})
			return
		}

		token, err := authenticator.Login(req.Email, req.Password)
		if err != nil {
			slog.Error("Playrun login failed", "error", err, "email", req.Email)
			c.JSON(http.StatusUnauthorized, PlayrunLoginResponse{
				Success: false,
				Error:   "Playrun login failed",
			})
			return
		}

		// Persist the Playrun JWT for this user so subsequent Playrun API
		// calls (podcast/playlist reconciliation) can reuse it without
		// requiring the user's Playrun password on every request.
if stateManager != nil {
		existing, err := stateManager.GetState(userID)
		if err != nil {
			if !errors.Is(err, redis.Nil) {
				slog.Error("Failed to load existing state for playrun JWT", "error", err, "user_id", userID)
			}
			// Either no existing state yet or a transient error: start
			// from a fresh state so we don't block the login.
			existing = &state.CobblepodState{}
		}
		existing.PlayrunJWT = token
		if err := stateManager.SaveState(userID, existing); err != nil {
			slog.Error("Failed to persist playrun JWT", "error", err, "user_id", userID)
			// Don't fail the request: the login itself succeeded and
			// the caller received the token. We'll retry on next login.
		} else {
			slog.Info("Persisted playrun JWT", "user_id", userID)
		}
	} else {
		slog.Warn("No state manager configured, playrun JWT not persisted", "user_id", userID)
	}

		c.JSON(http.StatusOK, PlayrunLoginResponse{
			Success: true,
			Token:   token,
		})
	}
}