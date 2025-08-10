package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cobblepod/pkg/audio"

	"github.com/gin-gonic/gin"
)

// Server represents the API server
type Server struct {
	router      *gin.Engine
	port        string
	processLock chan struct{}
}

// NewServer creates a new API server
func NewServer(port string) *Server {
	if port == "" {
		port = "8080"
	}

	// Set Gin mode based on environment
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	server := &Server{
		router:      router,
		port:        port,
		processLock: make(chan struct{}, 1),
	}

	server.setupRoutes()
	return server
}

// setupRoutes configures all the API routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/", s.healthCheck) // Also respond to root path

	// Process endpoint
	s.router.POST("/process", s.processHandler)
}

// healthCheck handler for the health check endpoint
func (s *Server) healthCheck(c *gin.Context) {
	processor := audio.NewProcessor()

	// Check if FFmpeg is available
	ffmpegErr := processor.CheckFFmpegAvailable()

	var status string
	var httpStatus int

	if ffmpegErr != nil {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	} else {
		status = "healthy"
		httpStatus = http.StatusOK
	}

	// Build ffmpeg check result
	ffmpegCheck := gin.H{
		"available": ffmpegErr == nil,
	}

	// Only include error field if there's an actual error
	if ffmpegErr != nil {
		ffmpegCheck["error"] = ffmpegErr.Error()
	}

	response := gin.H{
		"status":    status,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"service":   "cobblepod",
		"version":   "1.0.0",
		"checks": gin.H{
			"ffmpeg": ffmpegCheck,
		},
	}
	c.JSON(httpStatus, response)
}

// processHandler handles the /process endpoint
func (s *Server) processHandler(c *gin.Context) {
	// Try to acquire the lock with a non-blocking attempt
	select {
	case s.processLock <- struct{}{}:
		// Successfully acquired the lock
		defer func() { <-s.processLock }() // Release the lock when done
	default:
		// Lock is already taken
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":     "Processing already in progress",
			"message":   "Another processing operation is currently running. Please try again later.",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   "Processing completed successfully",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Start starts the API server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%s", s.port)
	log.Printf("Starting API server on port %s", s.port)

	srv := &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Give it 30 seconds to shutdown gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
		return err
	}

	log.Println("Server exited")
	return nil
}

// StartBackground starts the server in the background and returns immediately
func (s *Server) StartBackground() {
	addr := fmt.Sprintf(":%s", s.port)
	log.Printf("Starting API server in background on port %s", s.port)

	go func() {
		if err := s.router.Run(addr); err != nil {
			log.Printf("Failed to start background server: %v", err)
		}
	}()

	// Give the server a moment to start up
	time.Sleep(100 * time.Millisecond)
}
