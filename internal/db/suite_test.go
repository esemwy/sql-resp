package db

import (
	"context"
	"testing"
)

// runSuite runs the full driver test suite against any DB implementation.
// Both SQLite and Postgres must pass the exact same suite.
func runSuite(t *testing.T, open func() (DB, error)) {
	t.Helper()

	t.Run("Migrate_idempotent", func(t *testing.T) {
		d, err := open()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		ctx := context.Background()
		if err := d.Migrate(ctx); err != nil {
			t.Fatal("first Migrate:", err)
		}
		if err := d.Migrate(ctx); err != nil {
			t.Fatal("second Migrate:", err)
		}
	})

	t.Run("BasicCRUD", func(t *testing.T) {
		d, err := open()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		ctx := context.Background()
		d.Migrate(ctx) //nolint:errcheck

		d.ExecContext(ctx, `INSERT INTO keys(key, db, type) VALUES (?, ?, ?)`, "k", 0, "string")
		d.ExecContext(ctx, `INSERT INTO strings(key, db, value) VALUES (?, ?, ?)`, "k", 0, "hello")

		var val string
		err = d.QueryRowContext(ctx,
			`SELECT value FROM strings WHERE key=? AND db=?`, "k", 0).Scan(&val)
		if err != nil || val != "hello" {
			t.Fatalf("read back: %q %v", val, err)
		}
	})

	t.Run("Transaction_commit", func(t *testing.T) {
		d, err := open()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		ctx := context.Background()
		d.Migrate(ctx) //nolint:errcheck

		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		tx.ExecContext(ctx, `INSERT INTO keys(key,db,type) VALUES(?,?,'string')`, "tx", 0)
		tx.ExecContext(ctx, `INSERT INTO strings(key,db,value) VALUES(?,?,'txval')`, "tx", 0)
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}

		var v string
		d.QueryRowContext(ctx, `SELECT value FROM strings WHERE key=?`, "tx").Scan(&v)
		if v != "txval" {
			t.Errorf("after commit: %q", v)
		}
	})

	t.Run("Transaction_rollback", func(t *testing.T) {
		d, err := open()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		ctx := context.Background()
		d.Migrate(ctx) //nolint:errcheck

		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		tx.ExecContext(ctx, `INSERT INTO keys(key,db,type) VALUES(?,?,'string')`, "rb", 0)
		tx.Rollback() //nolint:errcheck

		var count int
		d.QueryRowContext(ctx, `SELECT COUNT(*) FROM keys WHERE key=?`, "rb").Scan(&count)
		if count != 0 {
			t.Error("rollback: key should not exist")
		}
	})

	t.Run("ForeignKey_cascade", func(t *testing.T) {
		d, err := open()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		ctx := context.Background()
		d.Migrate(ctx) //nolint:errcheck

		d.ExecContext(ctx, `INSERT INTO keys(key,db,type) VALUES(?,?,'string')`, "fk", 0)
		d.ExecContext(ctx, `INSERT INTO strings(key,db,value) VALUES(?,?,'v')`, "fk", 0)
		d.ExecContext(ctx, `DELETE FROM keys WHERE key=? AND db=?`, "fk", 0)

		var count int
		d.QueryRowContext(ctx, `SELECT COUNT(*) FROM strings WHERE key=?`, "fk").Scan(&count)
		if count != 0 {
			t.Error("cascade: string row should have been deleted")
		}
	})

	t.Run("OnConflict_upsert", func(t *testing.T) {
		d, err := open()
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		ctx := context.Background()
		d.Migrate(ctx) //nolint:errcheck

		d.ExecContext(ctx, `INSERT INTO keys(key,db,type) VALUES(?,?,'string')`, "oc", 0)
		d.ExecContext(ctx, `INSERT INTO strings(key,db,value) VALUES(?,?,'v1')`, "oc", 0)

		// Second insert should update via ON CONFLICT.
		d.ExecContext(ctx,
			`INSERT INTO strings(key,db,value) VALUES(?,?,?) ON CONFLICT(key,db) DO UPDATE SET value=excluded.value`,
			"oc", 0, "v2")

		var val string
		d.QueryRowContext(ctx, `SELECT value FROM strings WHERE key=?`, "oc").Scan(&val)
		if val != "v2" {
			t.Errorf("upsert: expected v2, got %q", val)
		}
	})
}

// TestSQLiteDriverSuite runs the shared suite against the SQLite backend.
func TestSQLiteDriverSuite(t *testing.T) {
	runSuite(t, func() (DB, error) {
		return OpenSQLite(":memory:")
	})
}
