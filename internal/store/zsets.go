package store

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strconv"
	"strings"
)

func (s *Store) zEnsure(ctx context.Context, key string, dbIdx int) error {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return err
	}
	if err := s.assertType(ctx, key, dbIdx, "zset"); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO keys(key,db,type) VALUES(?,?,'zset') ON CONFLICT(key,db) DO NOTHING`,
		key, dbIdx)
	return err
}

func (s *Store) zAssert(ctx context.Context, key string, dbIdx int) error {
	return s.assertType(ctx, key, dbIdx, "zset")
}

// ZAddOptions controls ZADD behaviour.
type ZAddOptions struct {
	NX bool // only add new members
	XX bool // only update existing members
}

// ZAdd adds or updates members. Returns count of new members added.
func (s *Store) ZAdd(ctx context.Context, key string, dbIdx int, members map[string]float64, opts ZAddOptions) (int64, error) {
	if err := s.zEnsure(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	var added int64
	for member, score := range members {
		var exists int
		s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM zsets WHERE key=? AND db=? AND member=?`,
			key, dbIdx, member).Scan(&exists)

		if opts.NX && exists > 0 {
			continue
		}
		if opts.XX && exists == 0 {
			continue
		}

		if exists == 0 {
			_, err := s.db.ExecContext(ctx,
				`INSERT INTO zsets(key,db,member,score) VALUES(?,?,?,?)`,
				key, dbIdx, member, score)
			if err != nil {
				return added, err
			}
			added++
		} else {
			_, err := s.db.ExecContext(ctx,
				`UPDATE zsets SET score=? WHERE key=? AND db=? AND member=?`,
				score, key, dbIdx, member)
			if err != nil {
				return added, err
			}
		}
	}
	return added, nil
}

// ZRem removes members. Returns count removed.
func (s *Store) ZRem(ctx context.Context, key string, dbIdx int, members []string) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.zAssert(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	var removed int64
	for _, m := range members {
		res, err := s.db.ExecContext(ctx,
			`DELETE FROM zsets WHERE key=? AND db=? AND member=?`, key, dbIdx, m)
		if err != nil {
			return removed, err
		}
		n, _ := res.RowsAffected()
		removed += n
	}
	return removed, nil
}

// ZScore returns the score of member, or (0, false, nil) if not found.
func (s *Store) ZScore(ctx context.Context, key string, dbIdx int, member string) (float64, bool, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, false, err
	}
	if err := s.zAssert(ctx, key, dbIdx); err != nil {
		return 0, false, err
	}
	var score float64
	err := s.db.QueryRowContext(ctx,
		`SELECT score FROM zsets WHERE key=? AND db=? AND member=?`,
		key, dbIdx, member).Scan(&score)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return score, true, err
}

// ZRank returns the 0-based rank by ascending score, or (-1, false, nil) if not found.
func (s *Store) ZRank(ctx context.Context, key string, dbIdx int, member string) (int64, bool, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, false, err
	}
	if err := s.zAssert(ctx, key, dbIdx); err != nil {
		return 0, false, err
	}
	var rank int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM zsets WHERE key=? AND db=? AND score < (
			SELECT score FROM zsets WHERE key=? AND db=? AND member=?
		)`,
		key, dbIdx, key, dbIdx, member).Scan(&rank)
	if err != nil {
		// member not found
		return 0, false, nil
	}
	// Verify member exists.
	var exists int
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM zsets WHERE key=? AND db=? AND member=?`,
		key, dbIdx, member).Scan(&exists)
	if exists == 0 {
		return 0, false, nil
	}
	return rank, true, nil
}

// ZRange returns members in ascending score order for [start, stop] (inclusive).
func (s *Store) ZRange(ctx context.Context, key string, dbIdx int, start, stop int64) ([]string, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	if err := s.zAssert(ctx, key, dbIdx); err != nil {
		return nil, err
	}

	var count int64
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM zsets WHERE key=? AND db=?`, key, dbIdx).Scan(&count)

	if start < 0 {
		start = count + start
	}
	if stop < 0 {
		stop = count + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= count {
		stop = count - 1
	}
	if start > stop {
		return []string{}, nil
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT member FROM zsets WHERE key=? AND db=? ORDER BY score ASC, member ASC LIMIT ? OFFSET ?`,
		key, dbIdx, stop-start+1, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var m string
		rows.Scan(&m)
		result = append(result, m)
	}
	return result, rows.Err()
}

// ZRangeByScore returns members with scores in [min, max].
// min/max may use Redis syntax: "-inf", "+inf", "(n" (exclusive).
func (s *Store) ZRangeByScore(ctx context.Context, key string, dbIdx int, minStr, maxStr string) ([]string, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return nil, err
	}
	if err := s.zAssert(ctx, key, dbIdx); err != nil {
		return nil, err
	}

	minVal, minExcl, err := parseScore(minStr)
	if err != nil {
		return nil, err
	}
	maxVal, maxExcl, err := parseScore(maxStr)
	if err != nil {
		return nil, err
	}

	minOp := ">="
	if minExcl {
		minOp = ">"
	}
	maxOp := "<="
	if maxExcl {
		maxOp = "<"
	}

	query := `SELECT member FROM zsets WHERE key=? AND db=? AND score ` + minOp + ` ? AND score ` + maxOp + ` ? ORDER BY score ASC, member ASC`
	rows, err := s.db.QueryContext(ctx, query, key, dbIdx, minVal, maxVal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var m string
		rows.Scan(&m)
		result = append(result, m)
	}
	return result, rows.Err()
}

// ZCard returns the number of members.
func (s *Store) ZCard(ctx context.Context, key string, dbIdx int) (int64, error) {
	if err := s.deleteExpired(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	if err := s.zAssert(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	var n int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM zsets WHERE key=? AND db=?`, key, dbIdx).Scan(&n)
	return n, err
}

// ZCount returns the number of members with scores in [min, max].
func (s *Store) ZCount(ctx context.Context, key string, dbIdx int, minStr, maxStr string) (int64, error) {
	members, err := s.ZRangeByScore(ctx, key, dbIdx, minStr, maxStr)
	if err != nil {
		return 0, err
	}
	return int64(len(members)), nil
}

// ZIncrBy increments the score of member by delta. Returns new score.
func (s *Store) ZIncrBy(ctx context.Context, key string, dbIdx int, delta float64, member string) (float64, error) {
	if err := s.zEnsure(ctx, key, dbIdx); err != nil {
		return 0, err
	}
	var cur float64
	err := s.db.QueryRowContext(ctx,
		`SELECT score FROM zsets WHERE key=? AND db=? AND member=?`,
		key, dbIdx, member).Scan(&cur)
	if errors.Is(err, sql.ErrNoRows) {
		cur = 0
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO zsets(key,db,member,score) VALUES(?,?,?,?)`,
			key, dbIdx, member, delta)
		return delta, err
	}
	if err != nil {
		return 0, err
	}
	newScore := cur + delta
	_, err = s.db.ExecContext(ctx,
		`UPDATE zsets SET score=? WHERE key=? AND db=? AND member=?`,
		newScore, key, dbIdx, member)
	return newScore, err
}

// parseScore parses a Redis score string: "-inf", "+inf", "(n", or "n".
func parseScore(s string) (float64, bool, error) {
	excl := false
	if strings.HasPrefix(s, "(") {
		excl = true
		s = s[1:]
	}
	switch strings.ToLower(s) {
	case "-inf":
		return math.Inf(-1), excl, nil
	case "+inf", "inf":
		return math.Inf(1), excl, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, excl, err
}
