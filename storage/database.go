package storage

import (
	"context"
	stdsql "database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/ev3rlit/mwosa/storage/ent"
	"github.com/samber/oops"
	_ "modernc.org/sqlite"
)

type Database struct {
	path   string
	mu     sync.Mutex
	client *ent.Client
}

func NewDatabase(path string) *Database {
	return &Database{path: path}
}

func (db *Database) Client(ctx context.Context) (*ent.Client, error) {
	if db == nil || strings.TrimSpace(db.path) == "" {
		return nil, oops.In("storage_database").New("sqlite database path is empty")
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.client != nil {
		return db.client, nil
	}
	if err := os.MkdirAll(filepath.Dir(db.path), 0o755); err != nil {
		return nil, oops.In("storage_database").With("path", db.path, "directory", filepath.Dir(db.path)).Wrapf(err, "create sqlite database directory")
	}

	rawDB, err := stdsql.Open("sqlite", db.path)
	if err != nil {
		return nil, oops.In("storage_database").With("path", db.path).Wrapf(err, "open sqlite database")
	}
	rawDB.SetMaxOpenConns(1)

	if err := setupDatabase(ctx, rawDB); err != nil {
		_ = rawDB.Close()
		return nil, oops.In("storage_database").With("path", db.path).Wrap(err)
	}

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, rawDB)))
	if err := client.Schema.Create(ctx); err != nil {
		return nil, oops.Join(
			oops.In("storage_database").With("path", db.path).Wrapf(err, "apply sqlite ent schema"),
			oops.In("storage_database").With("path", db.path).Wrap(client.Close()),
		)
	}
	db.client = client
	return db.client, nil
}

func (db *Database) Close() error {
	if db == nil {
		return nil
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.client == nil {
		return nil
	}
	err := db.client.Close()
	db.client = nil
	if err != nil {
		return oops.In("storage_database").With("path", db.path).Wrapf(err, "close sqlite database")
	}
	return nil
}

func setupDatabase(ctx context.Context, db *stdsql.DB) error {
	for _, statement := range []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return oops.In("storage_database").With("statement", statement).Wrapf(err, "configure sqlite database")
		}
	}
	return nil
}
