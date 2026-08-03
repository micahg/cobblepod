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
	Error   string `json:"error,omitempty"`
}

// PlayrunLogoutResponse is the response for POST /api/playrun/logout.
type PlayrunLogoutResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// PlayrunStatusResponse is the response for GET /api/playrun. It reports the
// current Playrun connection status derived from the persisted Playrun JWT:
// whether a token is present (LoggedIn), and when known, its expiry and the
// associated account email.
type PlayrunStatusResponse struct {
	LoggedIn  bool   `json:"loggedIn"`
	Email     string `json:"email,omitempty"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
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
				// Don't fail the request: the login itself succeeded. We'll retry on next login.
			} else {
				slog.Info("Persisted playrun JWT", "user_id", userID)
			}
		} else {
			slog.Warn("No state manager configured, playrun JWT not persisted", "user_id", userID)
		}

		c.JSON(http.StatusOK, PlayrunLoginResponse{
			Success: true,
		})
	}
}

// HandlePlayrunLogout logs a user out of Playrun by wiping the persisted
// Playrun JWT from this user's application state. It does not call any
// Playrun API logout endpoint; it only clears the locally cached token so
// subsequent Playrun API calls no longer reuse it. Other fields of the
// CobblepodState (e.g. LastRun) are preserved.
// @Summary      Playrun logout
// @Description  Clears the persisted Playrun JWT for the authenticated user
// @Tags         playrun
// @Produce      json
// @Success      200   {object}  PlayrunLogoutResponse
// @Failure      401   {object}  PlayrunLogoutResponse
// @Router       /playrun/logout [post]
func HandlePlayrunLogout(stateManager state.CobblepodStateManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := GetUserID(c)
		if err != nil {
			slog.Error("Failed to get user ID from context", "error", err)
			c.JSON(http.StatusUnauthorized, PlayrunLogoutResponse{
				Success: false,
				Error:   "Unauthorized",
			})
			return
		}

		if stateManager != nil {
			existing, err := stateManager.GetState(userID)
			if err != nil {
				if !errors.Is(err, redis.Nil) {
					slog.Error("Failed to load existing state for playrun logout", "error", err, "user_id", userID)
				}
				existing = &state.CobblepodState{}
			}
			existing.PlayrunJWT = ""
			if err := stateManager.SaveState(userID, existing); err != nil {
				slog.Error("Failed to clear playrun JWT", "error", err, "user_id", userID)
				c.JSON(http.StatusInternalServerError, PlayrunLogoutResponse{
					Success: false,
					Error:   "Failed to clear playrun JWT",
				})
				return
			}
			slog.Info("Cleared playrun JWT", "user_id", userID)
		} else {
			slog.Warn("No state manager configured, playrun JWT not cleared", "user_id", userID)
		}

		c.JSON(http.StatusOK, PlayrunLogoutResponse{
			Success: true,
		})
	}
}

// HandlePlayrunStatus returns the current Playrun connection status for the
// authenticated user, derived from the persisted Playrun JWT. It reports
// whether a token is present (LoggedIn), and when the token can be parsed,
// its expiry (exp claim) and the associated account email. It never fails
// other than on missing authentication: a missing or malformed token simply
// yields LoggedIn=false with empty email/expiry fields.
// @Summary      Playrun status
// @Description  Returns whether the user is logged in to Playrun, plus the token expiry and account email when available
// @Tags         playrun
// @Produce      json
// @Success      200  {object}  PlayrunStatusResponse
// @Failure      401  {object}  PlayrunStatusResponse
// @Router       /playrun [get]
func HandlePlayrunStatus(stateManager state.CobblepodStateManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := GetUserID(c)
		if err != nil {
			slog.Error("Failed to get user ID from context", "error", err)
			c.JSON(http.StatusUnauthorized, PlayrunStatusResponse{})
			return
		}

		resp := PlayrunStatusResponse{}
		if stateManager != nil {
			existing, err := stateManager.GetState(userID)
			if err != nil {
				if !errors.Is(err, redis.Nil) {
					slog.Error("Failed to load state for playrun status", "error", err, "user_id", userID)
				}
				existing = &state.CobblepodState{}
			}
			if existing.PlayrunJWT != "" {
				resp.LoggedIn = true
				claims, err := auth.ParsePlayrunToken(existing.PlayrunJWT)
				if err != nil {
					slog.Warn("Failed to parse playrun JWT", "error", err, "user_id", userID)
				} else {
					resp.Email = claims.Email
					resp.ExpiresAt = claims.Exp
				}
			}
		} else {
			slog.Warn("No state manager configured, playrun status unavailable", "user_id", userID)
		}

		c.JSON(http.StatusOK, resp)
	}
}
