package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cobblepod/internal/config"
	"cobblepod/internal/processor"
)

// cobbleWorker handles processing job requests
func cobbleWorker(ctx context.Context, processingJobs <-chan struct{}) {
	proc, err := processor.NewProcessor(ctx)
	if err != nil {
		slog.Error("Failed to create processor", "error", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("Cobble worker shutting down")
			return
		case _, ok := <-processingJobs:
			if !ok {
				// Channel closed
				slog.Info("Processing jobs channel closed, worker exiting")
				return
			}

			if err := proc.Run(ctx); err != nil {
				if err == context.Canceled {
					slog.Info("Processing cancelled")
					return
				} else {
					slog.Error("Error during processing", "error", err)
				}
			}
		}
	}
}

func main() {
	// Initialize structured logging with JSON handler
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(jsonHandler))

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a ticker for the polling interval
	ticker := time.NewTicker(time.Duration(config.PollInterval) * time.Second)
	defer ticker.Stop()

	// Channel for processing jobs - buffered to allow one pending job
	processingJobs := make(chan struct{})

	// Start the processing worker
	go cobbleWorker(ctx, processingJobs)

	slog.Info("Starting cobblepod", "poll_interval_seconds", config.PollInterval)
	processingJobs <- struct{}{}

	for {
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, shutting down")
			return
		case sig := <-sigChan:
			slog.Info("Received signal, shutting down gracefully", "signal", sig)
			cancel()
			return
		case <-ticker.C:
			select {
			case processingJobs <- struct{}{}:
			default:
				slog.Warn("Skipping processing - queue is full")
			}
		}
	}
}
