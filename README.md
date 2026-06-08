# sql-resp

A drop-in Redis replacement that stores data in a SQL database. It speaks RESP2 over TCP — existing Redis clients connect to it without any code changes.

<a href='https://ko-fi.com/N6A8210Y3L' target='_blank'><img height='36' style='border:0px;height:36px;' src='https://storage.ko-fi.com/cdn/kofi2.png?v=6' border='0' alt='Buy Me a Coffee at ko-fi.com' /></a>

## Why

Redis requires a separate, dedicated process and stores everything in memory. sql-resp stores data in a SQL database instead, giving you:

- **Durability by default** — every write is committed before the client gets a response, with no separate persistence configuration required
- **Existing infrastructure** — run against a PostgreSQL instance you already operate, or use SQLite for lightweight deployments
- **Standard tooling** — inspect, query, and back up your data with any SQL client

## Status

Core data structures and the full v1 command set are implemented and tested against the official `go-redis` client. PostgreSQL and SQLite are both supported.

## Quick start

```bash
# Build
go build -o sql-resp ./cmd/sql-resp

# Run with SQLite (creates sql-resp.db in the current directory)
./sql-resp

# Run in-memory (no persistence, fast for dev/test)
./sql-resp --no-persist

# Point any Redis client at it
redis-cli -p 6379 PING
# => PONG
```

## Configuration

Configuration is loaded from a `redis.conf`-style file, with CLI flags as overrides and environment variables for secrets.

```bash
./sql-resp --config /etc/sql-resp.conf --port 6380
```

**Config file** (`redis.conf` style):

```
port 6379
requirepass mysecret

# SQLite backend (default)
backend sqlite
dsn /var/lib/sql-resp/data.db

# PostgreSQL backend
# backend postgres
# dsn postgres://user:password@localhost:5432/mydb?sslmode=disable

# TLS
tls-cert-file /etc/ssl/certs/server.crt
tls-key-file  /etc/ssl/private/server.key

# TTL sweep interval in milliseconds (default: 100)
hz 100

# In-memory mode (data lost on restart)
# save ""
```

**Supported config keys:**

| Key | Default | Description |
|-----|---------|-------------|
| `port` | `6379` | TCP port to listen on |
| `requirepass` | _(none)_ | Password for AUTH |
| `tls-cert-file` | _(none)_ | TLS certificate path |
| `tls-key-file` | _(none)_ | TLS private key path |
| `backend` | `sqlite` | `sqlite` or `postgres` |
| `dsn` | `sql-resp.db` | Database connection string |
| `databases` | `16` | Number of databases (0–15) |
| `hz` | `100` | Background expiry sweep interval (ms) |
| `save` | _(persist)_ | Set to `""` for in-memory mode |

**Environment variables** (override config file):

| Variable | Config key |
|----------|------------|
| `SQL_RESP_PASSWORD` | `requirepass` |
| `SQL_RESP_DSN` | `dsn` |

## Backends

### SQLite

The default backend. Suitable for single-node deployments, development, and any workload that doesn't require a separate database server.

```
backend sqlite
dsn /path/to/data.db
```

Uses WAL mode for better read concurrency. Pass `:memory:` as the DSN (or use `--no-persist`) for a fast in-memory instance.

### PostgreSQL

Suitable for production deployments where you want centralised storage, replication, or an existing PostgreSQL infrastructure.

```
backend postgres
dsn postgres://user:password@localhost:5432/mydb?sslmode=disable
```

Schema is created automatically on first startup and is idempotent on subsequent restarts.

## Supported commands

### Strings
`GET` `SET` `DEL` `EXISTS` `EXPIRE` `PEXPIRE` `TTL` `PTTL` `PERSIST`
`INCR` `INCRBY` `DECR` `DECRBY` `MGET` `MSET` `SETNX` `GETSET`
`APPEND` `STRLEN`

`SET` supports all standard options: `EX`, `PX`, `NX`, `XX`.

### Hashes
`HGET` `HSET` `HDEL` `HEXISTS` `HGETALL` `HKEYS` `HVALS` `HLEN` `HMGET` `HMSET`

### Lists
`LPUSH` `RPUSH` `LPOP` `RPOP` `LLEN` `LRANGE` `LINDEX` `LSET` `LINSERT` `LREM`

### Sets
`SADD` `SREM` `SMEMBERS` `SISMEMBER` `SCARD` `SUNION` `SINTER` `SDIFF`

### Sorted sets
`ZADD` `ZREM` `ZSCORE` `ZRANK` `ZRANGE` `ZRANGEBYSCORE` `ZCARD` `ZCOUNT` `ZINCRBY`

`ZADD` supports `NX` and `XX` flags. `ZRANGEBYSCORE` supports `-inf`, `+inf`, and exclusive bounds with `(`.

### Transactions
`MULTI` `EXEC` `DISCARD`

Commands queued in a `MULTI` block execute inside a single SQL transaction. Runtime errors within `EXEC` are returned per-command without rolling back the rest, matching Redis semantics.

### Pub/Sub
`SUBSCRIBE` `UNSUBSCRIBE` `PUBLISH`

In-process fan-out with no SQL involvement. Messages are not persisted.

### Server
`PING` `ECHO` `SELECT` `DBSIZE` `FLUSHDB` `FLUSHALL` `KEYS` `TYPE`
`RENAME` `RANDOMKEY` `AUTH` `QUIT` `COMMAND` `COMMAND COUNT` `INFO`

## Architecture

```
cmd/sql-resp/          binary entry point, config loading, sweep goroutine
internal/
  resp/                RESP2 encoder/decoder — no dependencies
  config/              redis.conf-style file + CLI flags + env vars
  db/                  DB interface + SQLite and PostgreSQL drivers
  store/               all Redis data structure operations (wraps db/)
  commands/            command registry + handlers (wraps store/)
  server/              TCP listener, connection loop, pub/sub dispatch
  pubsub/              in-memory pub/sub broker
```

The `db/` package defines a `DB` interface that hides the backend. Swapping from SQLite to PostgreSQL requires only a config change — no application code changes.

Commands register themselves in `init()` via the command registry. Adding a new command is a single file with no changes to the dispatch loop.

## Development

**Run tests** (no external dependencies required):

```bash
go test ./...
```

**Run Postgres integration tests** (requires a running Postgres instance):

```bash
# Default DSN: postgres://postgres:postgres@localhost:5432/sql_resp_test?sslmode=disable
go test -tags postgres ./internal/db/... ./internal/store/...

# Custom DSN
SQL_RESP_DSN=postgres://... go test -tags postgres ./internal/db/... ./internal/store/...
```

**Run with Docker Compose** (Postgres example):

```yaml
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: sql_resp
    ports: ["5432:5432"]

  sql-resp:
    build: .
    environment:
      SQL_RESP_DSN: postgres://postgres:postgres@db:5432/sql_resp?sslmode=disable
    command: ["--config", "/etc/sql-resp.conf"]
    ports: ["6379:6379"]
    depends_on: [db]
```

## Not yet implemented

The following Redis features are out of scope for v1 and tracked as future issues:

- `SCAN` cursor-based key iteration
- `WATCH` / optimistic locking for transactions
- Keyspace notifications
- Streams (`XADD`, `XREAD`, consumer groups)
- Bitfield operations
- RESP3 protocol
- ACL / multi-user auth
- Replication and clustering
- Lua scripting (`EVAL`)
