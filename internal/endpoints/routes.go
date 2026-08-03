package endpoints

import (
	"cobblepod/internal/queue"
	"cobblepod/internal/state"

	_ "cobblepod/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupRoutes configures all API routes
func SetupRoutes(r *gin.Engine, jobQueue *queue.Queue, stateManager state.CobblepodStateManager) {
	// API group with common middleware
	api := r.Group("/api")
	{
		// Swagger endpoint
		api.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

		// Health check endpoint
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":  "healthy",
				"service": "cobblepod",
			})
		})

		// Backup routes (protected)
		backup := api.Group("/backup")
		backup.Use(Auth0Middleware()) // Require authentication
		{
			backup.POST("/upload", HandleBackupUpload(jobQueue))
		}

		// RSS routes (protected)
		api.GET("/rss", Auth0Middleware(), HandleGetRSS())

		// Job routes (protected)
		jobs := api.Group("/jobs")
		jobs.Use(Auth0Middleware())
		{
			jobs.GET("", HandleGetJobs(jobQueue))
			jobs.GET("/:id/items", HandleGetJobItems(jobQueue))
			jobs.DELETE("/:id", HandleCancelJob(jobQueue))
		}

		// Playrun routes (protected: persists per-user state, which requires
		// knowing the Auth0 user identity to key the state store).
		playrun := api.Group("/playrun")
		playrun.Use(Auth0Middleware())
		{
			playrun.GET("", HandlePlayrunStatus(stateManager))
			playrun.POST("/login", HandlePlayrunLogin(nil, stateManager))
			playrun.POST("/logout", HandlePlayrunLogout(stateManager))
		}
	}
}
