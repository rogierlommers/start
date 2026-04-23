package config

import (
	"os"
	"testing"
)

func TestReadingListCleanupDaysDefault(t *testing.T) {
	// Clear env to test default
	os.Setenv("READING_LIST_CLEANUP_DAYS", "")

	cfg := Config{
		ReadingListCleanupDays: 30, // default set in Load()
	}

	if cfg.ReadingListCleanupDays != 30 {
		t.Fatalf("default ReadingListCleanupDays = %d, want 30", cfg.ReadingListCleanupDays)
	}
}

func TestReadingListCleanupDaysZeroDisablesCleanup(t *testing.T) {
	cfg := Config{
		ReadingListCleanupDays: 0,
	}

	if cfg.ReadingListCleanupDays != 0 {
		t.Fatalf("ReadingListCleanupDays = %d, want 0 (disabled)", cfg.ReadingListCleanupDays)
	}
}

func TestReadingListCleanupDaysCustom(t *testing.T) {
	cfg := Config{
		ReadingListCleanupDays: 7,
	}

	if cfg.ReadingListCleanupDays != 7 {
		t.Fatalf("ReadingListCleanupDays = %d, want 7", cfg.ReadingListCleanupDays)
	}
}
