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

func TestDatabaseCreatesDailyBarIndexes(t *testing.T) {
	database := NewDatabase(filepath.Join(t.TempDir(), "mwosa.db"))
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close database: %v", err)
		}
	})

	client, err := database.Client(context.Background())
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	rows, err := client.QueryContext(context.Background(), `PRAGMA index_list('daily_bar')`)
	if err != nil {
		t.Fatalf("index list: %v", err)
	}
	defer rows.Close()

	indexes := make(map[string]bool)
	for rows.Next() {
		var seq int
		var name string
		var unique bool
		var origin string
		var partial bool
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan index row: %v", err)
		}
		indexes[name] = unique
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate index rows: %v", err)
	}

	if !indexes["daily_bar_natural_key"] {
		t.Fatal("daily_bar_natural_key unique index was not created")
	}
	for _, name := range []string{"idx_daily_bar_date", "idx_daily_bar_symbol_date"} {
		if _, ok := indexes[name]; !ok {
			t.Fatalf("%s index was not created", name)
		}
	}
}
