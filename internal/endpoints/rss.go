package endpoints

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"cobblepod/internal/auth"
	"cobblepod/internal/podcast"
	"cobblepod/internal/storage"

	"github.com/gin-gonic/gin"
)

// RSSProcessor defines the interface for RSS operations
type RSSProcessor interface {
	GetRSSFeedID(ctx context.Context) string
}

// Dependencies to be swapped in tests
var (
	getGoogleAccessTokenFunc = auth.GetGoogleAccessToken
	newStorageServiceFunc    = func(ctx context.Context, token string) (storage.Storage, error) {
		return storage.NewServiceWithToken(ctx, token)
	}
	newRSSProcessorFunc = func(channelTitle string, driveService storage.Storage) RSSProcessor {
		return podcast.NewRSSProcessor(channelTitle, driveService)
	}
)

// RSSResponse represents the RSS download URL response
type RSSResponse struct {
	URL   string `json:"url,omitempty"`
	Error string `json:"error,omitempty"`
}

// HandleGetRSS returns the download URL for the user's RSS feed
// @Summary      Get RSS feed URL
// @Description  Returns the download URL for the user's RSS feed
// @Tags         rss
// @Produce      json
// @Success      200  {object}  RSSResponse
// @Failure      401  {object}  RSSResponse
// @Failure      404  {object}  RSSResponse
// @Router       /rss [get]
func HandleGetRSS() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by Auth0Middleware)
		userID, err := GetUserID(c)
		if err != nil {
			slog.Error("Failed to get user ID from context", "error", err)
			c.JSON(http.StatusUnauthorized, RSSResponse{
				Error: "Unauthorized",
			})
			return
		}

		// Exchange Auth0 token for Google access token
		googleToken, err := getGoogleAccessTokenFunc(c.Request.Context(), userID)
		if err != nil {
			slog.Error("Failed to get Google access token", "error", err, "user_id", userID)
			c.JSON(http.StatusFailedDependency, RSSResponse{
				Error: fmt.Sprintf("Failed to authenticate with Google: %v", err),
			})
			return
		}

		// Create Google Drive service with user's Google access token
		driveService, err := newStorageServiceFunc(c.Request.Context(), googleToken)
		if err != nil {
			slog.Error("Failed to create Drive service", "error", err)
			c.JSON(http.StatusInternalServerError, RSSResponse{
				Error: "Failed to initialize storage service",
			})
			return
		}

		// Create RSS processor with empty channel title as we're only reading
		rssProcessor := newRSSProcessorFunc("", driveService)

		// Get RSS feed ID
		rssFileID := rssProcessor.GetRSSFeedID(c.Request.Context())
		if rssFileID == "" {
			c.JSON(http.StatusNotFound, RSSResponse{
				Error: "RSS feed not found. Please process a backup first.",
			})
			return
		}

		// Generate download URL
		downloadURL := driveService.GenerateDownloadURL(rssFileID)

		c.JSON(http.StatusOK, RSSResponse{
			URL: downloadURL,
		})
	}
}
