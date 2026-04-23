package repository

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSQLiteDeleteReadingListItemsOlderThan(t *testing.T) {
	// Use in-memory SQLite for testing
	tmpDb, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	defer os.Remove(tmpDb.Name())
	tmpDb.Close()

	store, err := NewSQLiteStore(tmpDb.Name())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Create items with different ages
	old := ReadingListItem{URL: "http://old.com", Title: "Old"}
	recent := ReadingListItem{URL: "http://recent.com", Title: "Recent"}

	// Insert old item manually with past timestamp
	_, err = store.db.ExecContext(ctx,
		`INSERT INTO reading_list_items(url, title, created_at) VALUES(?, ?, ?)`,
		old.URL, old.Title, now.AddDate(0, 0, -45).Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("insert old item: %v", err)
	}

	// Insert recent item
	_, err = store.db.ExecContext(ctx,
		`INSERT INTO reading_list_items(url, title, created_at) VALUES(?, ?, ?)`,
		recent.URL, recent.Title, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("insert recent item: %v", err)
	}

	// Delete items older than 30 days
	deleted, err := store.DeleteReadingListItemsOlderThan(ctx, now.AddDate(0, 0, -30))
	if err != nil {
		t.Fatalf("delete old items: %v", err)
	}

	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	// Verify old item is gone
	items, err := store.ListReadingListItems(ctx)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("items count = %d, want 1 (only recent)", len(items))
	}

	if items[0].URL != recent.URL {
		t.Fatalf("remaining item URL = %q, want %q", items[0].URL, recent.URL)
	}
}

func TestSQLiteDeleteReadingListItemsNothingToDelete(t *testing.T) {
	tmpDb, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	defer os.Remove(tmpDb.Name())
	tmpDb.Close()

	store, err := NewSQLiteStore(tmpDb.Name())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// Insert a recent item
	_, err = store.db.ExecContext(ctx,
		`INSERT INTO reading_list_items(url, title, created_at) VALUES(?, ?, ?)`,
		"http://recent.com", "Recent", now.Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("insert item: %v", err)
	}

	// Try to delete items older than 30 days (should find nothing)
	deleted, err := store.DeleteReadingListItemsOlderThan(ctx, now.AddDate(0, 0, -30))
	if err != nil {
		t.Fatalf("delete old items: %v", err)
	}

	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
}

func TestMemoryStoreDeleteReadingListItemsOlderThan(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	now := time.Now().UTC()

	// Create items using public API
	oldInput := ReadingListItem{URL: "http://old.com", Title: "Old"}
	recentInput := ReadingListItem{URL: "http://recent.com", Title: "Recent"}

	_, err := store.CreateReadingListItem(ctx, oldInput)
	if err != nil {
		t.Fatalf("create old item: %v", err)
	}

	_, err = store.CreateReadingListItem(ctx, recentInput)
	if err != nil {
		t.Fatalf("create recent item: %v", err)
	}

	// Delete items with cutoff of 1 second in future - should delete all
	deleted, err := store.DeleteReadingListItemsOlderThan(ctx, now.Add(1*time.Second))
	if err != nil {
		t.Fatalf("delete items: %v", err)
	}

	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}

	// Verify all items are gone
	items, err := store.ListReadingListItems(ctx)
	if err != nil {
		t.Fatalf("list items: %v", err)
	}

	if len(items) != 0 {
		t.Fatalf("items count = %d, want 0", len(items))
	}
}
