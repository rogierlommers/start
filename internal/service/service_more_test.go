package service

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"start/internal/config"
	"start/internal/mailer"
	"start/internal/repository"
)

type recordingSender struct {
	mu       sync.Mutex
	messages []mailer.Message
	ch       chan mailer.Message
}

func (s *recordingSender) Send(_ context.Context, msg mailer.Message) error {
	s.mu.Lock()
	s.messages = append(s.messages, msg)
	s.mu.Unlock()
	if s.ch != nil {
		s.ch <- msg
	}
	return nil
}

func TestCategoryAndBookmarkLifecycle(t *testing.T) {
	svc := New(repository.NewMemoryStore(), mailer.DisabledSender{}, config.Config{})

	cat, err := svc.CreateCategory(context.Background(), CreateCategoryInput{Name: " General "})
	if err != nil {
		t.Fatalf("CreateCategory() error = %v", err)
	}
	if cat.Name != "General" {
		t.Fatalf("CreateCategory().Name = %q, want %q", cat.Name, "General")
	}

	cats, err := svc.ListCategories(context.Background())
	if err != nil || len(cats) != 1 {
		t.Fatalf("ListCategories() = (%v, %v), want 1 category", cats, err)
	}

	bm, err := svc.CreateBookmark(context.Background(), CreateBookmarkInput{
		URL:        "https://example.com",
		Title:      " Example ",
		Tag:        " work ",
		CategoryID: cat.ID,
	})
	if err != nil {
		t.Fatalf("CreateBookmark() error = %v", err)
	}
	if bm.Title != "Example" {
		t.Fatalf("CreateBookmark().Title = %q, want %q", bm.Title, "Example")
	}
	if bm.Tag != "work" {
		t.Fatalf("CreateBookmark().Tag = %q, want %q", bm.Tag, "work")
	}

	bm, err = svc.UpdateBookmark(context.Background(), UpdateBookmarkInput{
		ID:         bm.ID,
		URL:        "https://example.org",
		Title:      " Updated ",
		Tag:        " reference ",
		CategoryID: cat.ID,
	})
	if err != nil {
		t.Fatalf("UpdateBookmark() error = %v", err)
	}
	if bm.URL != "https://example.org" || bm.Title != "Updated" || bm.Tag != "reference" {
		t.Fatalf("UpdateBookmark() = %+v", bm)
	}

	bm, err = svc.ToggleBookmarkHidden(context.Background(), bm.ID, true)
	if err != nil {
		t.Fatalf("ToggleBookmarkHidden() error = %v", err)
	}
	if !bm.Hidden {
		t.Fatal("ToggleBookmarkHidden() returned Hidden=false, want true")
	}

	visible, err := svc.ListBookmarks(context.Background(), false)
	if err != nil {
		t.Fatalf("ListBookmarks(false) error = %v", err)
	}
	if len(visible) != 0 {
		t.Fatalf("ListBookmarks(false) len = %d, want 0", len(visible))
	}

	hidden, err := svc.ListBookmarks(context.Background(), true)
	if err != nil {
		t.Fatalf("ListBookmarks(true) error = %v", err)
	}
	if len(hidden) != 1 || hidden[0].ID != bm.ID {
		t.Fatalf("ListBookmarks(true) = %+v", hidden)
	}
	if hidden[0].Tag != "reference" {
		t.Fatalf("ListBookmarks(true).Tag = %q, want %q", hidden[0].Tag, "reference")
	}

	if err := svc.ReorderBookmarks(context.Background(), []int64{bm.ID}); err != nil {
		t.Fatalf("ReorderBookmarks() error = %v", err)
	}

	if err := svc.DeleteBookmark(context.Background(), bm.ID); err != nil {
		t.Fatalf("DeleteBookmark() error = %v", err)
	}
}

func TestBookmarkValidationErrors(t *testing.T) {
	svc := New(repository.NewMemoryStore(), mailer.DisabledSender{}, config.Config{})

	if _, err := svc.CreateCategory(context.Background(), CreateCategoryInput{}); !errors.Is(err, ErrInvalidCategoryInput) {
		t.Fatalf("CreateCategory() error = %v, want %v", err, ErrInvalidCategoryInput)
	}

	if _, err := svc.CreateBookmark(context.Background(), CreateBookmarkInput{}); !errors.Is(err, ErrInvalidBookmarkInput) {
		t.Fatalf("CreateBookmark(empty) error = %v, want %v", err, ErrInvalidBookmarkInput)
	}

	_, err := svc.CreateBookmark(context.Background(), CreateBookmarkInput{URL: "https://example.com", CategoryID: 99})
	if !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("CreateBookmark(missing category) error = %v, want %v", err, ErrCategoryNotFound)
	}

	if _, err := svc.UpdateBookmark(context.Background(), UpdateBookmarkInput{ID: 0}); !errors.Is(err, ErrInvalidBookmarkInput) {
		t.Fatalf("UpdateBookmark() error = %v, want %v", err, ErrInvalidBookmarkInput)
	}

	if err := svc.ReorderBookmarks(context.Background(), nil); !errors.Is(err, ErrInvalidBookmarkInput) {
		t.Fatalf("ReorderBookmarks(nil) error = %v, want %v", err, ErrInvalidBookmarkInput)
	}

	if err := svc.DeleteBookmark(context.Background(), 0); !errors.Is(err, ErrInvalidBookmarkInput) {
		t.Fatalf("DeleteBookmark(0) error = %v, want %v", err, ErrInvalidBookmarkInput)
	}

	if _, err := svc.ToggleBookmarkHidden(context.Background(), 0, true); !errors.Is(err, ErrInvalidBookmarkInput) {
		t.Fatalf("ToggleBookmarkHidden(0) error = %v, want %v", err, ErrInvalidBookmarkInput)
	}
}

func TestReadingListValidationAndListing(t *testing.T) {
	svc := New(repository.NewMemoryStore(), mailer.DisabledSender{}, config.Config{})

	if _, err := svc.AddReadingListItem(context.Background(), AddReadingListItemInput{}); !errors.Is(err, ErrInvalidReadingListInput) {
		t.Fatalf("AddReadingListItem(empty) error = %v, want %v", err, ErrInvalidReadingListInput)
	}

	item, err := svc.AddReadingListItem(context.Background(), AddReadingListItemInput{URL: "https://example.com/read", Title: " Read me "})
	if err != nil {
		t.Fatalf("AddReadingListItem() error = %v", err)
	}
	if item.Title != "Read me" {
		t.Fatalf("AddReadingListItem().Title = %q, want %q", item.Title, "Read me")
	}

	items, err := svc.ListReadingListItems(context.Background())
	if err != nil || len(items) != 1 {
		t.Fatalf("ListReadingListItems() = (%v, %v), want 1 item", items, err)
	}
}

func TestSendMailValidationAndQueueing(t *testing.T) {
	disabledSvc := New(repository.NewMemoryStore(), mailer.DisabledSender{}, config.Config{})
	if err := disabledSvc.SendMail(context.Background(), SendMailInput{}); !errors.Is(err, ErrDisabledMailer) {
		t.Fatalf("SendMail(disabled) error = %v, want %v", err, ErrDisabledMailer)
	}

	svc := &Service{
		store:     repository.NewMemoryStore(),
		mailer:    &recordingSender{},
		mailQueue: make(chan mailTask, 1),
		done:      make(chan struct{}),
	}

	if err := svc.SendMail(context.Background(), SendMailInput{}); !errors.Is(err, ErrInvalidMailInput) {
		t.Fatalf("SendMail(empty) error = %v, want %v", err, ErrInvalidMailInput)
	}

	if err := svc.SendMail(context.Background(), SendMailInput{To: "not-an-email", Subject: "s", Body: "b"}); !errors.Is(err, ErrInvalidMailInput) {
		t.Fatalf("SendMail(invalid recipient) error = %v, want %v", err, ErrInvalidMailInput)
	}

	if err := svc.SendMail(context.Background(), SendMailInput{To: "person@example.com", Subject: "s", Body: "body"}); err != nil {
		t.Fatalf("SendMail(valid) error = %v", err)
	}

	if err := svc.SendMail(context.Background(), SendMailInput{To: "person@example.com", Subject: "s", Body: "body"}); !errors.Is(err, ErrMailQueueFull) {
		t.Fatalf("SendMail(full queue) error = %v, want %v", err, ErrMailQueueFull)
	}
}

func TestStartMailWorkerDrainsQueuedMessages(t *testing.T) {
	sender := &recordingSender{ch: make(chan mailer.Message, 1)}
	svc := &Service{
		store:     repository.NewMemoryStore(),
		mailer:    sender,
		mailQueue: make(chan mailTask, 2),
		done:      make(chan struct{}),
	}

	svc.StartMailWorker()
	if err := svc.SendMail(context.Background(), SendMailInput{
		To:      "person@example.com",
		Subject: "Queued",
		Body:    "body",
	}); err != nil {
		t.Fatalf("SendMail() error = %v", err)
	}

	select {
	case msg := <-sender.ch:
		if msg.Subject != "Queued" {
			t.Fatalf("worker delivered subject = %q, want %q", msg.Subject, "Queued")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for mail worker")
	}

	svc.Close()
}

func TestStorageLifecycleAndHelpers(t *testing.T) {
	uploadDir := t.TempDir()
	svc := New(repository.NewMemoryStore(), mailer.DisabledSender{}, config.Config{
		StorageUploadDir:   uploadDir,
		StorageMaxUploadMB: 1,
	})

	stored, err := svc.UploadStorageFile(context.Background(), UploadStorageFileInput{
		Filename:    " ../report.txt\n",
		ContentType: "text/plain",
		Reader:      strings.NewReader("hello"),
	})
	if err != nil {
		t.Fatalf("UploadStorageFile() error = %v", err)
	}
	if stored.Filename != "report.txt" {
		t.Fatalf("stored filename = %q, want %q", stored.Filename, "report.txt")
	}

	if _, err := svc.UploadStorageFile(context.Background(), UploadStorageFileInput{
		Filename: "report.txt",
		Reader:   strings.NewReader("again"),
	}); !errors.Is(err, ErrStorageFileExists) {
		t.Fatalf("UploadStorageFile(duplicate) error = %v, want %v", err, ErrStorageFileExists)
	}

	files, err := svc.ListStorageFiles(context.Background())
	if err != nil || len(files) != 1 {
		t.Fatalf("ListStorageFiles() = (%v, %v), want 1 file", files, err)
	}

	opened, err := svc.OpenStorageFile(context.Background(), "report.txt")
	if err != nil {
		t.Fatalf("OpenStorageFile() error = %v", err)
	}
	defer opened.File.Close()

	data, err := io.ReadAll(opened.File)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("opened file contents = %q, want %q", string(data), "hello")
	}

	if _, err := svc.OpenStorageFile(context.Background(), "../report.txt"); !errors.Is(err, ErrInvalidStorageInput) {
		t.Fatalf("OpenStorageFile(invalid) error = %v, want %v", err, ErrInvalidStorageInput)
	}

	if _, err := svc.OpenStorageFile(context.Background(), "missing.txt"); !errors.Is(err, ErrStorageNotFound) {
		t.Fatalf("OpenStorageFile(missing) error = %v, want %v", err, ErrStorageNotFound)
	}

	tooLarge := strings.Repeat("a", 1024*1024+1)
	if _, err := svc.UploadStorageFile(context.Background(), UploadStorageFileInput{
		Filename: "large.bin",
		Reader:   strings.NewReader(tooLarge),
	}); !errors.Is(err, ErrStorageTooLarge) {
		t.Fatalf("UploadStorageFile(too large) error = %v, want %v", err, ErrStorageTooLarge)
	}

	if got := sanitizeStorageFilename("  "); got != "upload.bin" {
		t.Fatalf("sanitizeStorageFilename(empty) = %q, want %q", got, "upload.bin")
	}
	if got := sanitizeStorageFilename("dir/file.txt"); got != "file.txt" {
		t.Fatalf("sanitizeStorageFilename(path) = %q, want %q", got, "file.txt")
	}
	if _, err := validateRequestedStorageFilename("subdir/file.txt"); !errors.Is(err, ErrInvalidStorageInput) {
		t.Fatalf("validateRequestedStorageFilename(path) error = %v, want %v", err, ErrInvalidStorageInput)
	}
	if got := detectStorageContentType("file.txt"); got == "application/octet-stream" {
		t.Fatalf("detectStorageContentType(txt) = %q, want specific type", got)
	}
}

func TestCleanupStorageOlderThan(t *testing.T) {
	uploadDir := t.TempDir()
	oldPath := filepath.Join(uploadDir, "old.txt")
	newPath := filepath.Join(uploadDir, "new.txt")
	if err := os.WriteFile(oldPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("WriteFile(old) error = %v", err)
	}
	if err := os.WriteFile(newPath, []byte("new"), 0o644); err != nil {
		t.Fatalf("WriteFile(new) error = %v", err)
	}
	oldTime := time.Now().AddDate(0, 0, -10)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes(old) error = %v", err)
	}

	svc := New(repository.NewMemoryStore(), mailer.DisabledSender{}, config.Config{StorageUploadDir: uploadDir})
	deleted, err := svc.cleanupStorageOlderThan(context.Background(), 5)
	if err != nil {
		t.Fatalf("cleanupStorageOlderThan() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("cleanupStorageOlderThan() deleted = %d, want %d", deleted, 1)
	}
	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old file stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("new file should remain, stat error = %v", err)
	}
}

func TestStartCleanupWorkersDisabled(t *testing.T) {
	svc := New(repository.NewMemoryStore(), mailer.DisabledSender{}, config.Config{
		StorageCleanupDays:     0,
		ReadingListCleanupDays: 0,
	})

	// These should return immediately without starting active cleanup loops.
	svc.StartStorageCleanupWorker()
	svc.StartReadingListCleanupWorker()

	svc.Close()
}

func TestStartStorageCleanupWorkerEnabled(t *testing.T) {
	uploadDir := t.TempDir()

	// Write a file old enough to be deleted by a 1-day retention policy.
	oldPath := filepath.Join(uploadDir, "old.log")
	if err := os.WriteFile(oldPath, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	oldTime := time.Now().AddDate(0, 0, -3)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	svc := &Service{
		store:     repository.NewMemoryStore(),
		mailer:    mailer.DisabledSender{},
		mailQueue: make(chan mailTask, 1),
		done:      make(chan struct{}),
		cfg: config.Config{
			StorageUploadDir:   uploadDir,
			StorageCleanupDays: 1,
		},
	}

	svc.StartStorageCleanupWorker()

	// Give the startup run a moment, then shut down.
	time.Sleep(100 * time.Millisecond)
	svc.Close()

	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old file not cleaned up: stat error = %v", err)
	}
}

func TestStartReadingListCleanupWorkerEnabled(t *testing.T) {
	store := repository.NewMemoryStore()
	ctx := context.Background()

	// Pre-delete old items so the worker's first run has nothing to do but still
	// exercises the goroutine startup and done-channel exit paths.
	if _, err := store.DeleteReadingListItemsOlderThan(ctx, time.Now()); err != nil {
		t.Fatalf("DeleteReadingListItemsOlderThan() error = %v", err)
	}

	svc := &Service{
		store:     store,
		mailer:    mailer.DisabledSender{},
		mailQueue: make(chan mailTask, 1),
		done:      make(chan struct{}),
		cfg: config.Config{
			ReadingListCleanupDays: 1,
		},
	}

	svc.StartReadingListCleanupWorker()
	time.Sleep(50 * time.Millisecond)
	svc.Close()
}
