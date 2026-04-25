package repository

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryStoreCategoryAndBookmarkFlow(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	cat, err := store.CreateCategory(ctx, Category{Name: "General"})
	if err != nil {
		t.Fatalf("CreateCategory() error = %v", err)
	}
	if _, err := store.CreateCategory(ctx, Category{Name: "general"}); !errors.Is(err, ErrCategoryAlreadyExists) {
		t.Fatalf("CreateCategory(duplicate) error = %v, want %v", err, ErrCategoryAlreadyExists)
	}

	categories, err := store.ListCategories(ctx)
	if err != nil || len(categories) != 1 {
		t.Fatalf("ListCategories() = (%v, %v), want one category", categories, err)
	}

	_, err = store.CreateBookmark(ctx, Bookmark{URL: "https://missing.example", CategoryID: 999})
	if !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("CreateBookmark(missing category) error = %v, want %v", err, ErrCategoryNotFound)
	}

	bm1, err := store.CreateBookmark(ctx, Bookmark{URL: "https://one.example", Tag: "alpha", CategoryID: cat.ID})
	if err != nil {
		t.Fatalf("CreateBookmark(first) error = %v", err)
	}
	bm2, err := store.CreateBookmark(ctx, Bookmark{URL: "https://two.example", CategoryID: cat.ID})
	if err != nil {
		t.Fatalf("CreateBookmark(second) error = %v", err)
	}

	_, err = store.CreateBookmark(ctx, Bookmark{URL: "https://one.example", CategoryID: cat.ID})
	if !errors.Is(err, ErrBookmarkAlreadyExists) {
		t.Fatalf("CreateBookmark(duplicate URL) error = %v, want %v", err, ErrBookmarkAlreadyExists)
	}

	updated, err := store.UpdateBookmark(ctx, Bookmark{ID: bm1.ID, URL: "https://updated.example", Tag: "updated", CategoryID: cat.ID, Hidden: true})
	if err != nil {
		t.Fatalf("UpdateBookmark() error = %v", err)
	}
	if !updated.Hidden || updated.URL != "https://updated.example" || updated.Tag != "updated" {
		t.Fatalf("UpdateBookmark() = %+v", updated)
	}

	_, err = store.UpdateBookmark(ctx, Bookmark{ID: 999, URL: "https://none.example", CategoryID: cat.ID})
	if !errors.Is(err, ErrBookmarkNotFound) {
		t.Fatalf("UpdateBookmark(missing) error = %v, want %v", err, ErrBookmarkNotFound)
	}

	visible, err := store.ListBookmarks(ctx, false)
	if err != nil {
		t.Fatalf("ListBookmarks(false) error = %v", err)
	}
	if len(visible) != 1 || visible[0].ID != bm2.ID {
		t.Fatalf("ListBookmarks(false) = %+v", visible)
	}

	all, err := store.ListBookmarks(ctx, true)
	if err != nil {
		t.Fatalf("ListBookmarks(true) error = %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListBookmarks(true) len = %d, want 2", len(all))
	}
	if all[0].Tag != "updated" {
		t.Fatalf("ListBookmarks(true) tags = %+v", all)
	}

	if err := store.ReorderBookmarks(ctx, []int64{bm1.ID}); !errors.Is(err, ErrInvalidBookmarkOrder) {
		t.Fatalf("ReorderBookmarks(wrong len) error = %v, want %v", err, ErrInvalidBookmarkOrder)
	}
	if err := store.ReorderBookmarks(ctx, []int64{bm1.ID, bm1.ID}); !errors.Is(err, ErrInvalidBookmarkOrder) {
		t.Fatalf("ReorderBookmarks(duplicate ids) error = %v, want %v", err, ErrInvalidBookmarkOrder)
	}
	if err := store.ReorderBookmarks(ctx, []int64{bm1.ID, 999}); !errors.Is(err, ErrBookmarkNotFound) {
		t.Fatalf("ReorderBookmarks(missing id) error = %v, want %v", err, ErrBookmarkNotFound)
	}
	if err := store.ReorderBookmarks(ctx, []int64{bm2.ID, bm1.ID}); err != nil {
		t.Fatalf("ReorderBookmarks(valid) error = %v", err)
	}

	if err := store.DeleteBookmark(ctx, 999); err == nil {
		t.Fatal("DeleteBookmark(missing) error = nil, want error")
	}
	if err := store.DeleteBookmark(ctx, bm1.ID); err != nil {
		t.Fatalf("DeleteBookmark(existing) error = %v", err)
	}
}

func TestMemoryStoreReadingListOrdering(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	first, err := store.CreateReadingListItem(ctx, ReadingListItem{URL: "https://a.example", Title: "A"})
	if err != nil {
		t.Fatalf("CreateReadingListItem(first) error = %v", err)
	}
	time.Sleep(5 * time.Millisecond)
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
	if items[0].ID != second.ID || items[1].ID != first.ID {
		t.Fatalf("ListReadingListItems() order = %+v", items)
	}

	deleted, err := store.DeleteReadingListItemsOlderThan(ctx, second.CreatedAt)
	if err != nil {
		t.Fatalf("DeleteReadingListItemsOlderThan() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("DeleteReadingListItemsOlderThan() deleted = %d, want 1", deleted)
	}
}

func TestNoopStoreImplementsHappyPath(t *testing.T) {
	store := NewNoopStore()
	ctx := context.Background()

	cat, err := store.CreateCategory(ctx, Category{Name: "Cat"})
	if err != nil || cat.Name != "Cat" {
		t.Fatalf("CreateCategory() = (%+v, %v)", cat, err)
	}

	categories, err := store.ListCategories(ctx)
	if err != nil || len(categories) != 0 {
		t.Fatalf("ListCategories() = (%+v, %v), want empty", categories, err)
	}

	bm, err := store.CreateBookmark(ctx, Bookmark{URL: "https://example.com"})
	if err != nil || bm.URL != "https://example.com" {
		t.Fatalf("CreateBookmark() = (%+v, %v)", bm, err)
	}

	bm, err = store.UpdateBookmark(ctx, Bookmark{ID: 1, URL: "https://updated.example"})
	if err != nil || bm.URL != "https://updated.example" {
		t.Fatalf("UpdateBookmark() = (%+v, %v)", bm, err)
	}

	bookmarks, err := store.ListBookmarks(ctx, true)
	if err != nil || len(bookmarks) != 0 {
		t.Fatalf("ListBookmarks() = (%+v, %v), want empty", bookmarks, err)
	}

	if err := store.ReorderBookmarks(ctx, []int64{1, 2}); err != nil {
		t.Fatalf("ReorderBookmarks() error = %v", err)
	}
	if err := store.DeleteBookmark(ctx, 1); err != nil {
		t.Fatalf("DeleteBookmark() error = %v", err)
	}

	item, err := store.CreateReadingListItem(ctx, ReadingListItem{URL: "https://read.example"})
	if err != nil || item.URL != "https://read.example" {
		t.Fatalf("CreateReadingListItem() = (%+v, %v)", item, err)
	}

	items, err := store.ListReadingListItems(ctx)
	if err != nil || len(items) != 0 {
		t.Fatalf("ListReadingListItems() = (%+v, %v), want empty", items, err)
	}

	deleted, err := store.DeleteReadingListItemsOlderThan(ctx, time.Now())
	if err != nil || deleted != 0 {
		t.Fatalf("DeleteReadingListItemsOlderThan() = (%d, %v), want (0,nil)", deleted, err)
	}
}
