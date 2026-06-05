// Package store implements Redis data structure operations on top of the db.DB interface.
// All TTL enforcement, lazy expiry, and multi-database isolation live here.
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gitlab.smy.com/work/sql-resp/internal/db"
)

// ErrWrongType is returned when a command targets a key of the wrong type.
var ErrWrongType = errors.New("WRONGTYPE Operation against a key holding the wrong kind of value")

// ErrNoKey is returned when a key does not exist (for operations that require it).
var ErrNoKey = errors.New("ERR no such key")

// Store wraps a DB and provides high-level Redis-like operations.
type Store struct {
	db db.DB
}

// New creates a Store backed by the given DB.
func New(d db.DB) *Store {
	return &Store{db: d}
}

// nowMs returns the current time as Unix milliseconds.
func nowMs() int64 {
	return time.Now().UnixMilli()
}

// expiredWhere returns the SQL fragment for checking expiry.
// A key is alive if expires_at IS NULL or expires_at > now.
const aliveWhere = `(expires_at IS NULL OR expires_at > ?)`

// keyType returns the type of a key, checking expiry. Returns "" if missing/expired.
func (s *Store) keyType(ctx context.Context, key string, dbIdx int) (string, error) {
	var typ string
	err := s.db.QueryRowContext(ctx,
		`SELECT type FROM keys WHERE key=? AND db=? AND `+aliveWhere,
		key, dbIdx, nowMs()).Scan(&typ)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return typ, err
}

// deleteExpired removes a key and all its data if it is expired.
// Called lazily before reads.
func (s *Store) deleteExpired(ctx context.Context, key string, dbIdx int) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM keys WHERE key=? AND db=? AND expires_at IS NOT NULL AND expires_at <= ?`,
		key, dbIdx, nowMs())
	return err
}

// assertType returns an error if the key exists with a different type.
// Returns nil if the key does not exist.
func (s *Store) assertType(ctx context.Context, key string, dbIdx int, want string) error {
	typ, err := s.keyType(ctx, key, dbIdx)
	if err != nil {
		return err
	}
	if typ != "" && typ != want {
		return ErrWrongType
	}
	return nil
}

// upsertKey creates or updates the keys row. Does not change expires_at if keepTTL is true.
func (s *Store) upsertKey(ctx context.Context, tx *sql.Tx, key string, dbIdx int, typ string, expiresAt *int64) error {
	if expiresAt == nil {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO keys(key, db, type, expires_at) VALUES(?,?,?,NULL)
			 ON CONFLICT(key,db) DO UPDATE SET type=excluded.type, expires_at=NULL`,
			key, dbIdx, typ)
		return err
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO keys(key, db, type, expires_at) VALUES(?,?,?,?)
		 ON CONFLICT(key,db) DO UPDATE SET type=excluded.type, expires_at=excluded.expires_at`,
		key, dbIdx, typ, *expiresAt)
	return err
}

// ── String commands ──────────────────────────────────────────────────────────

// Get returns the string value for key, or ("", false, nil) if missing/expired.
func (s *Store) Get(ctx context.Context, key string, dbIdx int) (string, bool, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return "", false, err
	}
	if err := s.assertType(ctx, key, dbIdx, "string"); err != nil {
		return "", false, err
	}
	var val string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM strings WHERE key=? AND db=?`, key, dbIdx).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return val, true, err
}

// SetOptions controls the behaviour of Set.
type SetOptions struct {
	ExpiresAt *int64 // Unix milliseconds; nil = no expiry
	NX        bool   // only set if key does not exist
	XX        bool   // only set if key already exists
	KeepTTL   bool   // keep existing TTL (not yet used)
}

// Set stores a string value. Returns (true, nil) on success, (false, nil) if
// NX/XX precondition not met.
func (s *Store) Set(ctx context.Context, key string, dbIdx int, value string, opts SetOptions) (bool, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return false, err
	}
	if err := s.assertType(ctx, key, dbIdx, "string"); err != nil {
		return false, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// Evaluate NX / XX by checking key existence inside the transaction.
	var exists int
	tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM keys WHERE key=? AND db=? AND `+aliveWhere,
		key, dbIdx, nowMs()).Scan(&exists)

	if opts.NX && exists > 0 {
		return false, nil
	}
	if opts.XX && exists == 0 {
		return false, nil
	}

	if err := s.upsertKey(ctx, tx, key, dbIdx, "string", opts.ExpiresAt); err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO strings(key, db, value) VALUES(?,?,?)
		 ON CONFLICT(key,db) DO UPDATE SET value=excluded.value`,
		key, dbIdx, value)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// Del deletes one or more keys. Returns the count of actually deleted keys.
func (s *Store) Del(ctx context.Context, keys []string, dbIdx int) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	var total int64
	for _, k := range keys {
		res, err := s.db.ExecContext(ctx,
			`DELETE FROM keys WHERE key=? AND db=?`, k, dbIdx)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

// Exists returns the number of the given keys that exist (and are not expired).
func (s *Store) Exists(ctx context.Context, keys []string, dbIdx int) (int64, error) {
	var count int64
	for _, k := range keys {
		if err := s.deleteExpired(ctx, k, dbIdx); err != nil {
			return count, err
		}
		var n int64
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM keys WHERE key=? AND db=?`, k, dbIdx).Scan(&n)
		count += n
	}
	return count, nil
}

// Expire sets the expiry for key as a duration from now (seconds).
// Returns (true, nil) if set, (false, nil) if key not found.
func (s *Store) Expire(ctx context.Context, key string, dbIdx int, seconds int64) (bool, error) {
	return s.ExpireAt(ctx, key, dbIdx, time.Now().Add(time.Duration(seconds)*time.Second).UnixMilli())
}

// PExpire sets the expiry in milliseconds from now.
func (s *Store) PExpire(ctx context.Context, key string, dbIdx int, ms int64) (bool, error) {
	return s.ExpireAt(ctx, key, dbIdx, nowMs()+ms)
}

// ExpireAt sets the expiry to an absolute Unix millisecond timestamp.
func (s *Store) ExpireAt(ctx context.Context, key string, dbIdx int, atMs int64) (bool, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return false, err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE keys SET expires_at=? WHERE key=? AND db=?`,
		atMs, key, dbIdx)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// TTL returns the remaining time-to-live in seconds.
// Returns -1 if the key has no expiry, -2 if the key does not exist.
func (s *Store) TTL(ctx context.Context, key string, dbIdx int) (int64, error) {
	return s.ttl(ctx, key, dbIdx, false)
}

// PTTL returns the remaining time-to-live in milliseconds.
func (s *Store) PTTL(ctx context.Context, key string, dbIdx int) (int64, error) {
	return s.ttl(ctx, key, dbIdx, true)
}

func (s *Store) ttl(ctx context.Context, key string, dbIdx int, millis bool) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return -2, err
	}
	var expiresAt sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT expires_at FROM keys WHERE key=? AND db=?`, key, dbIdx).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return -2, nil
	}
	if err != nil {
		return -2, err
	}
	if !expiresAt.Valid {
		return -1, nil
	}
	rem := expiresAt.Int64 - nowMs()
	if rem < 0 {
		return -2, nil
	}
	if millis {
		return rem, nil
	}
	return rem / 1000, nil
}

// Persist removes the TTL from a key.
// Returns (true, nil) if removed, (false, nil) if key has no TTL or doesn't exist.
func (s *Store) Persist(ctx context.Context, key string, dbIdx int) (bool, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return false, err
	}
	var expiresAt sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT expires_at FROM keys WHERE key=? AND db=?`, key, dbIdx).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) || !expiresAt.Valid {
		return false, nil
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE keys SET expires_at=NULL WHERE key=? AND db=?`, key, dbIdx)
	return err == nil, err
}

// Type returns the type string for a key ("string","list","hash","set","zset","none").
func (s *Store) Type(ctx context.Context, key string, dbIdx int) (string, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return "none", err
	}
	typ, err := s.keyType(ctx, key, dbIdx)
	if typ == "" {
		return "none", err
	}
	return typ, err
}

// SweepExpired deletes all expired keys. Called by the background sweep goroutine.
func (s *Store) SweepExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM keys WHERE expires_at IS NOT NULL AND expires_at <= ?`, nowMs())
	return err
}

// DBSize returns the number of alive keys in dbIdx.
func (s *Store) DBSize(ctx context.Context, dbIdx int) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM keys WHERE db=? AND `+aliveWhere,
		dbIdx, nowMs()).Scan(&n)
	return n, err
}

// FlushDB deletes all keys in dbIdx.
func (s *Store) FlushDB(ctx context.Context, dbIdx int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM keys WHERE db=?`, dbIdx)
	return err
}

// FlushAll deletes all keys in all databases.
func (s *Store) FlushAll(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM keys`)
	return err
}

// Rename atomically renames src to dst in dbIdx.
func (s *Store) Rename(ctx context.Context, src, dst string, dbIdx int) error {
	if err := s.deleteExpired(ctx, src, dbIdx); err != nil {
		return err
	}
	var typ string
	var expiresAt sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT type, expires_at FROM keys WHERE key=? AND db=?`, src, dbIdx).
		Scan(&typ, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoKey
	}
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete destination if it exists (cascade removes its data).
	tx.ExecContext(ctx, `DELETE FROM keys WHERE key=? AND db=?`, dst, dbIdx)

	// Insert dst into keys.
	var expVal any
	if expiresAt.Valid {
		expVal = expiresAt.Int64
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO keys(key, db, type, expires_at) VALUES(?,?,?,?)`,
		dst, dbIdx, typ, expVal); err != nil {
		return err
	}

	// Copy data rows from src to dst in the type-specific table.
	switch typ {
	case "string":
		_, err = tx.ExecContext(ctx,
			`INSERT INTO strings(key, db, value) SELECT ?, db, value FROM strings WHERE key=? AND db=?`,
			dst, src, dbIdx)
	case "list":
		_, err = tx.ExecContext(ctx,
			`INSERT INTO lists(key, db, idx, value) SELECT ?, db, idx, value FROM lists WHERE key=? AND db=?`,
			dst, src, dbIdx)
	case "hash":
		_, err = tx.ExecContext(ctx,
			`INSERT INTO hashes(key, db, field, value) SELECT ?, db, field, value FROM hashes WHERE key=? AND db=?`,
			dst, src, dbIdx)
	case "set":
		_, err = tx.ExecContext(ctx,
			`INSERT INTO sets(key, db, member) SELECT ?, db, member FROM sets WHERE key=? AND db=?`,
			dst, src, dbIdx)
	case "zset":
		_, err = tx.ExecContext(ctx,
			`INSERT INTO zsets(key, db, member, score) SELECT ?, db, member, score FROM zsets WHERE key=? AND db=?`,
			dst, src, dbIdx)
	}
	if err != nil {
		return err
	}

	// Delete src (cascade removes its data rows).
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM keys WHERE key=? AND db=?`, src, dbIdx); err != nil {
		return err
	}
	return tx.Commit()
}

// RandomKey returns a random alive key name from dbIdx, or "" if empty.
func (s *Store) RandomKey(ctx context.Context, dbIdx int) (string, error) {
	var key string
	err := s.db.QueryRowContext(ctx,
		`SELECT key FROM keys WHERE db=? AND `+aliveWhere+` ORDER BY RANDOM() LIMIT 1`,
		dbIdx, nowMs()).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return key, err
}

// Keys returns all alive key names in dbIdx matching the glob pattern.
func (s *Store) Keys(ctx context.Context, pattern string, dbIdx int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key FROM keys WHERE db=? AND `+aliveWhere,
		dbIdx, nowMs())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		if matchGlob(pattern, k) {
			result = append(result, k)
		}
	}
	return result, rows.Err()
}

// matchGlob matches key against a Redis glob pattern (* and ? wildcards).
func matchGlob(pattern, key string) bool {
	return globMatch(pattern, key)
}

func globMatch(pat, str string) bool {
	for len(pat) > 0 {
		switch pat[0] {
		case '*':
			pat = pat[1:]
			if len(pat) == 0 {
				return true
			}
			for i := 0; i <= len(str); i++ {
				if globMatch(pat, str[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(str) == 0 {
				return false
			}
			pat = pat[1:]
			str = str[1:]
		case '[':
			// Character class — simplified: skip to ']' and treat as literal match.
			end := 1
			for end < len(pat) && pat[end] != ']' {
				end++
			}
			if len(str) == 0 {
				return false
			}
			// Match any character in the class.
			class := pat[1:end]
			ch := str[0]
			matched := false
			for j := 0; j < len(class); j++ {
				if class[j] == ch {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
			pat = pat[end+1:]
			str = str[1:]
		default:
			if len(str) == 0 || pat[0] != str[0] {
				return false
			}
			pat = pat[1:]
			str = str[1:]
		}
	}
	return len(str) == 0
}
