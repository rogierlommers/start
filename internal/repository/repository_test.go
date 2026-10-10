package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
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

func TestSQLiteBankConnectionRoundTrip(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "bank.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if got, err := store.ListBankConnections(ctx); err != nil || len(got) != 0 {
		t.Fatalf("ListBankConnections() = (%+v, %v), want empty", got, err)
	}
	want := []BankConnection{
		{
			SessionID: "session", AccountID: "account-1", AccountName: "Current account", Currency: "EUR",
			ValidUntil: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC),
		},
		{
			SessionID: "session", AccountID: "account-2", AccountName: "Savings account", Currency: "EUR",
			ValidUntil:          time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:           time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC),
			TransactionsEnabled: true,
		},
	}
	if err := store.ReplaceBankConnections(ctx, want); err != nil {
		t.Fatalf("ReplaceBankConnections() error = %v", err)
	}
	got, err := store.ListBankConnections(ctx)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("ListBankConnections() = (%+v, %v), want %+v", got, err, want)
	}
	if err := store.ReplaceBankConnections(ctx, want[1:]); err != nil {
		t.Fatalf("ReplaceBankConnections(second) error = %v", err)
	}
	got, err = store.ListBankConnections(ctx)
	if err != nil || !reflect.DeepEqual(got, want[1:]) {
		t.Fatalf("ListBankConnections(after replace) = (%+v, %v), want %+v", got, err, want[1:])
	}
	var legacyAccountID string
	if err := store.db.QueryRowContext(ctx, `SELECT account_id FROM bank_connection WHERE id = 1`).Scan(&legacyAccountID); err != nil {
		t.Fatalf("read legacy bank connection: %v", err)
	}
	if legacyAccountID != "account-2" {
		t.Fatalf("legacy account ID = %q, want %q", legacyAccountID, "account-2")
	}
}

func TestSQLiteMigrationBackfillsExistingBankConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bank-v4.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	statements := []string{
		`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL)`,
		`INSERT INTO schema_migrations(version, name, applied_at) VALUES
			(1, 'initial_schema', '2026-10-10T00:00:00Z'),
			(2, 'add_bookmark_tags', '2026-10-10T00:00:00Z'),
			(3, 'add_bookmark_csv', '2026-10-10T00:00:00Z'),
			(4, 'add_bank_connection', '2026-10-10T00:00:00Z')`,
		`CREATE TABLE bank_connection (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			session_id TEXT NOT NULL,
			account_id TEXT NOT NULL,
			account_name TEXT NOT NULL DEFAULT '',
			currency TEXT NOT NULL DEFAULT '',
			valid_until TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`INSERT INTO bank_connection(id, session_id, account_id, account_name, currency, valid_until, updated_at)
		 VALUES(1, 'session', 'account-1', 'Current account', 'EUR', '2027-01-01T00:00:00Z', '2026-10-10T10:00:00Z')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			_ = db.Close()
			t.Fatalf("prepare v4 database: %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close v4 database: %v", err)
	}

	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()
	connections, err := store.ListBankConnections(context.Background())
	if err != nil || len(connections) != 1 || connections[0].AccountID != "account-1" || connections[0].TransactionsEnabled {
		t.Fatalf("backfilled connections = (%+v, %v)", connections, err)
	}
}
