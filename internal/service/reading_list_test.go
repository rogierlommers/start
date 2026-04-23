package service

import (
	"context"
	"testing"

	"start/internal/config"
	"start/internal/repository"
)

func TestCleanupReadingListDisabledWhenZero(t *testing.T) {
	store := repository.NewMemoryStore()
	cfg := config.Config{ReadingListCleanupDays: 0}
	svc := &Service{
		store: store,
		cfg:   cfg,
		done:  make(chan struct{}),
	}

	ctx := context.Background()

	// With 0 days, cleanup should return 0 without attempting deletion
	deleted, err := svc.cleanupReadingListOlderThan(ctx, 0)
	if err != nil {
		t.Fatalf("cleanup with 0 days: %v", err)
	}

	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0 (disabled)", deleted)
	}
}

func TestCleanupReadingListContextCancelled(t *testing.T) {
	store := repository.NewMemoryStore()
	cfg := config.Config{ReadingListCleanupDays: 30}
	svc := &Service{
		store: store,
		cfg:   cfg,
		done:  make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Cleanup with cancelled context should fail
	_, err := svc.cleanupReadingListOlderThan(ctx, 30)
	if err == nil {
		t.Fatalf("cleanup with cancelled context should error, got nil")
	}
}
