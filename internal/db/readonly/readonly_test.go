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
		dbType  string
		query   string
		refused bool
	}{
		// Plain read-only statements.
		{"select", "sqlite", "SELECT * FROM users", false},
		{"select lowercase", "sqlite", "select 1", false},
		{"values", "sqlite", "VALUES (1), (2)", false},
		{"table", "postgres", "TABLE users", false},
		{"show", "mysql", "SHOW TABLES", false},
		{"describe", "mysql", "DESCRIBE users", false},
		{"desc", "mysql", "DESC users", false},
		{"trailing semicolon", "sqlite", "SELECT 1;", false},
		{"leading comments", "sqlite", "/* comment */ -- x\n SELECT 1", false},
		{"keyword inside string literal", "sqlite", "SELECT 'DELETE FROM t'", false},
		{"keyword inside identifier", "sqlite", "SELECT * FROM delete_log", false},

		// A read-only leading keyword is not enough: these SELECTs write.
		{"select into table", "postgres", "SELECT * INTO backup FROM production_table", true},
		{"select into temp table", "postgres", "SELECT * INTO TEMP backup FROM t", true},
		{"select into outfile", "mysql", "SELECT * FROM t INTO OUTFILE '/tmp/x.csv'", true},
		{"select into dumpfile", "mysql", "SELECT a FROM t INTO DUMPFILE '/tmp/x.bin'", true},
		{"select into variable", "mysql", "SELECT a INTO @v FROM t", true},
		{"with body select into", "postgres", "WITH a AS (SELECT 1) SELECT * INTO backup FROM a", true},
		{"explain select into", "postgres", "EXPLAIN SELECT * INTO backup FROM t", true},
		{"into inside string literal", "postgres", "SELECT 'INTO' FROM t", false},
		{"into inside identifier", "postgres", "SELECT * FROM into_log", false},
		{"into as quoted identifier", "postgres", `SELECT "into" FROM t`, false},
		{"into inside comment", "postgres", "SELECT 1 -- INTO backup", false},

		// Writing statements.
		{"insert", "sqlite", "INSERT INTO t VALUES (1)", true},
		{"update", "sqlite", "UPDATE t SET a = 1", true},
		{"delete", "sqlite", "DELETE FROM t", true},
		{"drop", "sqlite", "DROP TABLE t", true},
		{"create", "sqlite", "CREATE TABLE t (a TEXT)", true},
		{"alter", "sqlite", "ALTER TABLE t ADD COLUMN b TEXT", true},
		{"truncate", "mysql", "TRUNCATE TABLE t", true},
		{"replace", "sqlite", "REPLACE INTO t VALUES (1)", true},
		{"merge", "postgres", "MERGE INTO t USING s ON t.a = s.a", true},
		{"grant", "postgres", "GRANT SELECT ON t TO bob", true},
		{"vacuum", "sqlite", "VACUUM", true},
		{"comment hides nothing", "sqlite", "-- harmless\nDELETE FROM t", true},

		// ATTACH: SQLite's mode=ro still allows it, so the guard must refuse.
		{"attach", "sqlite", "ATTACH DATABASE 'side.db' AS side", true},

		// Unknown keywords fail closed.
		{"unknown keyword", "sqlite", "FROBNICATE t", true},
		{"empty", "sqlite", "", true},
		{"comment only", "sqlite", "-- nothing here", true},

		// Multiple statements: the leading keyword describes only the first.
		{"select then delete", "postgres", "SELECT 1; DELETE FROM t", true},
		{"select then select", "postgres", "SELECT 1; SELECT 2", true},
		{"semicolon in string", "sqlite", "SELECT ';'", false},
		{"semicolon in comment", "sqlite", "SELECT 1 -- ; DELETE FROM t", false},

		// WITH: the body keyword alone is not enough.
		{"with select", "postgres", "WITH a AS (SELECT 1) SELECT * FROM a", false},
		{"with recursive", "postgres", "WITH RECURSIVE a AS (SELECT 1) SELECT * FROM a", false},
		{"with column list", "postgres", "WITH a (x) AS (SELECT 1) SELECT * FROM a", false},
		{"with materialized", "postgres", "WITH a AS MATERIALIZED (SELECT 1) SELECT * FROM a", false},
		{"with not materialized", "postgres", "WITH a AS NOT MATERIALIZED (SELECT 1) SELECT * FROM a", false},
		{"with dml body", "postgres", "WITH a AS (SELECT 1) DELETE FROM t", true},
		{
			"data-modifying cte",
			"postgres",
			"WITH gone AS (DELETE FROM t RETURNING *) SELECT * FROM gone",
			true,
		},
		{
			"data-modifying second cte",
			"postgres",
			"WITH a AS (SELECT 1), b AS (UPDATE t SET x = 1 RETURNING *) SELECT * FROM a",
			true,
		},
		{
			"data-modifying nested cte",
			"postgres",
			"WITH a AS (WITH b AS (DELETE FROM t RETURNING *) SELECT * FROM b) SELECT * FROM a",
			true,
		},
		{"with subquery in body", "postgres", "WITH a AS (SELECT 1) SELECT * FROM (SELECT 2) s", false},
		{"with named column definition list", "postgres",
			"WITH a AS (SELECT 1) SELECT * FROM json_to_record('{}') AS r(x int)", false},
		// Known conservative rejection: an unnamed column definition list in
		// the body looks exactly like a CTE term to the scanner. Refusing it
		// is the allow-list failing closed; naming the alias avoids it.
		{"with unnamed column definition list is refused", "postgres",
			"WITH a AS (SELECT 1) SELECT * FROM f() AS (x int, y text)", true},

		// EXPLAIN: ANALYZE executes its target on PostgreSQL.
		{"explain select", "postgres", "EXPLAIN SELECT 1", false},
		{"explain analyze select", "postgres", "EXPLAIN ANALYZE SELECT 1", false},
		{"explain query plan", "sqlite", "EXPLAIN QUERY PLAN SELECT 1", false},
		{"explain options select", "postgres", "EXPLAIN (ANALYZE, BUFFERS) SELECT 1", false},
		{"explain format mysql", "mysql", "EXPLAIN FORMAT=JSON SELECT 1", false},
		{"explain analyze delete", "postgres", "EXPLAIN ANALYZE DELETE FROM t", true},
		{"explain options delete", "postgres", "EXPLAIN (ANALYZE, BUFFERS) DELETE FROM t", true},
		{"explain delete", "postgres", "EXPLAIN DELETE FROM t", true},
		{"explain data-modifying cte", "postgres", "EXPLAIN ANALYZE WITH a AS (DELETE FROM t RETURNING *) SELECT * FROM a", true},
		{"nested explain", "postgres", "EXPLAIN EXPLAIN SELECT 1", true},
		{"explain nothing", "postgres", "EXPLAIN", true},

		// PRAGMA is read-write; only schema inspection is allowed.
		{"pragma table_info", "sqlite", "PRAGMA table_info(users)", false},
		{"pragma table_list", "sqlite", "PRAGMA table_list", false},
		{"pragma with schema prefix", "sqlite", "PRAGMA main.table_info(users)", false},
		{"pragma query_only assignment", "sqlite", "PRAGMA query_only=0", true},
		{"pragma query_only function form", "sqlite", "PRAGMA query_only(0)", true},
		{"pragma journal_mode", "sqlite", "PRAGMA journal_mode", true},
		{"pragma writable_schema", "sqlite", "PRAGMA writable_schema=ON", true},
		{"pragma quoted name", "sqlite", `PRAGMA "query_only"(0)`, true},
		{"pragma nothing", "sqlite", "PRAGMA", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Check(tt.query, tt.dbType)
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
		err := Check(tt.query, "sqlite")
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
	err := Check("DELETE FROM t", "sqlite")
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
