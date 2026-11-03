package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteCategoryLifecycle(t *testing.T) {
	store := mustNewSQLiteStore(t)
	ctx := context.Background()

	created, err := store.CreateCategory(ctx, Category{Name: "  Work  "})
	if err != nil {
		t.Fatalf("CreateCategory() error = %v", err)
	}
	if created.Name != "Work" {
		t.Fatalf("CreateCategory().Name = %q, want %q", created.Name, "Work")
	}

	_, err = store.CreateCategory(ctx, Category{Name: "work"})
	if !errors.Is(err, ErrCategoryAlreadyExists) {
		t.Fatalf("CreateCategory(duplicate) error = %v, want %v", err, ErrCategoryAlreadyExists)
	}

	categories, err := store.ListCategories(ctx)
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}
	if len(categories) != 1 || categories[0].ID != created.ID {
		t.Fatalf("ListCategories() = %+v", categories)
	}
}

func TestSQLiteBookmarkLifecycleAndErrors(t *testing.T) {
	store := mustNewSQLiteStore(t)
	ctx := context.Background()

	cat, err := store.CreateCategory(ctx, Category{Name: "General"})
	if err != nil {
		t.Fatalf("CreateCategory() error = %v", err)
	}

	_, err = store.CreateBookmark(ctx, Bookmark{URL: "https://example.com", CategoryID: 999})
	if !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("CreateBookmark(missing category) error = %v, want %v", err, ErrCategoryNotFound)
	}

	bm, err := store.CreateBookmark(ctx, Bookmark{URL: "https://example.com", Title: "Title", Tag: "alpha", CategoryID: cat.ID})
	if err != nil {
		t.Fatalf("CreateBookmark() error = %v", err)
	}
	if bm.Tag != "alpha" {
		t.Fatalf("CreateBookmark().Tag = %q", bm.Tag)
	}

	_, err = store.CreateBookmark(ctx, Bookmark{URL: "https://example.com", CategoryID: cat.ID})
	if !errors.Is(err, ErrBookmarkAlreadyExists) {
		t.Fatalf("CreateBookmark(duplicate URL) error = %v, want %v", err, ErrBookmarkAlreadyExists)
	}

	bm.Hidden = true
	bm.URL = "https://example.org"
	bm.Title = "Updated"
	bm.Tag = "updated"
	updated, err := store.UpdateBookmark(ctx, bm)
	if err != nil {
		t.Fatalf("UpdateBookmark() error = %v", err)
	}
	if !updated.Hidden || updated.URL != "https://example.org" || updated.Tag != "updated" {
		t.Fatalf("UpdateBookmark() = %+v", updated)
	}

	_, err = store.UpdateBookmark(ctx, Bookmark{ID: 999, URL: "https://none.example", CategoryID: cat.ID})
	if !errors.Is(err, ErrBookmarkNotFound) {
		t.Fatalf("UpdateBookmark(missing) error = %v, want %v", err, ErrBookmarkNotFound)
	}

	listed, err := store.ListBookmarks(ctx, false)
	if err != nil {
		t.Fatalf("ListBookmarks(false) error = %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("ListBookmarks(false) len = %d, want 0", len(listed))
	}

	listed, err = store.ListBookmarks(ctx, true)
	if err != nil {
		t.Fatalf("ListBookmarks(true) error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != bm.ID {
		t.Fatalf("ListBookmarks(true) = %+v", listed)
	}
	if listed[0].Tag != "updated" {
		t.Fatalf("ListBookmarks(true).Tag = %q", listed[0].Tag)
	}

	if err := store.ReorderBookmarks(ctx, []int64{}); !errors.Is(err, ErrInvalidBookmarkOrder) {
		t.Fatalf("ReorderBookmarks(empty) error = %v, want %v", err, ErrInvalidBookmarkOrder)
	}
	if err := store.ReorderBookmarks(ctx, []int64{999}); !errors.Is(err, ErrBookmarkNotFound) {
		t.Fatalf("ReorderBookmarks(missing bookmark) error = %v, want %v", err, ErrBookmarkNotFound)
	}
	if err := store.ReorderBookmarks(ctx, []int64{bm.ID}); err != nil {
		t.Fatalf("ReorderBookmarks(valid) error = %v", err)
	}

	if err := store.DeleteBookmark(ctx, bm.ID); err != nil {
		t.Fatalf("DeleteBookmark() error = %v", err)
	}
	if err := store.DeleteBookmark(ctx, bm.ID); !errors.Is(err, ErrBookmarkNotFound) {
		t.Fatalf("DeleteBookmark(missing) error = %v, want %v", err, ErrBookmarkNotFound)
	}
}

func TestSQLiteBookmarkCSVPersistsIndependently(t *testing.T) {
	store := mustNewSQLiteStore(t)
	ctx := context.Background()

	content := "\"tag1 tag2\",\"https://www.google.com\"\n\"reference\",\"https://example.com\""
	if err := store.SaveBookmarkCSV(ctx, content); err != nil {
		t.Fatalf("SaveBookmarkCSV() error = %v", err)
	}

	got, err := store.GetBookmarkCSV(ctx)
	if err != nil {
		t.Fatalf("GetBookmarkCSV() error = %v", err)
	}
	if got != content {
		t.Fatalf("GetBookmarkCSV() = %q, want %q", got, content)
	}

	if err := store.SaveBookmarkCSV(ctx, "updated"); err != nil {
		t.Fatalf("SaveBookmarkCSV(update) error = %v", err)
	}
	got, err = store.GetBookmarkCSV(ctx)
	if err != nil {
		t.Fatalf("GetBookmarkCSV(updated) error = %v", err)
	}
	if got != "updated" {
		t.Fatalf("GetBookmarkCSV(updated) = %q, want %q", got, "updated")
	}

	bookmarks, err := store.ListBookmarks(ctx, true)
	if err != nil {
		t.Fatalf("ListBookmarks() error = %v", err)
	}
	if len(bookmarks) != 0 {
		t.Fatalf("ListBookmarks() len = %d, want 0", len(bookmarks))
	}
}

func TestSQLiteReadingListLifecycle(t *testing.T) {
	store := mustNewSQLiteStore(t)
	ctx := context.Background()

	first, err := store.CreateReadingListItem(ctx, ReadingListItem{URL: "https://a.example", Title: "A"})
	if err != nil {
		t.Fatalf("CreateReadingListItem(first) error = %v", err)
	}
	second, err := store.CreateReadingListItem(ctx, ReadingListItem{URL: "https://b.example", Title: "B"})
	if err != nil {
		t.Fatalf("CreateReadingListItem(second) error = %v", err)
	}

	items, err := store.ListReadingListItems(ctx)
	if err != nil {
		t.Fatalf("ListReadingListItems() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("ListReadingListItems() len = %d, want 2", len(items))
	}

	if items[0].CreatedAt.Before(items[1].CreatedAt) {
		t.Fatalf("items not sorted desc by CreatedAt: %+v", items)
	}

	deleted, err := store.DeleteReadingListItemsOlderThan(ctx, second.CreatedAt.Add(-time.Second))
	if err != nil {
		t.Fatalf("DeleteReadingListItemsOlderThan() error = %v", err)
	}
	if deleted != 0 {
		t.Fatalf("DeleteReadingListItemsOlderThan() deleted = %d, want 0", deleted)
	}

	deleted, err = store.DeleteReadingListItemsOlderThan(ctx, first.CreatedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("DeleteReadingListItemsOlderThan(delete old) error = %v", err)
	}
	if deleted < 1 {
		t.Fatalf("DeleteReadingListItemsOlderThan() deleted = %d, want at least 1", deleted)
	}
}

func TestSQLiteReadingListRequestDiagnostics(t *testing.T) {
	store := mustNewSQLiteStore(t)
	ctx := context.Background()
	want := ReadingListItem{URL: "https://example.com/", ClientIP: "192.0.2.10", UserAgent: "Test/1.0", Referer: "https://example.com/article"}
	created, err := store.CreateReadingListItem(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.ListReadingListItems(ctx)
	if err != nil || len(items) != 1 {
		t.Fatalf("ListReadingListItems() = (%+v, %v)", items, err)
	}
	for _, got := range []ReadingListItem{created, items[0]} {
		if got.ClientIP != want.ClientIP || got.UserAgent != want.UserAgent || got.Referer != want.Referer {
			t.Fatalf("request diagnostics not preserved: %+v", got)
		}
	}
}

func TestSQLiteReadingListDiagnosticsMigrationPreservesExistingRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reading-list-v7.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL)`,
		`INSERT INTO schema_migrations(version, name, applied_at) VALUES
			(1, 'initial_schema', ''), (2, 'add_bookmark_tags', ''), (3, 'add_bookmark_csv', ''),
			(4, 'add_bank_connection', ''), (5, 'support_multiple_bank_accounts', ''),
			(6, 'track_bank_transaction_consent', ''), (7, 'add_bank_account_aliases', '')`,
		`CREATE TABLE reading_list_items (id INTEGER PRIMARY KEY AUTOINCREMENT, url TEXT NOT NULL, title TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL)`,
		`INSERT INTO reading_list_items(url, title, created_at) VALUES('https://old.example/', 'Old', '2026-10-01T12:00:00Z')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("prepare v7 database: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	items, err := store.ListReadingListItems(context.Background())
	if err != nil || len(items) != 1 || items[0].URL != "https://old.example/" || items[0].ClientIP != "" || items[0].UserAgent != "" || items[0].Referer != "" {
		t.Fatalf("migrated items = (%+v, %v)", items, err)
	}
}

func TestParseSQLiteTimeAndBoolToInt(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	parsed, err := parseSQLiteTime(now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("parseSQLiteTime(RFC3339Nano) error = %v", err)
	}
	if !parsed.Equal(now) {
		t.Fatalf("parseSQLiteTime(RFC3339Nano) = %v, want %v", parsed, now)
	}

	parsed, err = parseSQLiteTime(now.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("parseSQLiteTime(RFC3339) error = %v", err)
	}
	if parsed.IsZero() {
		t.Fatal("parseSQLiteTime(RFC3339) returned zero time")
	}

	if _, err := parseSQLiteTime("not-a-time"); err == nil {
		t.Fatal("parseSQLiteTime(invalid) error = nil, want error")
	}

	if got := boolToInt(true); got != 1 {
		t.Fatalf("boolToInt(true) = %d, want 1", got)
	}
	if got := boolToInt(false); got != 0 {
		t.Fatalf("boolToInt(false) = %d, want 0", got)
	}
}

func mustNewSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()

	tmpDb, err := os.CreateTemp("", "repo-more-*.db")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	if err := tmpDb.Close(); err != nil {
		t.Fatalf("temp db close error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(tmpDb.Name())
	})

	store, err := NewSQLiteStore(tmpDb.Name())
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	return store
}
