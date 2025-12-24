package endpoints

import (
	"cobblepod/internal/queue"

	_ "cobblepod/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupRoutes configures all API routes
func SetupRoutes(r *gin.Engine, jobQueue *queue.Queue) {
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

		// Job routes (protected)
		jobs := api.Group("/jobs")
		jobs.Use(Auth0Middleware())
		{
			jobs.GET("", HandleGetJobs(jobQueue))
		}
	}
}
