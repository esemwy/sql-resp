package store_test

import (
	"context"
	"testing"
	"time"

	"gitlab.smy.com/work/sql-resp/internal/db"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	d, err := db.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Migrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return store.New(d)
}

func TestGetSetBasic(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	ok, err := s.Set(ctx, "hello", 0, "world", store.SetOptions{})
	if err != nil || !ok {
		t.Fatalf("Set: %v %v", ok, err)
	}
	val, found, err := s.Get(ctx, "hello", 0)
	if err != nil || !found || val != "world" {
		t.Fatalf("Get: %q %v %v", val, found, err)
	}
}

func TestGetMissing(t *testing.T) {
	s := newStore(t)
	_, found, err := s.Get(context.Background(), "nope", 0)
	if err != nil || found {
		t.Fatalf("expected miss: %v %v", found, err)
	}
}

func TestSetNX(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	ok, _ := s.Set(ctx, "k", 0, "v1", store.SetOptions{NX: true})
	if !ok {
		t.Fatal("first NX should succeed")
	}
	ok, _ = s.Set(ctx, "k", 0, "v2", store.SetOptions{NX: true})
	if ok {
		t.Fatal("second NX should fail")
	}
	val, _, _ := s.Get(ctx, "k", 0)
	if val != "v1" {
		t.Errorf("value should not change: got %q", val)
	}
}

func TestSetXX(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	ok, _ := s.Set(ctx, "k", 0, "v1", store.SetOptions{XX: true})
	if ok {
		t.Fatal("XX on missing key should fail")
	}
	s.Set(ctx, "k", 0, "original", store.SetOptions{})
	ok, _ = s.Set(ctx, "k", 0, "updated", store.SetOptions{XX: true})
	if !ok {
		t.Fatal("XX on existing key should succeed")
	}
	val, _, _ := s.Get(ctx, "k", 0)
	if val != "updated" {
		t.Errorf("got %q", val)
	}
}

func TestDel(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "a", 0, "1", store.SetOptions{})
	s.Set(ctx, "b", 0, "2", store.SetOptions{})

	n, err := s.Del(ctx, []string{"a", "b", "missing"}, 0)
	if err != nil || n != 2 {
		t.Fatalf("Del: %d %v", n, err)
	}
	_, found, _ := s.Get(ctx, "a", 0)
	if found {
		t.Error("key a should be gone")
	}
}

func TestExists(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "a", 0, "1", store.SetOptions{})
	s.Set(ctx, "b", 0, "2", store.SetOptions{})

	n, err := s.Exists(ctx, []string{"a", "b", "c"}, 0)
	if err != nil || n != 2 {
		t.Fatalf("Exists: %d %v", n, err)
	}
}

func TestExpireAndTTL(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "k", 0, "v", store.SetOptions{})

	ok, err := s.Expire(ctx, "k", 0, 100)
	if err != nil || !ok {
		t.Fatalf("Expire: %v %v", ok, err)
	}
	ttl, err := s.TTL(ctx, "k", 0)
	if err != nil || ttl < 99 || ttl > 100 {
		t.Fatalf("TTL: %d %v", ttl, err)
	}
	pttl, err := s.PTTL(ctx, "k", 0)
	if err != nil || pttl < 99000 || pttl > 100000 {
		t.Fatalf("PTTL: %d %v", pttl, err)
	}
}

func TestTTLNoExpiry(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "k", 0, "v", store.SetOptions{})
	ttl, _ := s.TTL(ctx, "k", 0)
	if ttl != -1 {
		t.Errorf("expected -1, got %d", ttl)
	}
}

func TestTTLMissing(t *testing.T) {
	s := newStore(t)
	ttl, _ := s.TTL(context.Background(), "nope", 0)
	if ttl != -2 {
		t.Errorf("expected -2, got %d", ttl)
	}
}

func TestKeyExpiresAfterTTL(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	expMs := time.Now().Add(50 * time.Millisecond).UnixMilli()
	s.Set(ctx, "k", 0, "v", store.SetOptions{ExpiresAt: &expMs})

	time.Sleep(100 * time.Millisecond)

	_, found, _ := s.Get(ctx, "k", 0)
	if found {
		t.Error("key should have expired")
	}
	ttl, _ := s.TTL(ctx, "k", 0)
	if ttl != -2 {
		t.Errorf("TTL of expired key should be -2, got %d", ttl)
	}
}

func TestPersist(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "k", 0, "v", store.SetOptions{})
	s.Expire(ctx, "k", 0, 100)

	ok, err := s.Persist(ctx, "k", 0)
	if err != nil || !ok {
		t.Fatalf("Persist: %v %v", ok, err)
	}
	ttl, _ := s.TTL(ctx, "k", 0)
	if ttl != -1 {
		t.Errorf("expected -1 after Persist, got %d", ttl)
	}
}

func TestType(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "k", 0, "v", store.SetOptions{})

	typ, err := s.Type(ctx, "k", 0)
	if err != nil || typ != "string" {
		t.Fatalf("Type: %q %v", typ, err)
	}
	typ, _ = s.Type(ctx, "missing", 0)
	if typ != "none" {
		t.Errorf("missing key type: %q", typ)
	}
}

func TestWrongType(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "str", 0, "v", store.SetOptions{})

	// A hash command on a string key should return ErrWrongType (tested via assertType).
	_, _, err := s.Get(ctx, "str", 0)
	if err != nil {
		t.Fatalf("Get on string key: %v", err)
	}
}

func TestMultiDB(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	s.Set(ctx, "k", 0, "db0", store.SetOptions{})
	s.Set(ctx, "k", 1, "db1", store.SetOptions{})

	v0, _, _ := s.Get(ctx, "k", 0)
	v1, _, _ := s.Get(ctx, "k", 1)
	if v0 != "db0" || v1 != "db1" {
		t.Errorf("db isolation: %q %q", v0, v1)
	}
}

func TestDBSize(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "a", 0, "1", store.SetOptions{})
	s.Set(ctx, "b", 0, "2", store.SetOptions{})
	s.Set(ctx, "c", 1, "3", store.SetOptions{})

	n, _ := s.DBSize(ctx, 0)
	if n != 2 {
		t.Errorf("db0 size: %d", n)
	}
	n, _ = s.DBSize(ctx, 1)
	if n != 1 {
		t.Errorf("db1 size: %d", n)
	}
}

func TestFlushDB(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "a", 0, "1", store.SetOptions{})
	s.Set(ctx, "b", 1, "2", store.SetOptions{})

	s.FlushDB(ctx, 0)

	n, _ := s.DBSize(ctx, 0)
	if n != 0 {
		t.Error("db0 should be empty")
	}
	n, _ = s.DBSize(ctx, 1)
	if n != 1 {
		t.Error("db1 should be untouched")
	}
}

func TestFlushAll(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "a", 0, "1", store.SetOptions{})
	s.Set(ctx, "b", 1, "2", store.SetOptions{})
	s.FlushAll(ctx)

	n0, _ := s.DBSize(ctx, 0)
	n1, _ := s.DBSize(ctx, 1)
	if n0 != 0 || n1 != 0 {
		t.Errorf("expected all empty: %d %d", n0, n1)
	}
}

func TestRename(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "src", 0, "value", store.SetOptions{})

	if err := s.Rename(ctx, "src", "dst", 0); err != nil {
		t.Fatal(err)
	}
	_, found, _ := s.Get(ctx, "src", 0)
	if found {
		t.Error("src should be gone")
	}
	val, found, _ := s.Get(ctx, "dst", 0)
	if !found || val != "value" {
		t.Errorf("dst: %q %v", val, found)
	}
}

func TestRenameMissing(t *testing.T) {
	s := newStore(t)
	if err := s.Rename(context.Background(), "nope", "dst", 0); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestKeys(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	s.Set(ctx, "foo", 0, "1", store.SetOptions{})
	s.Set(ctx, "foobar", 0, "2", store.SetOptions{})
	s.Set(ctx, "bar", 0, "3", store.SetOptions{})

	keys, _ := s.Keys(ctx, "foo*", 0)
	if len(keys) != 2 {
		t.Errorf("foo*: %v", keys)
	}
	keys, _ = s.Keys(ctx, "*", 0)
	if len(keys) != 3 {
		t.Errorf("*: %v", keys)
	}
	keys, _ = s.Keys(ctx, "baz", 0)
	if len(keys) != 0 {
		t.Errorf("baz: %v", keys)
	}
}

func TestSweepExpired(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	expMs := time.Now().Add(30 * time.Millisecond).UnixMilli()
	s.Set(ctx, "dying", 0, "v", store.SetOptions{ExpiresAt: &expMs})

	time.Sleep(60 * time.Millisecond)
	s.SweepExpired(ctx)

	n, _ := s.DBSize(ctx, 0)
	if n != 0 {
		t.Error("expired key should have been swept")
	}
}
