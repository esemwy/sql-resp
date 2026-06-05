//go:build postgres

package db

import (
	"os"
	"testing"
)

// TestPostgresDriverSuite runs the full shared driver suite against a live
// PostgreSQL instance. Requires the `postgres` build tag and a running database.
//
// Usage:
//
//	go test -tags postgres ./internal/db/... -run TestPostgresDriverSuite
//
// The DSN is read from the SQL_RESP_DSN environment variable, defaulting to
// "postgres://postgres:postgres@localhost:5432/sql_resp_test?sslmode=disable"
func TestPostgresDriverSuite(t *testing.T) {
	dsn := os.Getenv("SQL_RESP_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/sql_resp_test?sslmode=disable"
	}

	runSuite(t, func() (DB, error) {
		d, err := OpenPostgres(dsn)
		if err != nil {
			return nil, err
		}
		// Wipe state between test runs.
		ctx := t.Context()
		for _, tbl := range []string{"zsets", "sets", "hashes", "lists", "strings", "keys"} {
			d.ExecContext(ctx, "DROP TABLE IF EXISTS "+tbl+" CASCADE") //nolint:errcheck
		}
		return d, nil
	})
}
