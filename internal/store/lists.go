package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) lEnsure(ctx context.Context, key string, dbIdx int) error {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return err
	}
	if err := s.assertType(ctx, key, dbIdx, "list"); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO keys(key,db,type) VALUES(?,?,'list') ON CONFLICT(key,db) DO NOTHING`,
		key, dbIdx)
	return err
}

func (s *Store) lAssert(ctx context.Context, key string, dbIdx int) error {
	return s.assertType(ctx, key, dbIdx, "list")
}

// lAll returns all values in idx order.
func (s *Store) lAll(ctx context.Context, key string, dbIdx int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT value FROM lists WHERE key=? AND db=? ORDER BY idx ASC`, key, dbIdx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []string
	for rows.Next() {
		var v string
		rows.Scan(&v)
		all = append(all, v)
	}
	return all, rows.Err()
}

// lRewrite deletes all rows for key and inserts values with sequential idx 0..N-1.
func (s *Store) lRewrite(ctx context.Context, key string, dbIdx int, values []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM lists WHERE key=? AND db=?`, key, dbIdx); err != nil {
		return err
	}
	for i, v := range values {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO lists(key,db,idx,value) VALUES(?,?,?,?)`,
			key, dbIdx, i, v); err != nil {
			return err
		}
	}
	if len(values) == 0 {
		tx.ExecContext(ctx, `DELETE FROM keys WHERE key=? AND db=?`, key, dbIdx)
	}
	return tx.Commit()
}

// LPush prepends one or more values. Each value is pushed to the head.
func (s *Store) LPush(ctx context.Context, key string, dbIdx int, values []string) (int64, error) {
	if err := s.lEnsure(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	existing, err := s.lAll(ctx, key, dbIdx)
	if err != nil {
		return 0, err
	}
	// values are prepended in order: LPUSH k a b c → [c,b,a,...existing]
	prepend := make([]string, len(values))
	for i, v := range values {
		prepend[len(values)-1-i] = v
	}
	newList := append(prepend, existing...)
	if err := s.lRewrite(ctx, key, dbIdx, newList); err != nil {
		return 0, err
	}
	return int64(len(newList)), nil
}

// RPush appends one or more values.
func (s *Store) RPush(ctx context.Context, key string, dbIdx int, values []string) (int64, error) {
	if err := s.lEnsure(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	existing, err := s.lAll(ctx, key, dbIdx)
	if err != nil {
		return 0, err
	}
	newList := append(existing, values...)
	if err := s.lRewrite(ctx, key, dbIdx, newList); err != nil {
		return 0, err
	}
	return int64(len(newList)), nil
}

// LPop removes and returns the leftmost element.
func (s *Store) LPop(ctx context.Context, key string, dbIdx int) (string, bool, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return "", false, err
	}
	if err := s.lAssert(ctx, key, dbIdx); err != nil {
		return "", false, err
	}
	items, err := s.lAll(ctx, key, dbIdx)
	if err != nil || len(items) == 0 {
		return "", false, err
	}
	val := items[0]
	if err := s.lRewrite(ctx, key, dbIdx, items[1:]); err != nil {
		return "", false, err
	}
	return val, true, nil
}

// RPop removes and returns the rightmost element.
func (s *Store) RPop(ctx context.Context, key string, dbIdx int) (string, bool, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return "", false, err
	}
	if err := s.lAssert(ctx, key, dbIdx); err != nil {
		return "", false, err
	}
	items, err := s.lAll(ctx, key, dbIdx)
	if err != nil || len(items) == 0 {
		return "", false, err
	}
	val := items[len(items)-1]
	if err := s.lRewrite(ctx, key, dbIdx, items[:len(items)-1]); err != nil {
		return "", false, err
	}
	return val, true, nil
}

// LLen returns the list length.
func (s *Store) LLen(ctx context.Context, key string, dbIdx int) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.lAssert(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM lists WHERE key=? AND db=?`, key, dbIdx).Scan(&n)
	return n, err
}

// LRange returns elements in [start, stop] (inclusive), negative indices supported.
func (s *Store) LRange(ctx context.Context, key string, dbIdx int, start, stop int64) ([]string, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	if err := s.lAssert(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	all, err := s.lAll(ctx, key, dbIdx)
	if err != nil {
		return nil, err
	}
	n := int64(len(all))
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n {
		return []string{}, nil
	}
	return all[start : stop+1], nil
}

// LIndex returns the element at index (negative indices supported).
func (s *Store) LIndex(ctx context.Context, key string, dbIdx int, index int64) (string, bool, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return "", false, err
	}
	if err := s.lAssert(ctx, key, dbIdx); err != nil {
		return "", false, err
	}
	all, err := s.lAll(ctx, key, dbIdx)
	if err != nil {
		return "", false, err
	}
	n := int64(len(all))
	if index < 0 {
		index = n + index
	}
	if index < 0 || index >= n {
		return "", false, nil
	}
	return all[index], true, nil
}

// LSet sets the element at index.
func (s *Store) LSet(ctx context.Context, key string, dbIdx int, index int64, value string) error {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return err
	}
	if err := s.lAssert(ctx, key, dbIdx); err != nil {
		return err
	}
	all, err := s.lAll(ctx, key, dbIdx)
	if err != nil {
		return err
	}
	n := int64(len(all))
	if index < 0 {
		index = n + index
	}
	if index < 0 || index >= n {
		return errors.New("ERR index out of range")
	}
	all[index] = value
	return s.lRewrite(ctx, key, dbIdx, all)
}

// LInsert inserts value before or after pivot. Returns new length or -1 if pivot not found.
func (s *Store) LInsert(ctx context.Context, key string, dbIdx int, before bool, pivot, value string) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.lAssert(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	all, err := s.lAll(ctx, key, dbIdx)
	if err != nil {
		return 0, err
	}

	pivotPos := -1
	for i, v := range all {
		if v == pivot {
			pivotPos = i
			break
		}
	}
	if pivotPos == -1 {
		return -1, nil
	}

	insertAt := pivotPos
	if !before {
		insertAt = pivotPos + 1
	}

	newList := make([]string, 0, len(all)+1)
	newList = append(newList, all[:insertAt]...)
	newList = append(newList, value)
	newList = append(newList, all[insertAt:]...)

	if err := s.lRewrite(ctx, key, dbIdx, newList); err != nil {
		return 0, err
	}
	return int64(len(newList)), nil
}

// LRem removes occurrences of value. count>0: from head; count<0: from tail; 0: all.
func (s *Store) LRem(ctx context.Context, key string, dbIdx int, count int64, value string) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.lAssert(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	all, err := s.lAll(ctx, key, dbIdx)
	if err != nil {
		return 0, err
	}

	fromTail := count < 0
	limit := count
	if fromTail {
		limit = -count
		// Reverse for tail-to-head scanning.
		for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
			all[i], all[j] = all[j], all[i]
		}
	}

	var removed int64
	result := make([]string, 0, len(all))
	for _, v := range all {
		if v == value && (limit == 0 || removed < limit) {
			removed++
		} else {
			result = append(result, v)
		}
	}

	if fromTail {
		for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
			result[i], result[j] = result[j], result[i]
		}
	}

	if err := s.lRewrite(ctx, key, dbIdx, result); err != nil {
		return 0, err
	}
	return removed, nil
}

// lDummySQL forces the sql import to be used.
var _ = sql.ErrNoRows
