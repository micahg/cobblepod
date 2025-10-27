package endpoints

import (
	"github.com/gin-gonic/gin"
)

// SetupRoutes configures all API routes
func SetupRoutes(r *gin.Engine) {
	// API group with common middleware
	api := r.Group("/api")
	{
		// Health check endpoint
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":  "healthy",
				"service": "cobblepod",
			})
		})

		// OAuth routes
		auth := api.Group("/auth")
		{
			auth.POST("/callback", HandleOAuthCallback)
			auth.GET("/callback", HandleOAuthCallback) // Auth0 typically uses GET
		}

		// Backup routes (protected)
		backup := api.Group("/backup")
		backup.Use(Auth0Middleware()) // Require authentication
		{
			backup.POST("/upload", HandleBackupUpload)
		}
	}
}
