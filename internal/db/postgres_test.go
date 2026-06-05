package db

import (
	"testing"
)

func TestRewritePlaceholders(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"SELECT 1", "SELECT 1"},
		{"SELECT * FROM t WHERE key=?", "SELECT * FROM t WHERE key=$1"},
		{"INSERT INTO t(a,b,c) VALUES(?,?,?)", "INSERT INTO t(a,b,c) VALUES($1,$2,$3)"},
		// Placeholder inside string literal must not be rewritten.
		{"SELECT '?' FROM t WHERE x=?", "SELECT '?' FROM t WHERE x=$1"},
		// Escaped single-quote inside string literal.
		{"SELECT 'it''s' WHERE x=?", "SELECT 'it''s' WHERE x=$1"},
		// Multiple placeholders.
		{"WHERE a=? AND b=? AND c=?", "WHERE a=$1 AND b=$2 AND c=$3"},
		// No placeholders.
		{"SELECT 1 FROM t WHERE type='string'", "SELECT 1 FROM t WHERE type='string'"},
		// ON CONFLICT clause (common in store layer).
		{
			"INSERT INTO keys(key,db,type) VALUES(?,?,'hash') ON CONFLICT(key,db) DO NOTHING",
			"INSERT INTO keys(key,db,type) VALUES($1,$2,'hash') ON CONFLICT(key,db) DO NOTHING",
		},
	}

	for _, tc := range cases {
		got := rewritePlaceholders(tc.in)
		if got != tc.want {
			t.Errorf("rewritePlaceholders(%q)\n  got  %q\n  want %q", tc.in, got, tc.want)
		}
	}
}

func TestSplitStatements(t *testing.T) {
	sql := `CREATE TABLE a (x INT);
CREATE TABLE b (y INT);

CREATE INDEX i ON a(x);`
	stmts := splitStatements(sql)
	if len(stmts) != 3 {
		t.Errorf("expected 3 statements, got %d: %v", len(stmts), stmts)
	}
}
