package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	dbPath := strings.TrimSpace(path)
	if dbPath == "" {
		dbPath = "start.db"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	store := &SQLiteStore{db: db}
	store.db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := store.db.PingContext(ctx); err != nil {
		_ = store.db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	if err := store.initSchema(ctx); err != nil {
		_ = store.db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) initSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}

	return s.applyMigrations(ctx)
}

func (s *SQLiteStore) applyMigrations(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer rollbackTx(tx)

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	applied, err := appliedMigrationVersions(ctx, tx)
	if err != nil {
		return err
	}

	lastVersion := 0
	appliedCount := 0
	for _, m := range sqliteMigrations {
		if m.version <= lastVersion {
			return fmt.Errorf("invalid sqlite migration order at version %d", m.version)
		}
		lastVersion = m.version

		if _, ok := applied[m.version]; ok {
			continue
		}

		logrus.Infof("applying sqlite migration v%d (%s)", m.version, m.name)

		for _, stmt := range m.statements {
			if _, err := tx.ExecContext(ctx, stmt); err != nil {
				return fmt.Errorf("apply sqlite migration %d (%s): %w", m.version, m.name, err)
			}
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, name, applied_at) VALUES(?, ?, ?)`,
			m.version,
			m.name,
			time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("record sqlite migration %d (%s): %w", m.version, m.name, err)
		}

		appliedCount++
		logrus.Infof("applied sqlite migration v%d (%s)", m.version, m.name)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}

	if appliedCount == 0 {
		logrus.Info("no pending sqlite migrations")
	} else {
		logrus.Infof("sqlite migrations complete: %d applied", appliedCount)
	}

	return nil
}

func appliedMigrationVersions(ctx context.Context, tx *sql.Tx) (map[int]struct{}, error) {
	rows, err := tx.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]struct{})
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration version: %w", err)
		}
		applied[version] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return applied, nil
}

func (s *SQLiteStore) CreateCategory(ctx context.Context, c Category) (Category, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO categories(name) VALUES(?)`, strings.TrimSpace(c.Name))
	if err != nil {
		return Category{}, fmt.Errorf("insert category: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Category{}, fmt.Errorf("category last insert id: %w", err)
	}

	return Category{ID: id, Name: strings.TrimSpace(c.Name)}, nil
}

func (s *SQLiteStore) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM categories ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	out := make([]Category, 0)
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		out = append(out, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories: %w", err)
	}

	return out, nil
}

func (s *SQLiteStore) CreateBookmark(ctx context.Context, b Bookmark) (Bookmark, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Bookmark{}, fmt.Errorf("begin create bookmark transaction: %w", err)
	}
	defer rollbackTx(tx)

	if ok, err := categoryExists(ctx, tx, b.CategoryID); err != nil {
		return Bookmark{}, err
	} else if !ok {
		return Bookmark{}, fmt.Errorf("category %d: %w", b.CategoryID, ErrCategoryNotFound)
	}

	position := 0
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) + 1 FROM bookmarks`).Scan(&position); err != nil {
		return Bookmark{}, fmt.Errorf("next bookmark position: %w", err)
	}

	createdAt := time.Now().UTC()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO bookmarks(url, title, category_id, position, hidden, created_at) VALUES(?, ?, ?, ?, ?, ?)`,
		b.URL,
		b.Title,
		b.CategoryID,
		position,
		boolToInt(b.Hidden),
		createdAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return Bookmark{}, fmt.Errorf("insert bookmark: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return Bookmark{}, fmt.Errorf("bookmark last insert id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Bookmark{}, fmt.Errorf("commit create bookmark transaction: %w", err)
	}

	return Bookmark{
		ID:         id,
		URL:        b.URL,
		Title:      b.Title,
		CategoryID: b.CategoryID,
		Position:   position,
		Hidden:     b.Hidden,
		CreatedAt:  createdAt,
	}, nil
}

func (s *SQLiteStore) UpdateBookmark(ctx context.Context, b Bookmark) (Bookmark, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Bookmark{}, fmt.Errorf("begin update bookmark transaction: %w", err)
	}
	defer rollbackTx(tx)

	if ok, err := categoryExists(ctx, tx, b.CategoryID); err != nil {
		return Bookmark{}, err
	} else if !ok {
		return Bookmark{}, fmt.Errorf("category %d: %w", b.CategoryID, ErrCategoryNotFound)
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE bookmarks SET url = ?, title = ?, category_id = ?, hidden = ? WHERE id = ?`,
		b.URL,
		b.Title,
		b.CategoryID,
		boolToInt(b.Hidden),
		b.ID,
	)
	if err != nil {
		return Bookmark{}, fmt.Errorf("update bookmark: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return Bookmark{}, fmt.Errorf("bookmark update rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return Bookmark{}, fmt.Errorf("bookmark %d: %w", b.ID, ErrBookmarkNotFound)
	}

	updated, err := getBookmarkByID(ctx, tx, b.ID)
	if err != nil {
		return Bookmark{}, err
	}

	if err := tx.Commit(); err != nil {
		return Bookmark{}, fmt.Errorf("commit update bookmark transaction: %w", err)
	}

	return updated, nil
}

func (s *SQLiteStore) ListBookmarks(ctx context.Context, includeHidden bool) ([]Bookmark, error) {
	query := `
		SELECT id, url, title, category_id, position, hidden, created_at
		FROM bookmarks
	`
	if includeHidden {
		query += ` ORDER BY position ASC, id ASC`
	} else {
		query += ` WHERE hidden = 0 ORDER BY position ASC, id ASC`
	}

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list bookmarks: %w", err)
	}
	defer rows.Close()

	out := make([]Bookmark, 0)
	for rows.Next() {
		b, err := scanBookmark(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookmarks: %w", err)
	}

	return out, nil
}

func (s *SQLiteStore) ReorderBookmarks(ctx context.Context, ids []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reorder bookmarks transaction: %w", err)
	}
	defer rollbackTx(tx)

	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM bookmarks`).Scan(&total); err != nil {
		return fmt.Errorf("count bookmarks: %w", err)
	}

	if total != len(ids) {
		return ErrInvalidBookmarkOrder
	}

	seen := make(map[int64]struct{}, len(ids))
	for idx, id := range ids {
		if _, ok := seen[id]; ok {
			return ErrInvalidBookmarkOrder
		}
		seen[id] = struct{}{}

		res, err := tx.ExecContext(ctx, `UPDATE bookmarks SET position = ? WHERE id = ?`, idx+1, id)
		if err != nil {
			return fmt.Errorf("reorder bookmark %d: %w", id, err)
		}
		rowsAffected, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("reorder rows affected for bookmark %d: %w", id, err)
		}
		if rowsAffected == 0 {
			return fmt.Errorf("bookmark %d: %w", id, ErrBookmarkNotFound)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder bookmarks transaction: %w", err)
	}

	return nil
}

func (s *SQLiteStore) DeleteBookmark(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM bookmarks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete bookmark rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("bookmark %d: %w", id, ErrBookmarkNotFound)
	}

	return nil
}

func (s *SQLiteStore) CreateReadingListItem(ctx context.Context, item ReadingListItem) (ReadingListItem, error) {
	createdAt := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO reading_list_items(url, title, created_at) VALUES(?, ?, ?)`,
		item.URL,
		item.Title,
		createdAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return ReadingListItem{}, fmt.Errorf("insert reading list item: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return ReadingListItem{}, fmt.Errorf("reading list item last insert id: %w", err)
	}

	return ReadingListItem{
		ID:        id,
		URL:       item.URL,
		Title:     item.Title,
		CreatedAt: createdAt,
	}, nil
}

func (s *SQLiteStore) ListReadingListItems(ctx context.Context) ([]ReadingListItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, title, created_at FROM reading_list_items ORDER BY created_at DESC, id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list reading list items: %w", err)
	}
	defer rows.Close()

	out := make([]ReadingListItem, 0)
	for rows.Next() {
		item, err := scanReadingListItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reading list items: %w", err)
	}

	return out, nil
}

func categoryExists(ctx context.Context, tx *sql.Tx, categoryID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM categories WHERE id = ? LIMIT 1`, categoryID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, fmt.Errorf("check category existence: %w", err)
}

func getBookmarkByID(ctx context.Context, tx *sql.Tx, id int64) (Bookmark, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT id, url, title, category_id, position, hidden, created_at FROM bookmarks WHERE id = ?`,
		id,
	)
	b, err := scanBookmarkRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Bookmark{}, fmt.Errorf("bookmark %d: %w", id, ErrBookmarkNotFound)
		}
		return Bookmark{}, err
	}
	return b, nil
}

func scanBookmark(rows *sql.Rows) (Bookmark, error) {
	var (
		b         Bookmark
		hiddenInt int
		createdAt string
	)

	if err := rows.Scan(&b.ID, &b.URL, &b.Title, &b.CategoryID, &b.Position, &hiddenInt, &createdAt); err != nil {
		return Bookmark{}, fmt.Errorf("scan bookmark: %w", err)
	}

	parsedTime, err := parseSQLiteTime(createdAt)
	if err != nil {
		return Bookmark{}, err
	}
	b.CreatedAt = parsedTime
	b.Hidden = hiddenInt != 0

	return b, nil
}

func scanBookmarkRow(row *sql.Row) (Bookmark, error) {
	var (
		b         Bookmark
		hiddenInt int
		createdAt string
	)

	if err := row.Scan(&b.ID, &b.URL, &b.Title, &b.CategoryID, &b.Position, &hiddenInt, &createdAt); err != nil {
		return Bookmark{}, fmt.Errorf("scan bookmark row: %w", err)
	}

	parsedTime, err := parseSQLiteTime(createdAt)
	if err != nil {
		return Bookmark{}, err
	}
	b.CreatedAt = parsedTime
	b.Hidden = hiddenInt != 0

	return b, nil
}

func scanReadingListItem(rows *sql.Rows) (ReadingListItem, error) {
	var (
		item      ReadingListItem
		createdAt string
	)

	if err := rows.Scan(&item.ID, &item.URL, &item.Title, &createdAt); err != nil {
		return ReadingListItem{}, fmt.Errorf("scan reading list item: %w", err)
	}

	parsedTime, err := parseSQLiteTime(createdAt)
	if err != nil {
		return ReadingListItem{}, err
	}
	item.CreatedAt = parsedTime

	return item, nil
}

func parseSQLiteTime(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return t, nil
	}

	t, err = time.Parse(time.RFC3339, value)
	if err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("parse sqlite timestamp %q: %w", value, err)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func rollbackTx(tx *sql.Tx) {
	_ = tx.Rollback()
}
