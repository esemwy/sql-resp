## Problem Statement

Redis is the dominant in-memory data structure store, but it is difficult to replace in environments that require SQL-backed durability, auditability, or integration with existing relational database infrastructure. Operators who want Redis semantics without a separate Redis process — or who need to store data durably in an open-source SQL database — have no drop-in alternative that speaks the RESP wire protocol.

## Solution

Build a drop-in Redis replacement that speaks RESP2 over TCP and stores all data in a SQL database. Applications point their existing Redis client at the new server and nothing else changes. The server targets open-source SQL backends (SQLite for development and testing, PostgreSQL for production), exposes a pluggable DB abstraction layer, and implements the most commonly used Redis commands first, expanding toward full parity over time.

## User Stories

1. As an application developer, I want to point my existing Redis client at sql-resp without changing any application code, so that I can swap backends without a migration.
2. As an application developer, I want GET and SET to work identically to Redis, so that my cache layer continues to function correctly.
3. As an application developer, I want EXPIRE, TTL, and PTTL to work correctly, so that my cache expiry logic is preserved.
4. As an application developer, I want INCR, INCRBY, DECR, and DECRBY to be atomic, so that counters remain correct under concurrent access.
5. As an application developer, I want MGET and MSET to work, so that I can batch reads and writes efficiently.
6. As an application developer, I want hash commands (HGET, HSET, HGETALL, etc.) to work, so that I can store structured objects.
7. As an application developer, I want list commands (LPUSH, RPUSH, LPOP, RPOP, LRANGE, etc.) to work, so that I can implement queues and stacks.
8. As an application developer, I want set commands (SADD, SREM, SMEMBERS, SUNION, SINTER, SDIFF, etc.) to work, so that I can perform membership and set algebra operations.
9. As an application developer, I want sorted set commands (ZADD, ZRANGE, ZRANGEBYSCORE, ZRANK, ZSCORE, etc.) to work, so that I can implement leaderboards and priority queues.
10. As an application developer, I want MULTI/EXEC to queue commands and execute them atomically, so that multi-step operations are safe under concurrent access.
11. As an application developer, I want DISCARD to cancel a queued transaction, so that I can abort a MULTI block cleanly.
12. As an application developer, I want SUBSCRIBE, UNSUBSCRIBE, and PUBLISH to work, so that I can use Redis pub/sub messaging patterns.
13. As an application developer, I want AUTH to work with a configured password, so that my existing auth-using clients connect without modification.
14. As an application developer, I want SELECT to switch between databases 0–15, so that multi-tenant or multi-purpose applications using database isolation continue to work.
15. As an application developer, I want KEYS with glob patterns to work, so that I can scan for keys matching a prefix or pattern.
16. As an application developer, I want RENAME to work atomically, so that I can move keys without races.
17. As an application developer, I want TYPE to return the correct data type for a key, so that my code can introspect key types.
18. As an application developer, I want COMMAND and COMMAND COUNT to return valid responses, so that Redis clients that call these on startup connect without warnings.
19. As an application developer, I want INFO to return a valid response with basic server stats, so that monitoring tools and health checks do not break.
20. As an application developer, I want PING and ECHO to work, so that connection health checks succeed.
21. As an application developer, I want DEL to delete one or more keys atomically, so that cleanup operations are correct.
22. As an application developer, I want EXISTS to check key presence, so that conditional logic works correctly.
23. As an application developer, I want PERSIST to remove a TTL from a key, so that I can promote a temporary key to permanent.
24. As an application developer, I want DBSIZE to return the number of keys in the current database, so that I can monitor key counts.
25. As an application developer, I want FLUSHDB and FLUSHALL to clear keys, so that test setup/teardown and emergency operations work.
26. As an application developer, I want RANDOMKEY to return a random key, so that sampling-based code continues to work.
27. As an application developer, I want APPEND and STRLEN to work on string keys, so that string accumulation patterns are supported.
28. As an application developer, I want SETNX and GETSET to work atomically, so that lock and swap patterns function correctly.
29. As an application developer, I want HKEYS, HVALS, HLEN, HMGET, HMSET, and HEXISTS to work, so that all common hash operations are available.
30. As an application developer, I want LINDEX, LSET, LINSERT, LREM, and LLEN to work, so that list manipulation is fully supported.
31. As an application developer, I want SISMEMBER and SCARD to work, so that set membership checks are available.
32. As an application developer, I want ZREM, ZCARD, ZCOUNT, and ZINCRBY to work, so that sorted set management is complete.
33. As an operator, I want to configure the server via a redis.conf-style file, so that I can use familiar configuration patterns.
34. As an operator, I want CLI flags to override config file values, so that I can make one-off changes without editing the config file.
35. As an operator, I want to set the password and DSN via environment variables, so that secrets are not stored in config files in production.
36. As an operator, I want to configure the listening port, so that I can run the server on a non-default port.
37. As an operator, I want TLS support with configurable cert and key paths, so that connections are encrypted in transit.
38. As an operator, I want to configure the SQL backend DSN, so that I can point the server at any supported database.
39. As an operator, I want to run with SQLite in-memory mode (--no-persist), so that I can deploy a fast, ephemeral cache without a separate database process.
40. As an operator, I want the TTL sweep interval to be configurable, so that I can tune the tradeoff between sweep frequency and DB load.
41. As an operator, I want expired keys to be cleaned up automatically in the background, so that storage does not grow unboundedly.
42. As an operator, I want the server to log connection events and errors, so that I can diagnose issues in production.
43. As a developer contributing to the project, I want a command registry where new commands are added by registering a handler, so that adding commands does not require modifying the dispatch loop.
44. As a developer contributing to the project, I want the DB abstraction layer to define a clear interface, so that adding a new SQL backend requires only implementing that interface.
45. As a developer contributing to the project, I want unit tests for every module, so that regressions are caught before they reach production.
46. As a developer contributing to the project, I want integration tests using the official go-redis client, so that protocol-level compatibility is continuously verified.

## Implementation Decisions

### Modules

- **resp**: Pure RESP2 protocol encoder and decoder. No dependencies on the DB or command layer. Handles inline commands, bulk strings, arrays, errors, and integers. The only entry point to the network protocol.
- **db**: Defines the DB abstraction interface used by the store layer. Provides two driver implementations: one for SQLite (using the `modernc.org/sqlite` CGo-free driver) and one for PostgreSQL (using `pgx`). Connection pooling is managed per driver. The interface is the only thing the rest of the application sees.
- **store**: Implements all Redis data structure operations (strings, lists, hashes, sets, sorted sets, key metadata) on top of the DB interface. All TTL enforcement, lazy expiry checks, and multi-database isolation (`db` column) live here. This is the deepest module — it contains the most logic and changes least often once stable.
- **commands**: Command registry mapping command names (uppercased) to handler functions. Each handler receives a context, a store handle, and a parsed argument list, and returns a RESP value or error. Commands self-register. The registry also handles arity validation before dispatch.
- **server**: TCP listener that accepts connections and spawns a goroutine per connection. Reads bytes from the socket, feeds them to the RESP decoder, dispatches decoded commands through the command registry, and writes RESP-encoded responses back. Manages per-connection MULTI/EXEC transaction state and pub/sub subscription state.
- **pubsub**: In-process message broker. Maintains a map of channel name to list of subscriber connections. SUBSCRIBE registers a connection; PUBLISH fans out a message to all subscribers on a channel; UNSUBSCRIBE removes a connection. No SQL involvement. Protected by a read/write mutex.
- **config**: Parses a redis.conf-style configuration file, applies CLI flag overrides, and reads secrets from environment variables. Exposes a single typed Config struct to the rest of the application.

### Schema

A shared `keys` table tracks key metadata. Each data type has its own table. All tables include a `db` column (0–15) as part of the primary key to support Redis's multiple-database semantics.

- `keys`: key, db, type, expires_at
- `strings`: key, db, value
- `lists`: key, db, idx, value (ordered by idx)
- `hashes`: key, db, field, value
- `sets`: key, db, member
- `zsets`: key, db, member, score

### Concurrency and Atomicity

Every command executes inside a database transaction. Read-only commands use READ COMMITTED isolation. Read-modify-write commands (INCR, GETSET, SETNX, LPOP, RPOP, ZADD with NX/XX, etc.) use SELECT FOR UPDATE on SQLite (deferred transactions) and Postgres to prevent lost updates. MULTI/EXEC queues commands in memory on the connection goroutine and wraps all of them in a single database transaction on EXEC. Runtime errors within an EXEC block are returned per-command without rolling back the transaction, matching Redis semantics.

### TTL and Expiry

Lazy expiry: every read operation checks `expires_at < NOW()` and deletes the key if expired before returning the result. Background sweep: a goroutine runs `DELETE FROM keys WHERE expires_at IS NOT NULL AND expires_at < NOW()` on a configurable interval (default 100ms). The sweep cascades to data tables via foreign key constraints or explicit deletes per backend.

### Authentication and TLS

If `requirepass` is set in config, all connections must issue AUTH before any other command. TLS is configured with cert and key file paths; the TCP listener wraps accepted connections with `tls.Server`. AUTH and TLS are both optional; an unconfigured server accepts connections without credentials (matching default Redis behavior).

### Durability

The default mode is always-durable: every write is committed to the SQL backend before the client receives a response. SQLite WAL mode is enabled by default for better read concurrency. In-memory mode (`--no-persist` flag or `save ""` in config) uses SQLite `:memory:` — data is not persisted across restarts.

### Configuration

Supported config keys (redis.conf-style): `port`, `requirepass`, `tls-cert-file`, `tls-key-file`, `databases`, `hz` (sweep interval), `save` (set to `""` for in-memory mode), `backend` (sqlite or postgres), `dsn`. Environment variables `SQL_RESP_PASSWORD` and `SQL_RESP_DSN` override config file values for secrets.

### Command Dispatch

The command name is uppercased and looked up in the registry map. If not found, an `ERR unknown command` response is returned. Arity is validated before the handler is called. MULTI state is checked before dispatch — if a connection is in MULTI mode, commands are queued rather than executed (with the exception of EXEC, DISCARD, and QUIT).

## Testing Decisions

A good test verifies observable external behavior, not internal implementation details. Tests should not assert on SQL queries, internal struct fields, or goroutine counts. They should assert on what a Redis client observes: the response to a command, the state of a key after an operation, error messages.

### Modules and approach

- **resp**: Unit tests for every RESP type — encode a value, decode the bytes, assert round-trip correctness. Test error cases: truncated input, invalid type bytes, negative lengths.
- **db**: Tests that run the same suite against both the SQLite and Postgres drivers and assert identical behavior. Covers connection, schema migration, basic CRUD, and transaction isolation. SQLite tests run in CI without external dependencies; Postgres tests run against a Docker container.
- **store**: Unit tests for every data structure operation using SQLite `:memory:`. Tests cover happy paths, edge cases (operating on wrong type, key not found, TTL expiry), and atomicity of read-modify-write operations. This is the most important test suite.
- **commands**: Integration tests that spin up the full TCP server with SQLite `:memory:` and connect using the official `go-redis` client. Each command family has a test file. Tests assert on client return values, not internal state.
- **pubsub**: Unit tests for the broker in isolation. Subscribe a mock connection, publish a message, assert it was delivered. Test unsubscribe, multiple subscribers on one channel, and publish to a channel with no subscribers.
- **server**: Integration tests via `go-redis` that verify connection lifecycle (AUTH rejection, MULTI/EXEC sequencing, pub/sub blocking), pipelining, and concurrent client behavior.

There is no prior art in this codebase — it is greenfield. The test patterns described above establish the baseline.

## Out of Scope

- RESP3 protocol support (may be added in a future release)
- Redis Cluster and Sentinel (high-availability and sharding)
- ACL (Access Control Lists) — multi-user permission system
- WATCH command (optimistic locking for transactions)
- Keyspace notifications (SUBSCRIBE to __keyevent__ channels)
- Lua scripting (EVAL, EVALSHA)
- Streams data type (XADD, XREAD, consumer groups)
- Bitfield and bitmap operations (BITCOUNT, BITOP, BITFIELD)
- SCAN cursor-based iteration
- OBJECT encoding/idletime introspection
- RDB/AOF persistence format compatibility
- WAIT, DEBUG, LATENCY, SLOWLOG, MONITOR commands
- Replication (REPLICAOF, SLAVEOF)
- Direct database durability tuning knobs (exposed to operators)

## Further Notes

- The project is intended to be long-lived and expand command coverage from most-popular to most-obscure over time. The command registry pattern is chosen specifically to make this incremental expansion low-friction.
- The DB abstraction layer should be stable early. Adding MySQL/MariaDB support later should require only a new driver implementation, not changes to store or commands.
- The official `go-redis` client is the primary test oracle. Any behavior that `go-redis` observes differently from a real Redis server is a bug.
- RESP2 pipelining (multiple commands in a single TCP write) must be handled correctly from day one — many clients pipeline aggressively and a server that only reads one command per syscall will deadlock.
- The `INFO` command response in v1 may return stub/zero values for sections that are not yet implemented (e.g. replication, memory stats), but must return a syntactically valid INFO response that clients can parse without error.
