package storage

import (
	"context"
	stdsql "database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/samber/oops"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite"
)

type Database struct {
	path   string
	mu     sync.Mutex
	client *bun.DB
}

func NewDatabase(path string) *Database {
	return &Database{path: path}
}

func (db *Database) Client(ctx context.Context) (*bun.DB, error) {
	return db.DB(ctx)
}

func (db *Database) DB(ctx context.Context) (*bun.DB, error) {
	if db == nil || strings.TrimSpace(db.path) == "" {
		return nil, oops.In("storage_database").New("sqlite database path is empty")
	}
	errb := oops.In("storage_database").With("path", db.path)

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.client != nil {
		return db.client, nil
	}
	directory := filepath.Dir(db.path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, errb.With("directory", directory).Wrapf(err, "create sqlite database directory")
	}

	rawDB, err := stdsql.Open("sqlite", db.path)
	if err != nil {
		return nil, errb.Wrapf(err, "open sqlite database")
	}
	rawDB.SetMaxOpenConns(1)

	if err := setupDatabase(ctx, rawDB); err != nil {
		_ = rawDB.Close()
		return nil, errb.Wrap(err)
	}

	client := bun.NewDB(rawDB, sqlitedialect.New())
	if err := setupSchema(ctx, client); err != nil {
		return nil, oops.Join(
			errb.Wrapf(err, "apply sqlite bun schema"),
			errb.Wrap(client.Close()),
		)
	}
	db.client = client
	return db.client, nil
}

func (db *Database) Close() error {
	if db == nil {
		return nil
	}
	errb := oops.In("storage_database").With("path", db.path)

	db.mu.Lock()
	defer db.mu.Unlock()

	if db.client == nil {
		return nil
	}
	err := db.client.Close()
	db.client = nil
	if err != nil {
		return errb.Wrapf(err, "close sqlite database")
	}
	return nil
}

func setupDatabase(ctx context.Context, db *stdsql.DB) error {
	errb := oops.In("storage_database")
	for _, statement := range []string{
		`PRAGMA journal_mode = WAL`,
		`PRAGMA foreign_keys = ON`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return errb.With("statement", statement).Wrapf(err, "configure sqlite database")
		}
	}
	return nil
}

func setupSchema(ctx context.Context, db *bun.DB) error {
	errb := oops.In("storage_database")
	if _, err := db.NewCreateTable().
		Model((*DailyBarRow)(nil)).
		IfNotExists().
		Exec(ctx); err != nil {
		return errb.Wrapf(err, "create daily_bar table")
	}

	indexes := []struct {
		name    string
		columns []string
		unique  bool
	}{
		{
			name:    "daily_bar_natural_key",
			columns: []string{"market", "security_type", "trading_date", "symbol", "provider", "provider_group"},
			unique:  true,
		},
		{
			name:    "idx_daily_bar_date",
			columns: []string{"market", "security_type", "trading_date"},
		},
		{
			name:    "idx_daily_bar_symbol_date",
			columns: []string{"market", "security_type", "symbol", "trading_date"},
		},
	}
	for _, index := range indexes {
		query := db.NewCreateIndex().
			Model((*DailyBarRow)(nil)).
			Index(index.name).
			Column(index.columns...).
			IfNotExists()
		if index.unique {
			query = query.Unique()
		}
		if _, err := query.Exec(ctx); err != nil {
			return errb.With("index", index.name).Wrapf(err, "create daily_bar index")
		}
	}
	return nil
}
