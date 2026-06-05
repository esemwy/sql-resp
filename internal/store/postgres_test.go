//go:build postgres

package store_test

import (
	"context"
	"os"
	"testing"

	"gitlab.smy.com/work/sql-resp/internal/db"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

// newPostgresStore creates a store backed by a live Postgres instance and
// wipes all tables before returning it. Requires the `postgres` build tag.
func newPostgresStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("SQL_RESP_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/sql_resp_test?sslmode=disable"
	}
	d, err := db.OpenPostgres(dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	ctx := context.Background()
	for _, tbl := range []string{"zsets", "sets", "hashes", "lists", "strings", "keys"} {
		d.ExecContext(ctx, "DROP TABLE IF EXISTS "+tbl+" CASCADE") //nolint:errcheck
	}
	if err := d.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return store.New(d)
}

// TestPostgresStore runs a representative subset of store operations against
// the Postgres backend to confirm the placeholder rewriter and schema are correct.
func TestPostgresStore(t *testing.T) {
	s := newPostgresStore(t)
	ctx := context.Background()

	t.Run("SetGet", func(t *testing.T) {
		s.Set(ctx, "pg_key", 0, "pg_val", store.SetOptions{})
		val, found, err := s.Get(ctx, "pg_key", 0)
		if err != nil || !found || val != "pg_val" {
			t.Fatalf("Get: %q %v %v", val, found, err)
		}
	})

	t.Run("Expire_TTL", func(t *testing.T) {
		s.Set(ctx, "pg_ttl", 0, "v", store.SetOptions{})
		s.Expire(ctx, "pg_ttl", 0, 100)
		ttl, err := s.TTL(ctx, "pg_ttl", 0)
		if err != nil || ttl <= 0 {
			t.Fatalf("TTL: %d %v", ttl, err)
		}
	})

	t.Run("IncrBy", func(t *testing.T) {
		n, err := s.IncrBy(ctx, "pg_ctr", 0, 5)
		if err != nil || n != 5 {
			t.Fatalf("IncrBy: %d %v", n, err)
		}
		n, err = s.IncrBy(ctx, "pg_ctr", 0, -2)
		if err != nil || n != 3 {
			t.Fatalf("IncrBy-2: %d %v", n, err)
		}
	})

	t.Run("Hash", func(t *testing.T) {
		s.HSet(ctx, "pg_h", 0, map[string]string{"f1": "v1", "f2": "v2"})
		val, found, err := s.HGet(ctx, "pg_h", 0, "f1")
		if err != nil || !found || val != "v1" {
			t.Fatalf("HGet: %q %v %v", val, found, err)
		}
		n, _ := s.HLen(ctx, "pg_h", 0)
		if n != 2 {
			t.Errorf("HLen: %d", n)
		}
	})

	t.Run("List", func(t *testing.T) {
		s.RPush(ctx, "pg_l", 0, []string{"a", "b", "c"})
		items, err := s.LRange(ctx, "pg_l", 0, 0, -1)
		if err != nil || len(items) != 3 || items[0] != "a" {
			t.Fatalf("LRange: %v %v", items, err)
		}
		val, found, _ := s.LPop(ctx, "pg_l", 0)
		if !found || val != "a" {
			t.Errorf("LPop: %q %v", val, found)
		}
	})

	t.Run("Set", func(t *testing.T) {
		s.SAdd(ctx, "pg_s", 0, []string{"x", "y", "z"})
		n, _ := s.SCard(ctx, "pg_s", 0)
		if n != 3 {
			t.Errorf("SCard: %d", n)
		}
		ok, _ := s.SIsMember(ctx, "pg_s", 0, "y")
		if !ok {
			t.Error("SIsMember y: expected true")
		}
	})

	t.Run("ZSet", func(t *testing.T) {
		s.ZAdd(ctx, "pg_z", 0, map[string]float64{"a": 1, "b": 2, "c": 3}, store.ZAddOptions{})
		members, err := s.ZRange(ctx, "pg_z", 0, 0, -1)
		if err != nil || len(members) != 3 || members[0] != "a" {
			t.Fatalf("ZRange: %v %v", members, err)
		}
		score, found, _ := s.ZScore(ctx, "pg_z", 0, "b")
		if !found || score != 2 {
			t.Errorf("ZScore b: %v %v", score, found)
		}
	})

	t.Run("MultiDB", func(t *testing.T) {
		s.Set(ctx, "shared", 0, "db0", store.SetOptions{})
		s.Set(ctx, "shared", 1, "db1", store.SetOptions{})
		v0, _, _ := s.Get(ctx, "shared", 0)
		v1, _, _ := s.Get(ctx, "shared", 1)
		if v0 != "db0" || v1 != "db1" {
			t.Errorf("db isolation: %q %q", v0, v1)
		}
	})
}
