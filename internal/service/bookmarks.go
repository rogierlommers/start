package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"start/internal/repository"
)

var ErrBookmarkNotFound = errors.New("bookmark not found")
var ErrInvalidBookmarkInput = errors.New("invalid bookmark input")
var ErrCategoryNotFound = errors.New("category not found")
var ErrInvalidCategoryInput = errors.New("invalid category input")

type Category struct {
	ID   int64
	Name string
}

type Bookmark struct {
	ID         int64
	URL        string
	Title      string
	CategoryID int64
	Position   int
	CreatedAt  time.Time
}

type CreateCategoryInput struct {
	Name string
}

type CreateBookmarkInput struct {
	URL        string
	Title      string
	CategoryID int64
}

type UpdateBookmarkInput struct {
	ID         int64
	URL        string
	Title      string
	CategoryID int64
}

func (s *Service) CreateCategory(ctx context.Context, in CreateCategoryInput) (Category, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Category{}, fmt.Errorf("%w: name is required", ErrInvalidCategoryInput)
	}

	c, err := s.store.CreateCategory(ctx, repository.Category{Name: name})
	if err != nil {
		return Category{}, fmt.Errorf("create category: %w", err)
	}

	return Category{ID: c.ID, Name: c.Name}, nil
}

func (s *Service) ListCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.store.ListCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	out := make([]Category, len(rows))
	for i, r := range rows {
		out[i] = Category{ID: r.ID, Name: r.Name}
	}

	return out, nil
}

func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be a positive integer", ErrInvalidCategoryInput)
	}

	if err := s.store.DeleteCategory(ctx, id); err != nil {
		return fmt.Errorf("delete category: %w", err)
	}

	return nil
}

func (s *Service) CreateBookmark(ctx context.Context, in CreateBookmarkInput) (Bookmark, error) {
	rawURL := strings.TrimSpace(in.URL)
	title := strings.TrimSpace(in.Title)

	if rawURL == "" {
		return Bookmark{}, fmt.Errorf("%w: url is required", ErrInvalidBookmarkInput)
	}

	if in.CategoryID <= 0 {
		return Bookmark{}, fmt.Errorf("%w: category_id is required", ErrInvalidBookmarkInput)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return Bookmark{}, fmt.Errorf("%w: url must be a valid http or https URL", ErrInvalidBookmarkInput)
	}

	b, err := s.store.CreateBookmark(ctx, repository.Bookmark{
		URL:        rawURL,
		Title:      title,
		CategoryID: in.CategoryID,
	})
	if err != nil {
		if errors.Is(err, repository.ErrCategoryNotFound) {
			return Bookmark{}, fmt.Errorf("%w: category %d does not exist", ErrCategoryNotFound, in.CategoryID)
		}
		return Bookmark{}, fmt.Errorf("create bookmark: %w", err)
	}

	return repoBookmarkToService(b), nil
}

func (s *Service) UpdateBookmark(ctx context.Context, in UpdateBookmarkInput) (Bookmark, error) {
	if in.ID <= 0 {
		return Bookmark{}, fmt.Errorf("%w: id must be a positive integer", ErrInvalidBookmarkInput)
	}

	rawURL := strings.TrimSpace(in.URL)
	title := strings.TrimSpace(in.Title)

	if rawURL == "" {
		return Bookmark{}, fmt.Errorf("%w: url is required", ErrInvalidBookmarkInput)
	}

	if in.CategoryID <= 0 {
		return Bookmark{}, fmt.Errorf("%w: category_id is required", ErrInvalidBookmarkInput)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return Bookmark{}, fmt.Errorf("%w: url must be a valid http or https URL", ErrInvalidBookmarkInput)
	}

	b, err := s.store.UpdateBookmark(ctx, repository.Bookmark{
		ID:         in.ID,
		URL:        rawURL,
		Title:      title,
		CategoryID: in.CategoryID,
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrBookmarkNotFound):
			return Bookmark{}, fmt.Errorf("%w: bookmark %d does not exist", ErrBookmarkNotFound, in.ID)
		case errors.Is(err, repository.ErrCategoryNotFound):
			return Bookmark{}, fmt.Errorf("%w: category %d does not exist", ErrCategoryNotFound, in.CategoryID)
		default:
			return Bookmark{}, fmt.Errorf("update bookmark: %w", err)
		}
	}

	return repoBookmarkToService(b), nil
}

func (s *Service) ListBookmarks(ctx context.Context) ([]Bookmark, error) {
	rows, err := s.store.ListBookmarks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list bookmarks: %w", err)
	}

	out := make([]Bookmark, len(rows))
	for i, r := range rows {
		out[i] = repoBookmarkToService(r)
	}

	return out, nil
}

func (s *Service) ReorderBookmarks(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("%w: ids is required", ErrInvalidBookmarkInput)
	}

	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("%w: ids must contain positive integers", ErrInvalidBookmarkInput)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%w: duplicate bookmark id %d", ErrInvalidBookmarkInput, id)
		}
		seen[id] = struct{}{}
	}

	if err := s.store.ReorderBookmarks(ctx, ids); err != nil {
		switch {
		case errors.Is(err, repository.ErrInvalidBookmarkOrder):
			return fmt.Errorf("%w: reorder payload must include every bookmark exactly once", ErrInvalidBookmarkInput)
		case errors.Is(err, repository.ErrBookmarkNotFound):
			return fmt.Errorf("%w: one or more bookmarks do not exist", ErrBookmarkNotFound)
		default:
			return fmt.Errorf("reorder bookmarks: %w", err)
		}
	}

	return nil
}

func (s *Service) DeleteBookmark(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be a positive integer", ErrInvalidBookmarkInput)
	}

	if err := s.store.DeleteBookmark(ctx, id); err != nil {
		return fmt.Errorf("delete bookmark: %w", err)
	}

	return nil
}

func repoBookmarkToService(b repository.Bookmark) Bookmark {
	return Bookmark{
		ID:         b.ID,
		URL:        b.URL,
		Title:      b.Title,
		CategoryID: b.CategoryID,
		Position:   b.Position,
		CreatedAt:  b.CreatedAt,
	}
}
