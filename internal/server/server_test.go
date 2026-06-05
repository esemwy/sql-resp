package server_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.smy.com/work/sql-resp/internal/db"
	"gitlab.smy.com/work/sql-resp/internal/server"
	"gitlab.smy.com/work/sql-resp/internal/store"
)

// newStore creates an in-memory store for tests.
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

// startServer spins up a server on a random port and returns a go-redis client
// pointed at it. The server is shut down when the test ends.
func startServer(t *testing.T, cfg server.Config) *redis.Client {
	t.Helper()

	if cfg.Store == nil {
		cfg.Store = newStore(t)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Addr = ln.Addr().String()
	ln.Close() // release so the server can bind

	srv := server.New(cfg)
	go srv.ListenAndServe() //nolint:errcheck
	t.Cleanup(func() { srv.Close() })

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
	})
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func TestPing(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	res, err := rdb.Ping(ctx).Result()
	if err != nil {
		t.Fatal(err)
	}
	if res != "PONG" {
		t.Errorf("got %q", res)
	}
}

func TestPingWithMessage(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	res, err := rdb.Do(ctx, "PING", "hello").Text()
	if err != nil {
		t.Fatal(err)
	}
	if res != "hello" {
		t.Errorf("got %q", res)
	}
}

func TestEcho(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	res, err := rdb.Do(ctx, "ECHO", "world").Text()
	if err != nil {
		t.Fatal(err)
	}
	if res != "world" {
		t.Errorf("got %q", res)
	}
}

func TestUnknownCommand(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	err := rdb.Do(ctx, "NOTACOMMAND").Err()
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestCommandCount(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	n, err := rdb.Do(ctx, "COMMAND", "COUNT").Int()
	if err != nil {
		t.Fatal(err)
	}
	if n <= 0 {
		t.Errorf("expected positive command count, got %d", n)
	}
}

func TestAuthRequired(t *testing.T) {
	srv := server.New(server.Config{Password: "secret"})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	// Reassign so we can set the addr.
	srv2 := server.New(server.Config{Addr: addr, Password: "secret"})
	go srv2.ListenAndServe() //nolint:errcheck
	t.Cleanup(func() { srv2.Close() })
	_ = srv

	// Without password → should fail.
	noAuth := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { noAuth.Close() })
	if err := noAuth.Ping(context.Background()).Err(); err == nil {
		t.Fatal("expected auth error without password")
	}

	// With correct password → should succeed.
	withAuth := redis.NewClient(&redis.Options{Addr: addr, Password: "secret"})
	t.Cleanup(func() { withAuth.Close() })
	if err := withAuth.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("unexpected error with correct password: %v", err)
	}
}

func TestWrongPassword(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()

	srv := server.New(server.Config{Addr: addr, Password: "correct"})
	go srv.ListenAndServe() //nolint:errcheck
	t.Cleanup(func() { srv.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: addr, Password: "wrong"})
	t.Cleanup(func() { rdb.Close() })

	if err := rdb.Ping(context.Background()).Err(); err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestPipelining(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	pipe := rdb.Pipeline()
	pings := make([]*redis.Cmd, 5)
	for i := range pings {
		pings[i] = pipe.Do(ctx, "PING", fmt.Sprintf("msg%d", i))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatal(err)
	}
	for i, p := range pings {
		got, err := p.Text()
		if err != nil {
			t.Errorf("pipeline[%d]: %v", i, err)
			continue
		}
		if got != fmt.Sprintf("msg%d", i) {
			t.Errorf("pipeline[%d]: got %q", i, got)
		}
	}
}

// ── String command integration tests ────────────────────────────────────────

func TestSetGet(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	if err := rdb.Set(ctx, "k", "v", 0).Err(); err != nil {
		t.Fatal(err)
	}
	val, err := rdb.Get(ctx, "k").Result()
	if err != nil || val != "v" {
		t.Fatalf("Get: %q %v", val, err)
	}
}

func TestGetMissing(t *testing.T) {
	rdb := startServer(t, server.Config{})
	err := rdb.Get(context.Background(), "nope").Err()
	if err != redis.Nil {
		t.Fatalf("expected redis.Nil, got %v", err)
	}
}

func TestSetWithEX(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.Set(ctx, "k", "v", 2*time.Second)

	ttl, err := rdb.TTL(ctx, "k").Result()
	if err != nil || ttl <= 0 || ttl > 2*time.Second {
		t.Fatalf("TTL: %v %v", ttl, err)
	}
}

func TestSetNX(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.Set(ctx, "k", "original", 0)

	ok, err := rdb.SetNX(ctx, "k", "new", 0).Result()
	if err != nil || ok {
		t.Fatalf("SETNX should return false: %v %v", ok, err)
	}
	val, _ := rdb.Get(ctx, "k").Result()
	if val != "original" {
		t.Errorf("value changed: %q", val)
	}
}

func TestDel(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.Set(ctx, "a", "1", 0)
	rdb.Set(ctx, "b", "2", 0)

	n, err := rdb.Del(ctx, "a", "b", "missing").Result()
	if err != nil || n != 2 {
		t.Fatalf("DEL: %d %v", n, err)
	}
}

func TestExists(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.Set(ctx, "a", "1", 0)

	n, err := rdb.Exists(ctx, "a", "nope").Result()
	if err != nil || n != 1 {
		t.Fatalf("EXISTS: %d %v", n, err)
	}
}

func TestExpireTTL(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.Set(ctx, "k", "v", 0)
	rdb.Expire(ctx, "k", 100*time.Second)

	ttl, err := rdb.TTL(ctx, "k").Result()
	if err != nil || ttl <= 0 {
		t.Fatalf("TTL: %v %v", ttl, err)
	}
}

func TestTTLNoExpiry(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.Set(ctx, "k", "v", 0)
	ttl, _ := rdb.TTL(ctx, "k").Result()
	// go-redis v9 returns time.Duration(-1) for "no expiry"
	if ttl != time.Duration(-1) {
		t.Errorf("expected -1, got %v", ttl)
	}
}

func TestTTLMissing(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ttl, _ := rdb.TTL(context.Background(), "nope").Result()
	// go-redis v9 returns time.Duration(-2) for "key not found"
	if ttl != time.Duration(-2) {
		t.Errorf("expected -2, got %v", ttl)
	}
}

func TestPersist(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.Set(ctx, "k", "v", 100*time.Second)
	rdb.Persist(ctx, "k")
	ttl, _ := rdb.TTL(ctx, "k").Result()
	if ttl != time.Duration(-1) {
		t.Errorf("expected -1 after PERSIST, got %v", ttl)
	}
}

func TestIncrDecr(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	n, err := rdb.Incr(ctx, "counter").Result()
	if err != nil || n != 1 {
		t.Fatalf("INCR: %d %v", n, err)
	}
	n, err = rdb.IncrBy(ctx, "counter", 9).Result()
	if err != nil || n != 10 {
		t.Fatalf("INCRBY: %d %v", n, err)
	}
	n, err = rdb.Decr(ctx, "counter").Result()
	if err != nil || n != 9 {
		t.Fatalf("DECR: %d %v", n, err)
	}
	n, err = rdb.DecrBy(ctx, "counter", 4).Result()
	if err != nil || n != 5 {
		t.Fatalf("DECRBY: %d %v", n, err)
	}
}

func TestMGetMSet(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.MSet(ctx, "a", "1", "b", "2")

	vals, err := rdb.MGet(ctx, "a", "b", "c").Result()
	if err != nil || len(vals) != 3 {
		t.Fatalf("MGET: %v %v", vals, err)
	}
	if vals[0] != "1" || vals[1] != "2" || vals[2] != nil {
		t.Errorf("MGET values: %v", vals)
	}
}

func TestGetSet(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.Set(ctx, "k", "old", 0)

	old, err := rdb.GetSet(ctx, "k", "new").Result()
	if err != nil || old != "old" {
		t.Fatalf("GETSET: %q %v", old, err)
	}
	val, _ := rdb.Get(ctx, "k").Result()
	if val != "new" {
		t.Errorf("new value: %q", val)
	}
}

func TestAppendStrlen(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	n, err := rdb.Append(ctx, "k", "hello").Result()
	if err != nil || n != 5 {
		t.Fatalf("APPEND: %d %v", n, err)
	}
	n, err = rdb.Append(ctx, "k", " world").Result()
	if err != nil || n != 11 {
		t.Fatalf("APPEND2: %d %v", n, err)
	}
	slen, err := rdb.StrLen(ctx, "k").Result()
	if err != nil || slen != 11 {
		t.Fatalf("STRLEN: %d %v", slen, err)
	}
}

func TestType(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.Set(ctx, "k", "v", 0)

	typ, err := rdb.Type(ctx, "k").Result()
	if err != nil || typ != "string" {
		t.Fatalf("TYPE: %q %v", typ, err)
	}
	typ, _ = rdb.Type(ctx, "missing").Result()
	if typ != "none" {
		t.Errorf("missing type: %q", typ)
	}
}

func TestDBSize(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.Set(ctx, "a", "1", 0)
	rdb.Set(ctx, "b", "2", 0)

	n, err := rdb.DBSize(ctx).Result()
	if err != nil || n != 2 {
		t.Fatalf("DBSIZE: %d %v", n, err)
	}
}

func TestFlushDB(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.Set(ctx, "a", "1", 0)
	rdb.FlushDB(ctx)

	n, _ := rdb.DBSize(ctx).Result()
	if n != 0 {
		t.Errorf("expected 0 after FLUSHDB, got %d", n)
	}
}

func TestSelect(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	// go-redis handles SELECT transparently via the DB option; test direct command.
	err := rdb.Do(ctx, "SELECT", "1").Err()
	if err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	err = rdb.Do(ctx, "SELECT", "16").Err()
	if err == nil {
		t.Fatal("SELECT 16 should fail")
	}
}
