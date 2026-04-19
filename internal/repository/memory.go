package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// MemoryStore is a thread-safe in-memory implementation of Store.
type MemoryStore struct {
	mu         sync.RWMutex
	categories map[int64]Category
	bookmarks  map[int64]Bookmark
	nextCatID  int64
	nextBmkID  int64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		categories: make(map[int64]Category),
		bookmarks:  make(map[int64]Bookmark),
		nextCatID:  1,
		nextBmkID:  1,
	}
}

func (m *MemoryStore) CreateCategory(_ context.Context, c Category) (Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c.ID = m.nextCatID
	m.nextCatID++
	m.categories[c.ID] = c

	return c, nil
}

func (m *MemoryStore) ListCategories(_ context.Context) ([]Category, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Category, 0, len(m.categories))
	for _, c := range m.categories {
		out = append(out, c)
	}

	return out, nil
}

func (m *MemoryStore) DeleteCategory(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.categories[id]; !ok {
		return fmt.Errorf("category %d not found", id)
	}

	delete(m.categories, id)

	return nil
}

func (m *MemoryStore) CreateBookmark(_ context.Context, b Bookmark) (Bookmark, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.categories[b.CategoryID]; !ok {
		return Bookmark{}, fmt.Errorf("category %d: %w", b.CategoryID, ErrCategoryNotFound)
	}

	b.ID = m.nextBmkID
	b.Position = m.nextBookmarkPosition()
	b.CreatedAt = time.Now().UTC()
	m.nextBmkID++
	m.bookmarks[b.ID] = b

	return b, nil
}

func (m *MemoryStore) ListBookmarks(_ context.Context) ([]Bookmark, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Bookmark, 0, len(m.bookmarks))
	for _, b := range m.bookmarks {
		out = append(out, b)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Position == out[j].Position {
			return out[i].ID < out[j].ID
		}
		return out[i].Position < out[j].Position
	})

	return out, nil
}

func (m *MemoryStore) ReorderBookmarks(_ context.Context, ids []int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(ids) != len(m.bookmarks) {
		return ErrInvalidBookmarkOrder
	}

	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return ErrInvalidBookmarkOrder
		}
		seen[id] = struct{}{}

		bookmark, ok := m.bookmarks[id]
		if !ok {
			return fmt.Errorf("bookmark %d: %w", id, ErrBookmarkNotFound)
		}

		bookmark.Position = len(seen)
		m.bookmarks[id] = bookmark
	}

	return nil
}

func (m *MemoryStore) DeleteBookmark(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.bookmarks[id]; !ok {
		return fmt.Errorf("bookmark %d not found", id)
	}

	delete(m.bookmarks, id)

	return nil
}

func (m *MemoryStore) nextBookmarkPosition() int {
	maxPosition := 0
	for _, b := range m.bookmarks {
		if b.Position > maxPosition {
			maxPosition = b.Position
		}
	}

	return maxPosition + 1
}
