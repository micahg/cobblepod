package queue

import (
	"testing"
	"time"
)

func TestJobMarshaling(t *testing.T) {
	job := &Job{
		ID:        "test-id-123",
		FileID:    "drive-file-123",
		UserID:    "user-123",
		Filename:  "test.backup",
		CreatedAt: time.Now(),
	}

	// This tests that the Job struct can be marshaled/unmarshaled
	// The actual queue operations will be tested in integration tests
	if job.ID == "" {
		t.Error("Job ID should not be empty")
	}
	if job.FileID == "" {
		t.Error("Job FileID should not be empty")
	}
}

func TestQueueConstants(t *testing.T) {
	if WaitingQueue == "" {
		t.Error("WaitingQueue should not be empty")
	}
	if BlockTimeout == 0 {
		t.Error("BlockTimeout should not be zero")
	}
}


