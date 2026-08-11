package readonly

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		refused bool
	}{
		// Plain read-only statements.
		{"select", "SELECT * FROM users", false},
		{"select lowercase", "select 1", false},
		{"values", "VALUES (1), (2)", false},
		{"table", "TABLE users", false},
		{"show", "SHOW TABLES", false},
		{"describe", "DESCRIBE users", false},
		{"desc", "DESC users", false},
		{"trailing semicolon", "SELECT 1;", false},
		{"leading comments", "/* comment */ -- x\n SELECT 1", false},
		{"keyword inside string literal", "SELECT 'DELETE FROM t'", false},
		{"keyword inside identifier", "SELECT * FROM delete_log", false},

		// A read-only leading keyword is not enough: these SELECTs write.
		{"select into table", "SELECT * INTO backup FROM production_table", true},
		{"select into temp table", "SELECT * INTO TEMP backup FROM t", true},
		{"select into outfile", "SELECT * FROM t INTO OUTFILE '/tmp/x.csv'", true},
		{"select into dumpfile", "SELECT a FROM t INTO DUMPFILE '/tmp/x.bin'", true},
		{"select into variable", "SELECT a INTO @v FROM t", true},
		{"with body select into", "WITH a AS (SELECT 1) SELECT * INTO backup FROM a", true},
		{"explain select into", "EXPLAIN SELECT * INTO backup FROM t", true},
		{"into inside string literal", "SELECT 'INTO' FROM t", false},
		{"into inside identifier", "SELECT * FROM into_log", false},
		{"into as quoted identifier", `SELECT "into" FROM t`, false},
		{"into inside comment", "SELECT 1 -- INTO backup", false},

		// The guard reads one portable subset of SQL rather than each
		// dialect's rules, so every construct dialects disagree about is
		// refused — including the ones that are harmless on the server the
		// session happens to be connected to. # is a comment on MySQL and the
		// bitwise-XOR operator on PostgreSQL; reading it either way is a guess.
		{"hash operator hides a second statement", "SELECT 1 # 2; DELETE FROM t", true},
		{"hash comment on mysql", "SELECT 1 # comment", true},
		{"hash comment does not hide a separator", "SELECT 1 # comment\n; DELETE FROM t", true},

		// A backslash-escaped quote is read differently depending on
		// NO_BACKSLASH_ESCAPES / standard_conforming_strings / ANSI_QUOTES, so
		// the run's extent — and every keyword after it — cannot be trusted.
		// The portable spelling ('' or "") is always available.
		{"ambiguous escape on mysql", `SELECT 'it\'s' FROM t`, true},
		{"ambiguous escape hiding into", `SELECT 'it\'s' FROM t INTO OUTFILE '/tmp/x'`, true},
		{"ambiguous escape on postgres", `SELECT 'it\'s' FROM t`, true},
		// E'...' would be unambiguous on PostgreSQL, but the guard does not
		// ask which server it is talking to. Refusing it costs a rewrite of
		// the literal; recognizing it costs another dialect rule.
		{"escape string is read correctly", `SELECT E'it\'s' FROM t`, true},
		{"escape string hiding into", `SELECT E'it\'s' INTO backup FROM t`, true},
		// SQLite has no backslash escapes, so this is unambiguous there — and
		// still refused, for the same reason.
		{"backslash is literal on sqlite", `SELECT 'a\' FROM t`, true},
		{"doubled quote is never ambiguous", `SELECT 'it''s' FROM t`, false},

		// MySQL needs whitespace after -- for it to be a comment; PostgreSQL
		// and SQLite do not. `1--1` is therefore arithmetic on one and a
		// comment on the others, so it is refused on all of them.
		{"mysql double dash without space", "SELECT 1--1; DELETE FROM t", true},
		{"mysql double dash with space", "SELECT 1 -- comment\n", false},
		{"postgres double dash without space", "SELECT 1--1 FROM t", true},

		// MySQL runs the contents of /*! ... */ instead of ignoring them. The
		// form is refused everywhere rather than only on MySQL.
		{"mysql executable comment", "SELECT 1; /*! DELETE FROM t */", true},
		{"mysql versioned executable comment", "SELECT 1; /*!50110 DELETE FROM t */", true},
		{"mariadb executable comment", "SELECT 1; /*M! DELETE FROM t */", true},
		{"mysql plain block comment", "SELECT 1 /* plain comment */", false},
		{"executable comment inside a literal", "SELECT '/*! x */' FROM t", false},
		{"executable comment form on postgres", "SELECT 1; /*! DELETE FROM t */", true},

		// On MySQL a double-quoted run is a string unless ANSI_QUOTES is set,
		// so an escaped quote inside it has the same setting-dependent extent
		// as a single-quoted one. Refused everywhere.
		{"mysql escaped double quote", `SELECT "a\"b"; DELETE FROM t`, true},
		{"mysql plain double quote", `SELECT "plain" FROM t`, false},
		{"postgres quoted identifier with backslash", `SELECT "a\"b" FROM t`, true},

		// Writing statements.
		{"insert", "INSERT INTO t VALUES (1)", true},
		{"update", "UPDATE t SET a = 1", true},
		{"delete", "DELETE FROM t", true},
		{"drop", "DROP TABLE t", true},
		{"create", "CREATE TABLE t (a TEXT)", true},
		{"alter", "ALTER TABLE t ADD COLUMN b TEXT", true},
		{"truncate", "TRUNCATE TABLE t", true},
		{"replace", "REPLACE INTO t VALUES (1)", true},
		{"merge", "MERGE INTO t USING s ON t.a = s.a", true},
		{"grant", "GRANT SELECT ON t TO bob", true},
		{"vacuum", "VACUUM", true},
		{"comment hides nothing", "-- harmless\nDELETE FROM t", true},

		// ATTACH: SQLite's mode=ro still allows it, so the guard must refuse.
		{"attach", "ATTACH DATABASE 'side.db' AS side", true},

		// Unknown keywords fail closed.
		{"unknown keyword", "FROBNICATE t", true},
		{"empty", "", true},
		{"comment only", "-- nothing here", true},

		// Multiple statements: the leading keyword describes only the first.
		{"select then delete", "SELECT 1; DELETE FROM t", true},
		{"select then select", "SELECT 1; SELECT 2", true},
		{"semicolon in string", "SELECT ';'", false},
		{"semicolon in comment", "SELECT 1 -- ; DELETE FROM t", false},

		// WITH: the body keyword alone is not enough.
		{"with select", "WITH a AS (SELECT 1) SELECT * FROM a", false},
		{"with recursive", "WITH RECURSIVE a AS (SELECT 1) SELECT * FROM a", false},
		{"with column list", "WITH a (x) AS (SELECT 1) SELECT * FROM a", false},
		{"with materialized", "WITH a AS MATERIALIZED (SELECT 1) SELECT * FROM a", false},
		{"with not materialized", "WITH a AS NOT MATERIALIZED (SELECT 1) SELECT * FROM a", false},
		{"with dml body", "WITH a AS (SELECT 1) DELETE FROM t", true},
		{
			"data-modifying cte",
			"WITH gone AS (DELETE FROM t RETURNING *) SELECT * FROM gone",
			true,
		},
		{
			"data-modifying second cte",
			"WITH a AS (SELECT 1), b AS (UPDATE t SET x = 1 RETURNING *) SELECT * FROM a",
			true,
		},
		{
			"data-modifying nested cte",
			"WITH a AS (WITH b AS (DELETE FROM t RETURNING *) SELECT * FROM b) SELECT * FROM a",
			true,
		},
		{"with subquery in body", "WITH a AS (SELECT 1) SELECT * FROM (SELECT 2) s", false},
		{"with named column definition list", "WITH a AS (SELECT 1) SELECT * FROM json_to_record('{}') AS r(x int)", false},
		// Known conservative rejection: an unnamed column definition list in
		// the body looks exactly like a CTE term to the scanner. Refusing it
		// is the allow-list failing closed; naming the alias avoids it.
		{"with unnamed column definition list is refused", "WITH a AS (SELECT 1) SELECT * FROM f() AS (x int, y text)", true},

		// EXPLAIN: ANALYZE executes its target on PostgreSQL.
		{"explain select", "EXPLAIN SELECT 1", false},
		{"explain analyze select", "EXPLAIN ANALYZE SELECT 1", false},
		{"explain query plan", "EXPLAIN QUERY PLAN SELECT 1", false},
		{"explain options select", "EXPLAIN (ANALYZE, BUFFERS) SELECT 1", false},
		{"explain format mysql", "EXPLAIN FORMAT=JSON SELECT 1", false},
		{"explain analyze delete", "EXPLAIN ANALYZE DELETE FROM t", true},
		{"explain options delete", "EXPLAIN (ANALYZE, BUFFERS) DELETE FROM t", true},
		{"explain delete", "EXPLAIN DELETE FROM t", true},
		{"explain data-modifying cte", "EXPLAIN ANALYZE WITH a AS (DELETE FROM t RETURNING *) SELECT * FROM a", true},
		{"nested explain", "EXPLAIN EXPLAIN SELECT 1", true},
		{"explain nothing", "EXPLAIN", true},

		// PRAGMA is read-write; only schema inspection is allowed.
		{"pragma table_info", "PRAGMA table_info(users)", false},
		{"pragma table_list", "PRAGMA table_list", false},
		{"pragma with schema prefix", "PRAGMA main.table_info(users)", false},
		{"pragma query_only assignment", "PRAGMA query_only=0", true},
		{"pragma query_only function form", "PRAGMA query_only(0)", true},
		{"pragma journal_mode", "PRAGMA journal_mode", true},
		{"pragma writable_schema", "PRAGMA writable_schema=ON", true},
		{"pragma quoted name", `PRAGMA "query_only"(0)`, true},
		{"pragma nothing", "PRAGMA", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Check(tt.query)
			if tt.refused && err == nil {
				t.Fatalf("Check(%q) = nil, want refusal", tt.query)
			}
			if !tt.refused && err != nil {
				t.Fatalf("Check(%q) = %v, want allowed", tt.query, err)
			}
			if err != nil && !IsRefused(err) {
				t.Fatalf("Check(%q) returned %T, want *readonly.Error", tt.query, err)
			}
		})
	}
}

// The refusal has to say what was refused: a status bar line that only says
// "not allowed" leaves the user guessing which part of their query was the
// problem.
func TestErrorMessageNamesTheSubject(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{"DELETE FROM t", "DELETE"},
		{"PRAGMA journal_mode", "PRAGMA journal_mode"},
		{"SELECT 1; DELETE FROM t", "multiple statements"},
		{"WITH gone AS (DELETE FROM t RETURNING *) SELECT * FROM gone", "DELETE"},
		{"EXPLAIN ANALYZE DELETE FROM t", "DELETE"},
		{"SELECT * INTO backup FROM t", "SELECT ... INTO"},
	}
	for _, tt := range tests {
		err := Check(tt.query)
		if err == nil {
			t.Fatalf("Check(%q) = nil, want refusal", tt.query)
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Check(%q) message = %q, want it to mention %q", tt.query, err, tt.want)
		}
	}
}

// A refusal must survive wrapping: the status bar shows whatever error comes
// back, and a caller that adds context to it should not turn a guard refusal
// into an unrecognizable database error.
func TestIsRefusedUnwraps(t *testing.T) {
	err := Check("DELETE FROM t")
	if err == nil {
		t.Fatal("Check(DELETE) = nil, want refusal")
	}
	if !IsRefused(fmt.Errorf("executing query: %w", err)) {
		t.Fatal("IsRefused lost the refusal through a wrap")
	}
	if IsRefused(errors.New("connection reset")) {
		t.Fatal("IsRefused claimed a plain database error")
	}
}
