package db

import (
	"context"
	"database/sql"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type postgresDB struct {
	db *sql.DB
}

// OpenPostgres opens a PostgreSQL connection using the given DSN.
// DSN format: "postgres://user:password@host:port/dbname?sslmode=disable"
func OpenPostgres(dsn string) (DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return &postgresDB{db: db}, nil
}

func (p *postgresDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return p.db.ExecContext(ctx, rewritePlaceholders(query), args...)
}

func (p *postgresDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, rewritePlaceholders(query), args...)
}

func (p *postgresDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return p.db.QueryRowContext(ctx, rewritePlaceholders(query), args...)
}

func (p *postgresDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := p.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &pgTx{tx: tx}, nil
}

func (p *postgresDB) Close() error {
	return p.db.Close()
}

func (p *postgresDB) Migrate(ctx context.Context) error {
	// Split on semicolons and run each statement individually, since
	// database/sql with pgx doesn't support multi-statement exec.
	for _, stmt := range splitStatements(postgresSchema) {
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// pgTx wraps *sql.Tx and rewrites ? placeholders to $N for every query.
type pgTx struct {
	tx *sql.Tx
}

func (t *pgTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, rewritePlaceholders(query), args...)
}

func (t *pgTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, rewritePlaceholders(query), args...)
}

func (t *pgTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, rewritePlaceholders(query), args...)
}

func (t *pgTx) Commit() error   { return t.tx.Commit() }
func (t *pgTx) Rollback() error { return t.tx.Rollback() }

// rewritePlaceholders converts ? positional placeholders to $1, $2, ... style
// required by PostgreSQL. Placeholders inside single-quoted string literals
// are left unchanged.
func rewritePlaceholders(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 1
	inString := false
	for i := 0; i < len(query); i++ {
		ch := query[i]
		switch {
		case ch == '\'' && !inString:
			inString = true
			b.WriteByte(ch)
		case ch == '\'' && inString:
			// Handle escaped single quote ('').
			if i+1 < len(query) && query[i+1] == '\'' {
				b.WriteByte(ch)
				b.WriteByte(ch)
				i++
			} else {
				inString = false
				b.WriteByte(ch)
			}
		case ch == '?' && !inString:
			b.WriteByte('$')
			b.WriteString(itoa(n))
			n++
		default:
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// splitStatements splits a multi-statement SQL string on semicolons,
// returning non-empty trimmed statements.
func splitStatements(sql string) []string {
	parts := strings.Split(sql, ";")
	var stmts []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			stmts = append(stmts, p)
		}
	}
	return stmts
}

// itoa converts a small positive int to its decimal string representation.
func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	// For larger numbers, fall back to fmt-style conversion.
	digits := make([]byte, 0, 3)
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
