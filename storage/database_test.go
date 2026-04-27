package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDatabaseOpensLazilyAndReusesClient(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "mwosa.db")
	database := NewDatabase(dbPath)
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Dir(dbPath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database directory exists before first use: %v", err)
	}

	first, err := database.Client(context.Background())
	if err != nil {
		t.Fatalf("first client: %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file after first use: %v", err)
	}

	second, err := database.Client(context.Background())
	if err != nil {
		t.Fatalf("second client: %v", err)
	}
	if first != second {
		t.Fatal("database returned a new client before close")
	}
}

func TestDatabaseRejectsEmptyPath(t *testing.T) {
	if _, err := NewDatabase("").Client(context.Background()); err == nil {
		t.Fatal("empty database path error is nil")
	}
}
