package db

import (
	"context"
	"testing"
)

func TestSQLiteMigrate(t *testing.T) {
	d, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	ctx := context.Background()
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// Idempotent — second call must not error.
	if err := d.Migrate(ctx); err != nil {
		t.Fatal("second Migrate: ", err)
	}
}

func TestSQLiteBasicCRUD(t *testing.T) {
	d, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// Insert a key then read it back.
	_, err = d.ExecContext(ctx,
		`INSERT INTO keys(key, db, type) VALUES (?, ?, ?)`, "hello", 0, "string")
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.ExecContext(ctx,
		`INSERT INTO strings(key, db, value) VALUES (?, ?, ?)`, "hello", 0, "world")
	if err != nil {
		t.Fatal(err)
	}

	var val string
	err = d.QueryRowContext(ctx,
		`SELECT value FROM strings WHERE key=? AND db=?`, "hello", 0).Scan(&val)
	if err != nil {
		t.Fatal(err)
	}
	if val != "world" {
		t.Errorf("got %q", val)
	}
}

func TestSQLiteTransaction(t *testing.T) {
	d, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO keys(key, db, type) VALUES (?, ?, ?)`, "txkey", 0, "string"); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	// Key must not exist after rollback.
	var count int
	d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM keys WHERE key=?`, "txkey").Scan(&count)
	if count != 0 {
		t.Error("rollback did not remove the row")
	}
}

func TestSQLiteForeignKeyConstraint(t *testing.T) {
	d, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// Inserting into strings without a corresponding keys row must fail.
	_, err = d.ExecContext(ctx,
		`INSERT INTO strings(key, db, value) VALUES (?, ?, ?)`, "ghost", 0, "val")
	if err == nil {
		t.Error("expected foreign key violation")
	}
}

func TestSQLiteCascadeDelete(t *testing.T) {
	d, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx := context.Background()
	if err := d.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	d.ExecContext(ctx, `INSERT INTO keys(key, db, type) VALUES (?, ?, ?)`, "k", 0, "string")
	d.ExecContext(ctx, `INSERT INTO strings(key, db, value) VALUES (?, ?, ?)`, "k", 0, "v")
	d.ExecContext(ctx, `DELETE FROM keys WHERE key=? AND db=?`, "k", 0)

	var count int
	d.QueryRowContext(ctx, `SELECT COUNT(*) FROM strings WHERE key=?`, "k").Scan(&count)
	if count != 0 {
		t.Error("cascade delete did not remove string row")
	}
}
