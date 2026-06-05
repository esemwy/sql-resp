package store

import (
	"context"
)

func (s *Store) sEnsure(ctx context.Context, key string, dbIdx int) error {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return err
	}
	if err := s.assertType(ctx, key, dbIdx, "set"); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO keys(key,db,type) VALUES(?,?,'set') ON CONFLICT(key,db) DO NOTHING`,
		key, dbIdx)
	return err
}

func (s *Store) sAssert(ctx context.Context, key string, dbIdx int) error {
	return s.assertType(ctx, key, dbIdx, "set")
}

// SAdd adds members. Returns count of new members.
func (s *Store) SAdd(ctx context.Context, key string, dbIdx int, members []string) (int64, error) {
	if err := s.sEnsure(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	var added int64
	for _, m := range members {
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO sets(key,db,member) VALUES(?,?,?) ON CONFLICT(key,db,member) DO NOTHING`,
			key, dbIdx, m)
		if err != nil {
			return added, err
		}
		n, _ := res.RowsAffected()
		added += n
	}
	return added, nil
}

// SRem removes members. Returns count removed.
func (s *Store) SRem(ctx context.Context, key string, dbIdx int, members []string) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.sAssert(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	var removed int64
	for _, m := range members {
		res, err := s.db.ExecContext(ctx,
			`DELETE FROM sets WHERE key=? AND db=? AND member=?`, key, dbIdx, m)
		if err != nil {
			return removed, err
		}
		n, _ := res.RowsAffected()
		removed += n
	}
	return removed, nil
}

// SMembers returns all members.
func (s *Store) SMembers(ctx context.Context, key string, dbIdx int) ([]string, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	if err := s.sAssert(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT member FROM sets WHERE key=? AND db=?`, key, dbIdx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var members []string
	for rows.Next() {
		var m string
		rows.Scan(&m)
		members = append(members, m)
	}
	return members, rows.Err()
}

// SIsMember returns true if member exists in the set.
func (s *Store) SIsMember(ctx context.Context, key string, dbIdx int, member string) (bool, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return false, err
	}
	if err := s.sAssert(ctx, key, dbIdx); err != nil {
		return false, err
	}
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sets WHERE key=? AND db=? AND member=?`,
		key, dbIdx, member).Scan(&n)
	return n > 0, err
}

// SCard returns the set cardinality.
func (s *Store) SCard(ctx context.Context, key string, dbIdx int) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.sAssert(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sets WHERE key=? AND db=?`, key, dbIdx).Scan(&n)
	return n, err
}

func (s *Store) sMembers(ctx context.Context, key string, dbIdx int) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT member FROM sets WHERE key=? AND db=?`, key, dbIdx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]struct{}{}
	for rows.Next() {
		var m string
		rows.Scan(&m)
		result[m] = struct{}{}
	}
	return result, rows.Err()
}

// SUnion returns the union of all given keys.
func (s *Store) SUnion(ctx context.Context, keys []string, dbIdx int) ([]string, error) {
	union := map[string]struct{}{}
	for _, k := range keys {
		if err := s.deleteExpired(ctx, k, dbIdx); err != nil {
			return nil, err
		}
		if err := s.sAssert(ctx, k, dbIdx); err != nil {
			return nil, err
		}
		m, err := s.sMembers(ctx, k, dbIdx)
		if err != nil {
			return nil, err
		}
		for v := range m {
			union[v] = struct{}{}
		}
	}
	result := make([]string, 0, len(union))
	for v := range union {
		result = append(result, v)
	}
	return result, nil
}

// SInter returns the intersection of all given keys.
func (s *Store) SInter(ctx context.Context, keys []string, dbIdx int) ([]string, error) {
	if len(keys) == 0 {
		return []string{}, nil
	}
	base, err := s.sMembers(ctx, keys[0], dbIdx)
	if err != nil {
		return nil, err
	}
	for _, k := range keys[1:] {
		if err := s.deleteExpired(ctx, k, dbIdx); err != nil {
			return nil, err
		}
		other, err := s.sMembers(ctx, k, dbIdx)
		if err != nil {
			return nil, err
		}
		for m := range base {
			if _, ok := other[m]; !ok {
				delete(base, m)
			}
		}
	}
	result := make([]string, 0, len(base))
	for v := range base {
		result = append(result, v)
	}
	return result, nil
}

// SDiff returns members in keys[0] not present in any of keys[1:].
func (s *Store) SDiff(ctx context.Context, keys []string, dbIdx int) ([]string, error) {
	if len(keys) == 0 {
		return []string{}, nil
	}
	base, err := s.sMembers(ctx, keys[0], dbIdx)
	if err != nil {
		return nil, err
	}
	for _, k := range keys[1:] {
		if err := s.deleteExpired(ctx, k, dbIdx); err != nil {
			return nil, err
		}
		other, err := s.sMembers(ctx, k, dbIdx)
		if err != nil {
			return nil, err
		}
		for m := range other {
			delete(base, m)
		}
	}
	result := make([]string, 0, len(base))
	for v := range base {
		result = append(result, v)
	}
	return result, nil
}
