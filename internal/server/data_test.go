package server_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.smy.com/work/sql-resp/internal/server"
)

// ── Hash commands ────────────────────────────────────────────────────────────

func TestHSetHGet(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	n, err := rdb.HSet(ctx, "h", "f1", "v1", "f2", "v2").Result()
	if err != nil || n != 2 {
		t.Fatalf("HSET: %d %v", n, err)
	}
	val, err := rdb.HGet(ctx, "h", "f1").Result()
	if err != nil || val != "v1" {
		t.Fatalf("HGET: %q %v", val, err)
	}
	err = rdb.HGet(ctx, "h", "missing").Err()
	if err != redis.Nil {
		t.Errorf("missing field: %v", err)
	}
}

func TestHGetAll(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.HSet(ctx, "h", "a", "1", "b", "2")

	m, err := rdb.HGetAll(ctx, "h").Result()
	if err != nil || len(m) != 2 || m["a"] != "1" {
		t.Fatalf("HGETALL: %v %v", m, err)
	}
}

func TestHDel(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.HSet(ctx, "h", "f", "v")

	n, err := rdb.HDel(ctx, "h", "f", "nope").Result()
	if err != nil || n != 1 {
		t.Fatalf("HDEL: %d %v", n, err)
	}
}

func TestHLen(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.HSet(ctx, "h", "a", "1", "b", "2")

	n, err := rdb.HLen(ctx, "h").Result()
	if err != nil || n != 2 {
		t.Fatalf("HLEN: %d %v", n, err)
	}
}

func TestHMGet(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.HSet(ctx, "h", "a", "1", "b", "2")

	vals, err := rdb.HMGet(ctx, "h", "a", "b", "c").Result()
	if err != nil || len(vals) != 3 || vals[0] != "1" || vals[2] != nil {
		t.Fatalf("HMGET: %v %v", vals, err)
	}
}

// ── List commands ────────────────────────────────────────────────────────────

func TestLPushLRange(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	rdb.LPush(ctx, "l", "c", "b", "a")
	vals, err := rdb.LRange(ctx, "l", 0, -1).Result()
	if err != nil || len(vals) != 3 || vals[0] != "a" {
		t.Fatalf("LRANGE: %v %v", vals, err)
	}
}

func TestRPushLPop(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	rdb.RPush(ctx, "l", "x", "y", "z")
	val, err := rdb.LPop(ctx, "l").Result()
	if err != nil || val != "x" {
		t.Fatalf("LPOP: %q %v", val, err)
	}
}

func TestLLen(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.RPush(ctx, "l", "a", "b", "c")

	n, err := rdb.LLen(ctx, "l").Result()
	if err != nil || n != 3 {
		t.Fatalf("LLEN: %d %v", n, err)
	}
}

func TestLIndex(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.RPush(ctx, "l", "a", "b", "c")

	val, err := rdb.LIndex(ctx, "l", 1).Result()
	if err != nil || val != "b" {
		t.Fatalf("LINDEX: %q %v", val, err)
	}
}

// ── Set commands ─────────────────────────────────────────────────────────────

func TestSAddSMembers(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	n, err := rdb.SAdd(ctx, "s", "a", "b", "c").Result()
	if err != nil || n != 3 {
		t.Fatalf("SADD: %d %v", n, err)
	}
	members, err := rdb.SMembers(ctx, "s").Result()
	if err != nil || len(members) != 3 {
		t.Fatalf("SMEMBERS: %v %v", members, err)
	}
}

func TestSIsMember(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.SAdd(ctx, "s", "yes")

	ok, err := rdb.SIsMember(ctx, "s", "yes").Result()
	if err != nil || !ok {
		t.Fatalf("SISMEMBER yes: %v %v", ok, err)
	}
	ok, err = rdb.SIsMember(ctx, "s", "no").Result()
	if err != nil || ok {
		t.Fatalf("SISMEMBER no: %v %v", ok, err)
	}
}

func TestSUnionSInterSDiff(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.SAdd(ctx, "s1", "a", "b", "c")
	rdb.SAdd(ctx, "s2", "b", "c", "d")

	union, _ := rdb.SUnion(ctx, "s1", "s2").Result()
	if len(union) != 4 {
		t.Errorf("SUNION: %v", union)
	}
	inter, _ := rdb.SInter(ctx, "s1", "s2").Result()
	if len(inter) != 2 {
		t.Errorf("SINTER: %v", inter)
	}
	diff, _ := rdb.SDiff(ctx, "s1", "s2").Result()
	if len(diff) != 1 || diff[0] != "a" {
		t.Errorf("SDIFF: %v", diff)
	}
}

// ── Sorted set commands ──────────────────────────────────────────────────────

func TestZAddZRange(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	rdb.ZAdd(ctx, "z", redis.Z{Score: 1, Member: "a"}, redis.Z{Score: 2, Member: "b"}, redis.Z{Score: 3, Member: "c"})
	members, err := rdb.ZRange(ctx, "z", 0, -1).Result()
	if err != nil || len(members) != 3 || members[0] != "a" {
		t.Fatalf("ZRANGE: %v %v", members, err)
	}
}

func TestZScore(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.ZAdd(ctx, "z", redis.Z{Score: 42, Member: "m"})

	score, err := rdb.ZScore(ctx, "z", "m").Result()
	if err != nil || score != 42 {
		t.Fatalf("ZSCORE: %v %v", score, err)
	}
}

func TestZRank(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.ZAdd(ctx, "z", redis.Z{Score: 1, Member: "a"}, redis.Z{Score: 2, Member: "b"})

	rank, err := rdb.ZRank(ctx, "z", "b").Result()
	if err != nil || rank != 1 {
		t.Fatalf("ZRANK: %d %v", rank, err)
	}
}

func TestZRangeByScore(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.ZAdd(ctx, "z",
		redis.Z{Score: 1, Member: "a"},
		redis.Z{Score: 2, Member: "b"},
		redis.Z{Score: 3, Member: "c"},
	)

	members, err := rdb.ZRangeByScore(ctx, "z", &redis.ZRangeBy{Min: "1", Max: "2"}).Result()
	if err != nil || len(members) != 2 {
		t.Fatalf("ZRANGEBYSCORE: %v %v", members, err)
	}
}

func TestZIncrBy(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()
	rdb.ZAdd(ctx, "z", redis.Z{Score: 1, Member: "m"})

	newScore, err := rdb.ZIncrBy(ctx, "z", 4, "m").Result()
	if err != nil || newScore != 5 {
		t.Fatalf("ZINCRBY: %v %v", newScore, err)
	}
}

// ── MULTI/EXEC ───────────────────────────────────────────────────────────────

func TestMultiExec(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	pipe := rdb.TxPipeline()
	pipe.Set(ctx, "k1", "v1", 0)
	pipe.Set(ctx, "k2", "v2", 0)
	pipe.Get(ctx, "k1")
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		t.Fatalf("EXEC: %v", err)
	}
	if len(cmds) != 3 {
		t.Fatalf("expected 3 results, got %d", len(cmds))
	}
}

func TestMultiDiscard(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	if err := rdb.Do(ctx, "MULTI").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Do(ctx, "SET", "k", "v").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.Do(ctx, "DISCARD").Err(); err != nil {
		t.Fatal(err)
	}
	// Key should not exist.
	err := rdb.Get(ctx, "k").Err()
	if err != redis.Nil {
		t.Errorf("expected nil after DISCARD, got %v", err)
	}
}

// ── Pub/Sub ──────────────────────────────────────────────────────────────────

func TestPubSub(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	ps := rdb.Subscribe(ctx, "testchan")
	defer ps.Close()

	// Consume the subscribe confirmation before publishing.
	_, err := ps.ReceiveTimeout(ctx, time.Second)
	if err != nil {
		t.Fatalf("subscribe confirmation: %v", err)
	}

	n, err := rdb.Publish(ctx, "testchan", "hello").Result()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 subscriber, got %d", n)
	}

	msg, err := ps.ReceiveTimeout(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := msg.(*redis.Message)
	if !ok || m.Payload != "hello" {
		t.Errorf("unexpected message type %T: %v", msg, msg)
	}
}

// ── Background sweep ─────────────────────────────────────────────────────────

func TestKeyExpiresOnServer(t *testing.T) {
	rdb := startServer(t, server.Config{})
	ctx := context.Background()

	rdb.Set(ctx, "expiring", "val", 50*time.Millisecond)
	time.Sleep(150 * time.Millisecond)

	err := rdb.Get(ctx, "expiring").Err()
	if err != redis.Nil {
		t.Errorf("expected key to have expired, got %v", err)
	}
}
