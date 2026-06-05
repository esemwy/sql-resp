package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) hAssert(ctx context.Context, key string, dbIdx int) error {
	return s.assertType(ctx, key, dbIdx, "hash")
}

func (s *Store) hEnsure(ctx context.Context, key string, dbIdx int) error {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return err
	}
	if err := s.hAssert(ctx, key, dbIdx); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO keys(key,db,type) VALUES(?,?,'hash') ON CONFLICT(key,db) DO NOTHING`,
		key, dbIdx)
	return err
}

// HSet sets field→value pairs on key. Returns the number of new fields added.
func (s *Store) HSet(ctx context.Context, key string, dbIdx int, pairs map[string]string) (int64, error) {
	if err := s.hEnsure(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var added int64
	for field, val := range pairs {
		var exists int
		tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM hashes WHERE key=? AND db=? AND field=?`,
			key, dbIdx, field).Scan(&exists)
		if exists == 0 {
			added++
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO hashes(key,db,field,value) VALUES(?,?,?,?)
			 ON CONFLICT(key,db,field) DO UPDATE SET value=excluded.value`,
			key, dbIdx, field, val); err != nil {
			return 0, err
		}
	}
	return added, tx.Commit()
}

// HGet returns the value of a field, or ("", false, nil) if missing.
func (s *Store) HGet(ctx context.Context, key string, dbIdx int, field string) (string, bool, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return "", false, err
	}
	if err := s.hAssert(ctx, key, dbIdx); err != nil {
		return "", false, err
	}
	var val string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM hashes WHERE key=? AND db=? AND field=?`,
		key, dbIdx, field).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return val, true, err
}

// HDel removes fields. Returns count of actually removed fields.
func (s *Store) HDel(ctx context.Context, key string, dbIdx int, fields []string) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.hAssert(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	var removed int64
	for _, f := range fields {
		res, err := s.db.ExecContext(ctx,
			`DELETE FROM hashes WHERE key=? AND db=? AND field=?`, key, dbIdx, f)
		if err != nil {
			return removed, err
		}
		n, _ := res.RowsAffected()
		removed += n
	}
	return removed, nil
}

// HExists returns true if field exists in key.
func (s *Store) HExists(ctx context.Context, key string, dbIdx int, field string) (bool, error) {
	_, found, err := s.HGet(ctx, key, dbIdx, field)
	return found, err
}

// HGetAll returns all field-value pairs for key.
func (s *Store) HGetAll(ctx context.Context, key string, dbIdx int) (map[string]string, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	if err := s.hAssert(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT field, value FROM hashes WHERE key=? AND db=?`, key, dbIdx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var f, v string
		if err := rows.Scan(&f, &v); err != nil {
			return nil, err
		}
		result[f] = v
	}
	return result, rows.Err()
}

// HKeys returns all field names.
func (s *Store) HKeys(ctx context.Context, key string, dbIdx int) ([]string, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	if err := s.hAssert(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT field FROM hashes WHERE key=? AND db=?`, key, dbIdx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var fields []string
	for rows.Next() {
		var f string
		rows.Scan(&f)
		fields = append(fields, f)
	}
	return fields, rows.Err()
}

// HVals returns all values.
func (s *Store) HVals(ctx context.Context, key string, dbIdx int) ([]string, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	if err := s.hAssert(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT value FROM hashes WHERE key=? AND db=?`, key, dbIdx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vals []string
	for rows.Next() {
		var v string
		rows.Scan(&v)
		vals = append(vals, v)
	}
	return vals, rows.Err()
}

// HLen returns the number of fields.
func (s *Store) HLen(ctx context.Context, key string, dbIdx int) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.hAssert(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM hashes WHERE key=? AND db=?`, key, dbIdx).Scan(&n)
	return n, err
}

// HMGet returns values for multiple fields (empty string + false for missing).
func (s *Store) HMGet(ctx context.Context, key string, dbIdx int, fields []string) ([]interface{}, error) {
	result := make([]interface{}, len(fields))
	for i, f := range fields {
		val, found, err := s.HGet(ctx, key, dbIdx, f)
		if err != nil {
			return nil, err
		}
		if found {
			result[i] = val
		}
	}
	return result, nil
}
