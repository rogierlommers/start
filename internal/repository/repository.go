package repository

import (
	"context"
	"errors"
	"time"
)

// ErrCategoryNotFound is returned when a referenced category does not exist.
var ErrCategoryNotFound = errors.New("category not found")

// ErrBookmarkNotFound is returned when a referenced bookmark does not exist.
var ErrBookmarkNotFound = errors.New("bookmark not found")

// ErrInvalidBookmarkOrder is returned when a reorder payload is invalid.
var ErrInvalidBookmarkOrder = errors.New("invalid bookmark order")

// Category represents a bookmark category.
type Category struct {
	ID   int64
	Name string
}

// CategoryStore defines persistence operations for categories.
type CategoryStore interface {
	CreateCategory(ctx context.Context, c Category) (Category, error)
	ListCategories(ctx context.Context) ([]Category, error)
	DeleteCategory(ctx context.Context, id int64) error
}

// Bookmark represents a stored bookmark entry.
type Bookmark struct {
	ID         int64
	URL        string
	Title      string
	CategoryID int64
	Position   int
	CreatedAt  time.Time
}

// BookmarkStore defines persistence operations for bookmarks.
type BookmarkStore interface {
	CreateBookmark(ctx context.Context, b Bookmark) (Bookmark, error)
	UpdateBookmark(ctx context.Context, b Bookmark) (Bookmark, error)
	ListBookmarks(ctx context.Context) ([]Bookmark, error)
	ReorderBookmarks(ctx context.Context, ids []int64) error
	DeleteBookmark(ctx context.Context, id int64) error
}

// ReadingListItem represents a URL saved for later reading.
type ReadingListItem struct {
	ID        int64
	URL       string
	Title     string
	CreatedAt time.Time
}

// ReadingListStore defines persistence operations for reading-list items.
type ReadingListStore interface {
	CreateReadingListItem(ctx context.Context, item ReadingListItem) (ReadingListItem, error)
	ListReadingListItems(ctx context.Context) ([]ReadingListItem, error)
}

// Store defines persistence dependencies used by the service layer.
type Store interface {
	CategoryStore
	BookmarkStore
	ReadingListStore
}

// NoopStore is a placeholder repository implementation for scaffolding.
type NoopStore struct{}

func NewNoopStore() *NoopStore {
	return &NoopStore{}
}

func (n *NoopStore) CreateCategory(_ context.Context, c Category) (Category, error) {
	return c, nil
}

func (n *NoopStore) ListCategories(_ context.Context) ([]Category, error) {
	return []Category{}, nil
}

func (n *NoopStore) DeleteCategory(_ context.Context, _ int64) error {
	return nil
}

func (n *NoopStore) CreateBookmark(_ context.Context, b Bookmark) (Bookmark, error) {
	return b, nil
}

func (n *NoopStore) UpdateBookmark(_ context.Context, b Bookmark) (Bookmark, error) {
	return b, nil
}

func (n *NoopStore) ListBookmarks(_ context.Context) ([]Bookmark, error) {
	return []Bookmark{}, nil
}

func (n *NoopStore) ReorderBookmarks(_ context.Context, _ []int64) error {
	return nil
}

func (n *NoopStore) DeleteBookmark(_ context.Context, _ int64) error {
	return nil
}

func (n *NoopStore) CreateReadingListItem(_ context.Context, item ReadingListItem) (ReadingListItem, error) {
	return item, nil
}

func (n *NoopStore) ListReadingListItems(_ context.Context) ([]ReadingListItem, error) {
	return []ReadingListItem{}, nil
}
