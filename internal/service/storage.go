package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	ErrInvalidStorageInput = errors.New("invalid storage input")
	ErrStorageTooLarge     = errors.New("file is too large")
	ErrStorageNotFound     = errors.New("storage file not found")
	ErrStorageFileExists   = errors.New("storage file already exists")
)

type UploadStorageFileInput struct {
	Filename    string
	ContentType string
	Reader      io.Reader
}

type StoredFile struct {
	Filename    string
	Path        string
	Size        int64
	ContentType string
}

type OpenedStorageFile struct {
	StoredFile
	File *os.File
}

func (s *Service) UploadStorageFile(ctx context.Context, in UploadStorageFileInput) (StoredFile, error) {
	if ctx.Err() != nil {
		return StoredFile{}, ctx.Err()
	}
	if in.Reader == nil {
		return StoredFile{}, ErrInvalidStorageInput
	}

	cleanName := sanitizeStorageFilename(in.Filename)
	uploadDir := strings.TrimSpace(s.cfg.StorageUploadDir)
	if uploadDir == "" {
		uploadDir = "uploads"
	}

	maxUploadMB := s.cfg.StorageMaxUploadMB
	if maxUploadMB <= 0 {
		maxUploadMB = 10
	}
	maxBytes := maxUploadMB * 1024 * 1024

	if err := os.MkdirAll(uploadDir, 0o750); err != nil {
		return StoredFile{}, fmt.Errorf("create upload directory: %w", err)
	}

	storedName := cleanName
	fullPath := filepath.Join(uploadDir, storedName)

	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return StoredFile{}, ErrStorageFileExists
		}
		return StoredFile{}, fmt.Errorf("open destination file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, io.LimitReader(in.Reader, maxBytes+1))
	if err != nil {
		return StoredFile{}, fmt.Errorf("write uploaded file: %w", err)
	}
	if written > maxBytes {
		_ = os.Remove(fullPath)
		return StoredFile{}, ErrStorageTooLarge
	}

	return StoredFile{
		Filename:    cleanName,
		Path:        filepath.ToSlash(fullPath),
		Size:        written,
		ContentType: strings.TrimSpace(in.ContentType),
	}, nil
}

func (s *Service) ListStorageFiles(ctx context.Context) ([]StoredFile, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	uploadDir := strings.TrimSpace(s.cfg.StorageUploadDir)
	if uploadDir == "" {
		uploadDir = "uploads"
	}

	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []StoredFile{}, nil
		}
		return nil, fmt.Errorf("list upload directory: %w", err)
	}

	files := make([]StoredFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		name := entry.Name()
		fullPath := filepath.Join(uploadDir, name)

		files = append(files, StoredFile{
			Filename:    name,
			Path:        filepath.ToSlash(fullPath),
			Size:        info.Size(),
			ContentType: detectStorageContentType(name),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Filename < files[j].Filename
	})

	return files, nil
}

func (s *Service) OpenStorageFile(ctx context.Context, filename string) (OpenedStorageFile, error) {
	if ctx.Err() != nil {
		return OpenedStorageFile{}, ctx.Err()
	}

	cleanName, err := validateRequestedStorageFilename(filename)
	if err != nil {
		return OpenedStorageFile{}, err
	}

	uploadDir := strings.TrimSpace(s.cfg.StorageUploadDir)
	if uploadDir == "" {
		uploadDir = "uploads"
	}

	fullPath := filepath.Join(uploadDir, cleanName)
	f, err := os.Open(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return OpenedStorageFile{}, ErrStorageNotFound
		}
		return OpenedStorageFile{}, fmt.Errorf("open storage file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return OpenedStorageFile{}, fmt.Errorf("stat storage file: %w", err)
	}

	if info.IsDir() {
		f.Close()
		return OpenedStorageFile{}, ErrStorageNotFound
	}

	return OpenedStorageFile{
		StoredFile: StoredFile{
			Filename:    cleanName,
			Path:        filepath.ToSlash(fullPath),
			Size:        info.Size(),
			ContentType: detectStorageContentType(cleanName),
		},
		File: f,
	}, nil
}

func sanitizeStorageFilename(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." || base == "/" || base == string(filepath.Separator) {
		return "upload.bin"
	}

	base = strings.ReplaceAll(base, "\x00", "")
	base = strings.ReplaceAll(base, "\r", "")
	base = strings.ReplaceAll(base, "\n", "")

	if base == "" {
		return "upload.bin"
	}
	return base
}

func validateRequestedStorageFilename(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", ErrInvalidStorageInput
	}

	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") {
		return "", ErrInvalidStorageInput
	}

	if filepath.Base(trimmed) != trimmed {
		return "", ErrInvalidStorageInput
	}

	return trimmed, nil
}

func detectStorageContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return "application/octet-stream"
	}

	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}

	return "application/octet-stream"
}

// StartStorageCleanupWorker starts a daily cleanup loop that deletes files older than configured retention days.
// STORAGE_CLEANUP_DAYS=0 disables cleanup.
func (s *Service) StartStorageCleanupWorker() {
	retentionDays := s.cfg.StorageCleanupDays
	if retentionDays <= 0 {
		logrus.Info("storage cleanup disabled")
		return
	}

	go func() {
		// Run once at startup so old files are cleaned without waiting for the first tick.
		if deleted, err := s.cleanupStorageOlderThan(context.Background(), retentionDays); err != nil {
			logrus.Errorf("storage cleanup failed: %v", err)
		} else if deleted > 0 {
			logrus.Infof("storage cleanup removed %d file(s)", deleted)
		}

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				deleted, err := s.cleanupStorageOlderThan(context.Background(), retentionDays)
				if err != nil {
					logrus.Errorf("storage cleanup failed: %v", err)
					continue
				}
				if deleted > 0 {
					logrus.Infof("storage cleanup removed %d file(s)", deleted)
				}
			case <-s.done:
				return
			}
		}
	}()
}

func (s *Service) cleanupStorageOlderThan(ctx context.Context, days int) (int, error) {
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if days <= 0 {
		return 0, nil
	}

	uploadDir := strings.TrimSpace(s.cfg.StorageUploadDir)
	if uploadDir == "" {
		return 0, fmt.Errorf("invalid upload directory configuration")
	}

	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("read upload directory: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	deleted := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}

		if !info.ModTime().Before(cutoff) {
			continue
		}

		fullPath := filepath.Join(uploadDir, entry.Name())
		if removeErr := os.Remove(fullPath); removeErr != nil {
			logrus.Warnf("storage cleanup failed to remove %s: %v", fullPath, removeErr)
			continue
		} else {
			logrus.Infof("storage cleanup removed %s (last modified: %s)", fullPath, info.ModTime().Format(time.RFC3339))
		}

		deleted++
	}

	return deleted, nil
}
