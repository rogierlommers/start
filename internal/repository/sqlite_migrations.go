package repository

type sqliteMigration struct {
	version    int
	name       string
	statements []string
}

// Migration guide:
// - Add new migrations only by appending entries with strictly increasing versions.
// - Never modify or delete already shipped migration entries.
// - Keep statements idempotent when possible (for example, IF NOT EXISTS).
// - Prefer additive changes that are safe for existing data.
//
// Example v2 migration:
//
//	{
//		version: 2,
//		name:    "add_categories_name_index",
//		statements: []string{
//			`CREATE INDEX IF NOT EXISTS idx_categories_name ON categories(name)`,
//		},
//	},
var sqliteMigrations = []sqliteMigration{
	{
		version: 1,
		name:    "initial_schema",
		statements: []string{
			`CREATE TABLE IF NOT EXISTS categories (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_name_unique_nocase ON categories(name COLLATE NOCASE)`,
			`CREATE TABLE IF NOT EXISTS bookmarks (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				url TEXT NOT NULL,
				title TEXT NOT NULL DEFAULT '',
				category_id INTEGER NOT NULL,
				position INTEGER NOT NULL,
				hidden INTEGER NOT NULL DEFAULT 0,
				created_at TEXT NOT NULL,
				FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE RESTRICT
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_bookmarks_url_unique_nocase ON bookmarks(url COLLATE NOCASE)`,
			`CREATE INDEX IF NOT EXISTS idx_bookmarks_position ON bookmarks(position, id)`,
			`CREATE TABLE IF NOT EXISTS reading_list_items (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				url TEXT NOT NULL,
				title TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_reading_list_created_at ON reading_list_items(created_at DESC, id DESC)`,
		},
	},
}
