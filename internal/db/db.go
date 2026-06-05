// Package db defines the database abstraction interface and provides
// driver implementations for SQLite and PostgreSQL.
package db

import (
	"context"
	"database/sql"
)

// Tx is the transaction interface used by the store layer.
// *sql.Tx satisfies this interface directly, so SQLite needs no wrapper.
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	Commit() error
	Rollback() error
}

// DB is the interface the store layer uses to interact with the backend.
// Each method receives a context that carries deadlines and cancellation.
type DB interface {
	// ExecContext executes a statement that returns no rows.
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)

	// QueryContext executes a query that returns rows.
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)

	// QueryRowContext executes a query expected to return at most one row.
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row

	// BeginTx starts a transaction.
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)

	// Close releases all resources held by the driver.
	Close() error

	// Migrate creates or updates the schema. Idempotent.
	Migrate(ctx context.Context) error
}

// sqliteSchema is the DDL for SQLite. Uses BLOB and REAL which are native SQLite affinities.
const sqliteSchema = `
CREATE TABLE IF NOT EXISTS keys (
    key        TEXT    NOT NULL,
    db         INTEGER NOT NULL DEFAULT 0,
    type       TEXT    NOT NULL,
    expires_at INTEGER,
    PRIMARY KEY (key, db)
);

CREATE TABLE IF NOT EXISTS strings (
    key   TEXT NOT NULL,
    db    INTEGER NOT NULL DEFAULT 0,
    value BLOB NOT NULL,
    PRIMARY KEY (key, db),
    FOREIGN KEY (key, db) REFERENCES keys(key, db) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS lists (
    key   TEXT    NOT NULL,
    db    INTEGER NOT NULL DEFAULT 0,
    idx   INTEGER NOT NULL,
    value BLOB    NOT NULL,
    PRIMARY KEY (key, db, idx),
    FOREIGN KEY (key, db) REFERENCES keys(key, db) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS hashes (
    key   TEXT NOT NULL,
    db    INTEGER NOT NULL DEFAULT 0,
    field TEXT NOT NULL,
    value BLOB NOT NULL,
    PRIMARY KEY (key, db, field),
    FOREIGN KEY (key, db) REFERENCES keys(key, db) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sets (
    key    TEXT    NOT NULL,
    db     INTEGER NOT NULL DEFAULT 0,
    member TEXT    NOT NULL,
    PRIMARY KEY (key, db, member),
    FOREIGN KEY (key, db) REFERENCES keys(key, db) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS zsets (
    key    TEXT    NOT NULL,
    db     INTEGER NOT NULL DEFAULT 0,
    member TEXT    NOT NULL,
    score  REAL    NOT NULL,
    PRIMARY KEY (key, db, member),
    FOREIGN KEY (key, db) REFERENCES keys(key, db) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS zsets_score ON zsets(key, db, score);
`

// postgresSchema is the DDL for PostgreSQL.
const postgresSchema = `
CREATE TABLE IF NOT EXISTS keys (
    key        TEXT   NOT NULL,
    db         INT    NOT NULL DEFAULT 0,
    type       TEXT   NOT NULL,
    expires_at BIGINT,
    PRIMARY KEY (key, db)
);

CREATE TABLE IF NOT EXISTS strings (
    key   TEXT NOT NULL,
    db    INT  NOT NULL DEFAULT 0,
    value TEXT NOT NULL,
    PRIMARY KEY (key, db),
    FOREIGN KEY (key, db) REFERENCES keys(key, db) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS lists (
    key   TEXT NOT NULL,
    db    INT  NOT NULL DEFAULT 0,
    idx   BIGINT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (key, db, idx),
    FOREIGN KEY (key, db) REFERENCES keys(key, db) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS hashes (
    key   TEXT NOT NULL,
    db    INT  NOT NULL DEFAULT 0,
    field TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (key, db, field),
    FOREIGN KEY (key, db) REFERENCES keys(key, db) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sets (
    key    TEXT NOT NULL,
    db     INT  NOT NULL DEFAULT 0,
    member TEXT NOT NULL,
    PRIMARY KEY (key, db, member),
    FOREIGN KEY (key, db) REFERENCES keys(key, db) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS zsets (
    key    TEXT             NOT NULL,
    db     INT              NOT NULL DEFAULT 0,
    member TEXT             NOT NULL,
    score  DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (key, db, member),
    FOREIGN KEY (key, db) REFERENCES keys(key, db) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS zsets_score ON zsets(key, db, score);
`
