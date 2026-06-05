package db

import (
	"context"
	"database/sql"

	_ "modernc.org/sqlite"
)

type sqliteDB struct {
	db *sql.DB
}

// OpenSQLite opens (or creates) a SQLite database at the given DSN.
// Use ":memory:" for an in-memory database.
func OpenSQLite(dsn string) (DB, error) {
	if dsn != ":memory:" {
		dsn += "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	} else {
		dsn += "?_pragma=foreign_keys(ON)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &sqliteDB{db: db}, nil
}

func (s *sqliteDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

func (s *sqliteDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

func (s *sqliteDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

// BeginTx returns a *sql.Tx which satisfies the Tx interface directly.
func (s *sqliteDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	return s.db.BeginTx(ctx, opts)
}

func (s *sqliteDB) Close() error {
	return s.db.Close()
}

func (s *sqliteDB) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, sqliteSchema)
	return err
}
