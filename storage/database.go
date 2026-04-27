package storage

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/ev3rlit/mwosa/storage/ent"
	_ "modernc.org/sqlite"
)

type Database struct {
	path string
}

func NewDatabase(path string) *Database {
	return &Database{path: path}
}

func (db *Database) Open(ctx context.Context) (*ent.Client, error) {
	if db == nil || strings.TrimSpace(db.path) == "" {
		return nil, fmt.Errorf("sqlite database path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(db.path), 0o755); err != nil {
		return nil, fmt.Errorf("create sqlite database directory: %w", err)
	}

	rawDB, err := stdsql.Open("sqlite", db.path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database %s: %w", db.path, err)
	}
	rawDB.SetMaxOpenConns(1)

	if err := setupDatabase(ctx, rawDB); err != nil {
		_ = rawDB.Close()
		return nil, err
	}

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, rawDB)))
	if err := client.Schema.Create(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("apply sqlite ent schema: %w", err)
	}
	return client, nil
}

func setupDatabase(ctx context.Context, db *stdsql.DB) error {
	for _, statement := range []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure sqlite database: %w", err)
		}
	}
	return nil
}
