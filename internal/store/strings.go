package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
)

// IncrBy atomically increments the integer value of key by delta.
// If key does not exist it is initialized to 0 first.
func (s *Store) IncrBy(ctx context.Context, key string, dbIdx int, delta int64) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.assertType(ctx, key, dbIdx, "string"); err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var raw sql.NullString
	tx.QueryRowContext(ctx,
		`SELECT value FROM strings WHERE key=? AND db=?`, key, dbIdx).Scan(&raw)

	var cur int64
	if raw.Valid {
		cur, err = strconv.ParseInt(raw.String, 10, 64)
		if err != nil {
			return 0, errors.New("ERR value is not an integer or out of range")
		}
	}

	next := cur + delta

	if raw.Valid {
		_, err = tx.ExecContext(ctx,
			`UPDATE strings SET value=? WHERE key=? AND db=?`,
			strconv.FormatInt(next, 10), key, dbIdx)
	} else {
		// Key does not exist — create it.
		if _, err2 := tx.ExecContext(ctx,
			`INSERT INTO keys(key, db, type) VALUES(?,?,'string')
			 ON CONFLICT(key,db) DO UPDATE SET type='string'`,
			key, dbIdx); err2 != nil {
			return 0, err2
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO strings(key, db, value) VALUES(?,?,?)`,
			key, dbIdx, strconv.FormatInt(next, 10))
	}
	if err != nil {
		return 0, err
	}
	return next, tx.Commit()
}

// Append appends suffix to the string value of key and returns the new length.
func (s *Store) Append(ctx context.Context, key string, dbIdx int, suffix string) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.assertType(ctx, key, dbIdx, "string"); err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var cur string
	row := tx.QueryRowContext(ctx,
		`SELECT value FROM strings WHERE key=? AND db=?`, key, dbIdx)
	exists := true
	if err := row.Scan(&cur); errors.Is(err, sql.ErrNoRows) {
		exists = false
	} else if err != nil {
		return 0, err
	}

	newVal := cur + suffix
	if exists {
		_, err = tx.ExecContext(ctx,
			`UPDATE strings SET value=? WHERE key=? AND db=?`, newVal, key, dbIdx)
	} else {
		tx.ExecContext(ctx,
			`INSERT INTO keys(key, db, type) VALUES(?,?,'string')
			 ON CONFLICT(key,db) DO UPDATE SET type='string'`,
			key, dbIdx)
		_, err = tx.ExecContext(ctx,
			`INSERT INTO strings(key, db, value) VALUES(?,?,?)`, key, dbIdx, newVal)
	}
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int64(len(newVal)), nil
}

// StrLen returns the byte length of the string value for key (0 if missing).
func (s *Store) StrLen(ctx context.Context, key string, dbIdx int) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.assertType(ctx, key, dbIdx, "string"); err != nil {
		return 0, err
	}
	var val string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM strings WHERE key=? AND db=?`, key, dbIdx).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return int64(len(val)), nil
}
