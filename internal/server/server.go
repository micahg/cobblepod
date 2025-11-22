package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"cobblepod/internal/endpoints"
	"cobblepod/internal/queue"

	"github.com/gin-gonic/gin"
)

// Server wraps the HTTP server
type Server struct {
	httpServer *http.Server
	router     *gin.Engine
	queue      *queue.Queue
}

// NewServer creates a new HTTP server instance
func NewServer(port string) (*Server, error) {
	// Set Gin mode based on environment
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize queue
	ctx := context.Background()
	jobQueue, err := queue.NewQueue(ctx)
	if err != nil {
		return nil, err
	}

	router := gin.New()

	// Add essential middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Add CORS middleware for frontend communication
	router.Use(corsMiddleware())

	// Setup all routes with dependencies
	endpoints.SetupRoutes(router, jobQueue)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		httpServer: httpServer,
		router:     router,
		queue:      jobQueue,
	}, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	slog.Info("Starting HTTP server", "address", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("Shutting down HTTP server")

	// Close queue connection
	if s.queue != nil {
		if err := s.queue.Close(); err != nil {
			slog.Error("Failed to close queue", "error", err)
		}
	}

	return s.httpServer.Shutdown(ctx)
}

// corsMiddleware handles CORS for the frontend
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*") // In production, specify your frontend domain
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
