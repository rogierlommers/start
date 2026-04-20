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

var ErrInvalidReadingListInput = errors.New("invalid reading list input")

type ReadingListItem struct {
	ID        int64
	URL       string
	Title     string
	CreatedAt time.Time
}

type AddReadingListItemInput struct {
	URL   string
	Title string
}

func (s *Service) AddReadingListItem(ctx context.Context, in AddReadingListItemInput) (ReadingListItem, error) {
	rawURL := strings.TrimSpace(in.URL)
	title := strings.TrimSpace(in.Title)

	if rawURL == "" {
		return ReadingListItem{}, fmt.Errorf("%w: url is required", ErrInvalidReadingListInput)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ReadingListItem{}, fmt.Errorf("%w: url must be a valid http or https URL", ErrInvalidReadingListInput)
	}

	item, err := s.store.CreateReadingListItem(ctx, repository.ReadingListItem{
		URL:   rawURL,
		Title: title,
	})
	if err != nil {
		return ReadingListItem{}, fmt.Errorf("add reading list item: %w", err)
	}

	return repoReadingListItemToService(item), nil
}

func (s *Service) ListReadingListItems(ctx context.Context) ([]ReadingListItem, error) {
	rows, err := s.store.ListReadingListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("list reading list items: %w", err)
	}

	out := make([]ReadingListItem, len(rows))
	for i, row := range rows {
		out[i] = repoReadingListItemToService(row)
	}

	return out, nil
}

func repoReadingListItemToService(item repository.ReadingListItem) ReadingListItem {
	return ReadingListItem{
		ID:        item.ID,
		URL:       item.URL,
		Title:     item.Title,
		CreatedAt: item.CreatedAt,
	}
}
