package endpoints

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// OAuthCallbackRequest represents the Auth0 callback payload
type OAuthCallbackRequest struct {
	Code  string `json:"code" form:"code"`
	State string `json:"state" form:"state"`
	Error string `json:"error" form:"error"`
}

// OAuthCallbackResponse represents the callback response
type OAuthCallbackResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HandleOAuthCallback processes Auth0 OAuth callback
func HandleOAuthCallback(c *gin.Context) {
	var req OAuthCallbackRequest

	// Bind query parameters or form data
	if err := c.ShouldBind(&req); err != nil {
		slog.Error("Failed to bind OAuth callback request", "error", err)
		c.JSON(http.StatusBadRequest, OAuthCallbackResponse{
			Success: false,
			Error:   "Invalid request parameters",
		})
		return
	}

	// Handle OAuth error from Auth0
	if req.Error != "" {
		slog.Warn("OAuth callback received error", "error", req.Error, "state", req.State)
		c.JSON(http.StatusBadRequest, OAuthCallbackResponse{
			Success: false,
			Error:   req.Error,
		})
		return
	}

	// Validate required parameters
	if req.Code == "" {
		slog.Error("OAuth callback missing authorization code")
		c.JSON(http.StatusBadRequest, OAuthCallbackResponse{
			Success: false,
			Error:   "Missing authorization code",
		})
		return
	}

	// Log successful callback (in production, you'd exchange the code for tokens here)
	slog.Info("OAuth callback received", "code_length", len(req.Code), "state", req.State)

	// TODO: Exchange authorization code for access token
	// This is where you would:
	// 1. Validate the state parameter (CSRF protection)
	// 2. Exchange the code for an access token with Auth0
	// 3. Validate the JWT token
	// 4. Create/update user session
	// 5. Redirect to appropriate page or return token

	c.JSON(http.StatusOK, OAuthCallbackResponse{
		Success: true,
		Message: "OAuth callback processed successfully",
	})
}
