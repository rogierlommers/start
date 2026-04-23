package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"start/internal/repository"

	"github.com/sirupsen/logrus"
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

// StartReadingListCleanupWorker starts a daily cleanup loop that deletes reading-list items older than configured retention days.
// READING_LIST_CLEANUP_DAYS=0 disables cleanup.
func (s *Service) StartReadingListCleanupWorker() {
	retentionDays := s.cfg.ReadingListCleanupDays
	if retentionDays <= 0 {
		logrus.Info("reading-list cleanup disabled")
		return
	}

	go func() {
		// Run once at startup so stale items are cleaned without waiting for the first tick.
		if deleted, err := s.cleanupReadingListOlderThan(context.Background(), retentionDays); err != nil {
			logrus.Errorf("reading-list cleanup failed: %v", err)
		} else if deleted > 0 {
			logrus.Infof("reading-list cleanup removed %d item(s)", deleted)
		}

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				deleted, err := s.cleanupReadingListOlderThan(context.Background(), retentionDays)
				if err != nil {
					logrus.Errorf("reading-list cleanup failed: %v", err)
					continue
				}
				if deleted > 0 {
					logrus.Infof("reading-list cleanup removed %d item(s)", deleted)
				}
			case <-s.done:
				return
			}
		}
	}()
}

func (s *Service) cleanupReadingListOlderThan(ctx context.Context, days int) (int, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if days <= 0 {
		return 0, nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	deleted, err := s.store.DeleteReadingListItemsOlderThan(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("cleanup reading list items older than %d days: %w", days, err)
	}

	return deleted, nil
}
