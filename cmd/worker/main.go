package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"time"

	"cobblepod/internal/processor"
	"cobblepod/internal/queue"
)

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

	// Initialize job queue
	jobQueue, err := queue.NewQueue(ctx)
	if err != nil {
		slog.Error("Failed to connect to job queue", "error", err)
		os.Exit(1)
	}
	defer jobQueue.Close()

	// Initialize processor
	proc, err := processor.NewProcessor(ctx, jobQueue)
	if err != nil {
		slog.Error("Failed to create processor", "error", err)
		os.Exit(1)
	}

	// Start cleanup ticker (every hour)
	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer cleanupTicker.Stop()

	slog.Info("Worker started, waiting for jobs...")

	// Main worker loop
	for {
		select {
		case <-ctx.Done():
			slog.Info("Context cancelled, shutting down")
			return
		case sig := <-sigChan:
			slog.Info("Received signal, shutting down gracefully", "signal", sig)
			cancel()
			return
		case <-cleanupTicker.C:
			slog.Info("Running scheduled cleanup")
			if err := jobQueue.CleanupExpiredJobs(ctx); err != nil {
				slog.Error("Failed to cleanup expired jobs", "error", err)
			}
		default:
			// Dequeue job (blocks until job available or timeout)
			job, err := jobQueue.Dequeue(ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				slog.Error("Failed to dequeue job", "error", err)
				continue
			}

			if job == nil {
				// Timeout, no job available - loop continues
				continue
			}

			// Try to mark user as running
			started, err := jobQueue.StartJob(ctx, job.UserID, job.ID)
			if err != nil {
				slog.Error("Failed to mark job as started", "error", err, "job_id", job.ID)
				// Fail the job due to system error (don't hold lock)
				jobQueue.FailJob(ctx, job, "Failed to acquire user lock")
				continue
			}

			if !started {
				// User already has a running job - fail this one (don't hold lock)
				slog.Warn("User already has running job, failing new job",
					"user_id", job.UserID, "job_id", job.ID)
				jobQueue.FailJob(ctx, job, "User already has a job being processed")
				continue
			}

			// Process the job - use a function to ensure defer runs
			func() {
				// Always release the user lock when done
				defer func() {
					if err := jobQueue.CompleteJob(ctx, job.UserID, job.ID); err != nil {
						slog.Error("Failed to release user lock", "error", err, "user_id", job.UserID)
					}
				}()

				slog.Info("Processing job", "job_id", job.ID, "user_id", job.UserID, "file_id", job.FileID)

				if err := proc.Run(ctx, job); err != nil {
					slog.Error("Job processing failed", "error", err, "job_id", job.ID)
					jobQueue.FailJob(ctx, job, err.Error())
				} else {
					slog.Info("Job completed successfully", "job_id", job.ID)
				}
			}()
		}
	}
}
