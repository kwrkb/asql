package sqlite

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLeadingKeyword(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"plain select", "SELECT * FROM t", "select"},
		{"leading whitespace", "  INSERT INTO t VALUES (1)", "insert"},
		{"line comment", "-- comment\nSELECT 1", "select"},
		{"block comment", "/* comment */ UPDATE t SET a=1", "update"},
		{"leading semicolon", ";; SELECT 1", "select"},
		{"empty string", "", ""},
		{"only comment", "-- nothing", ""},
		{"unclosed block comment", "/* unclosed SELECT 1", ""},
		{"mixed comments", "-- line\n/* block */\nDELETE FROM t", "delete"},
		{"uppercase", "PRAGMA table_info(t)", "pragma"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := leadingKeyword(tt.query)
			if got != tt.want {
				t.Errorf("leadingKeyword(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestContainsReturning(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"insert returning", "INSERT INTO t VALUES (1) RETURNING id", true},
		{"update returning", "UPDATE t SET a=1 RETURNING a", true},
		{"delete returning", "DELETE FROM t WHERE id=1 RETURNING *", true},
		{"insert no returning", "INSERT INTO t VALUES (1)", false},
		{"returning in string literal", "VALUES ('returning')", false},
		{"returning as quoted identifier", `INSERT INTO "returning" VALUES (1)`, false},
		{"returning in line comment", "INSERT INTO t VALUES (1) -- RETURNING id", false},
		{"returning in block comment", "INSERT INTO t VALUES (1) /* RETURNING */", false},
		{"partial match in name", "INSERT INTO t_returning_log VALUES (1)", false},
		{"mixed case", "UPDATE t SET a=1 Returning a", true},
		{"returning in escaped double-quoted identifier", `UPDATE t SET name = "a ""returning"" name"`, false},
		{"returning as backtick-quoted identifier", "INSERT INTO `returning` VALUES (1)", false},
		{"returning as bracket-quoted identifier", "INSERT INTO [returning] VALUES (1)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsReturning(tt.query)
			if got != tt.want {
				t.Errorf("containsReturning(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestReturnsRows(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"select", "SELECT 1", true},
		{"pragma", "PRAGMA table_info(t)", true},
		{"with", "WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"explain", "EXPLAIN SELECT 1", true},
		{"values", "VALUES (1, 2)", true},
		{"insert", "INSERT INTO t VALUES (1)", false},
		{"update", "UPDATE t SET a=1", false},
		{"delete", "DELETE FROM t", false},
		{"create", "CREATE TABLE t (id INTEGER)", false},
		{"empty", "", false},
		{"insert returning", "INSERT INTO t VALUES (1) RETURNING id", true},
		{"update returning", "UPDATE t SET a=1 RETURNING a", true},
		{"delete returning", "DELETE FROM t WHERE id=1 RETURNING *", true},
		{"returning in string", "INSERT INTO t VALUES ('returning')", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := returnsRows(tt.query)
			if got != tt.want {
				t.Errorf("returnsRows(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestStringifyValue(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"nil", nil, "NULL"},
		{"byte slice", []byte("hello"), "hello"},
		{"time", fixedTime, "2024-01-15T12:00:00Z"},
		{"int", 42, "42"},
		{"int64", int64(100), "100"},
		{"float64", 3.14, "3.14"},
		{"string", "world", "world"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"custom struct", struct{ X int }{42}, "{42}"},
		{"binary blob", []byte{0xDE, 0xAD, 0xBE, 0xEF}, "deadbeef"},
		{"empty byte slice", []byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringifyValue(tt.value)
			if got != tt.want {
				t.Errorf("stringifyValue(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestOpen(t *testing.T) {
	t.Run("memory db opens successfully", func(t *testing.T) {
		a, err := Open(":memory:")
		if err != nil {
			t.Fatalf("Open(':memory:') failed: %v", err)
		}
		defer a.Close()
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		_, err := Open("/nonexistent/path/that/does/not/exist/db.sqlite")
		if err == nil {
			t.Error("expected error for invalid path, got nil")
		}
	})
}

func TestTables(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) *Adapter {
		t.Helper()
		a, err := Open(":memory:")
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		t.Cleanup(func() { a.Close() })
		return a
	}

	t.Run("empty database returns no tables", func(t *testing.T) {
		a := setup(t)
		tables, err := a.Tables(ctx)
		if err != nil {
			t.Fatalf("Tables() failed: %v", err)
		}
		if len(tables) != 0 {
			t.Errorf("expected 0 tables, got %d: %v", len(tables), tables)
		}
	})

	t.Run("returns tables sorted by name", func(t *testing.T) {
		a := setup(t)
		for _, ddl := range []string{
			"CREATE TABLE zebra (id INTEGER)",
			"CREATE TABLE alpha (id INTEGER)",
			"CREATE TABLE middle (id INTEGER)",
		} {
			if _, err := a.Query(ctx, ddl); err != nil {
				t.Fatalf("failed: %v", err)
			}
		}

		tables, err := a.Tables(ctx)
		if err != nil {
			t.Fatalf("Tables() failed: %v", err)
		}
		if len(tables) != 3 {
			t.Fatalf("expected 3 tables, got %d", len(tables))
		}
		if tables[0] != "alpha" || tables[1] != "middle" || tables[2] != "zebra" {
			t.Errorf("expected [alpha middle zebra], got %v", tables)
		}
	})

	t.Run("excludes views", func(t *testing.T) {
		a := setup(t)
		if _, err := a.Query(ctx, "CREATE TABLE t (id INTEGER)"); err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		if _, err := a.Query(ctx, "CREATE VIEW v AS SELECT * FROM t"); err != nil {
			t.Fatalf("CREATE VIEW failed: %v", err)
		}

		tables, err := a.Tables(ctx)
		if err != nil {
			t.Fatalf("Tables() failed: %v", err)
		}
		if len(tables) != 1 || tables[0] != "t" {
			t.Errorf("expected [t], got %v", tables)
		}
	})
}

func TestQuery(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) *Adapter {
		t.Helper()
		a, err := Open(":memory:")
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		t.Cleanup(func() { a.Close() })
		return a
	}

	t.Run("SELECT returns columns and rows", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE users (id INTEGER, name TEXT)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		_, err = a.Query(ctx, "INSERT INTO users VALUES (1, 'alice'), (2, 'bob')")
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}

		result, err := a.Query(ctx, "SELECT id, name FROM users ORDER BY id")
		if err != nil {
			t.Fatalf("SELECT failed: %v", err)
		}
		if len(result.Columns) != 2 {
			t.Errorf("expected 2 columns, got %d", len(result.Columns))
		}
		if len(result.Rows) != 2 {
			t.Errorf("expected 2 rows, got %d", len(result.Rows))
		}
		if result.Rows[0][0] != "1" || result.Rows[0][1] != "alice" {
			t.Errorf("unexpected first row: %v", result.Rows[0])
		}
		if !strings.Contains(result.Message, "2 row(s) returned") {
			t.Errorf("unexpected message: %q", result.Message)
		}
	})

	t.Run("INSERT returns rows affected message", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE t (v INTEGER)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}

		result, err := a.Query(ctx, "INSERT INTO t VALUES (1), (2), (3)")
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}
		if !strings.Contains(result.Message, "3 row(s) affected") {
			t.Errorf("unexpected message: %q", result.Message)
		}
		if len(result.Columns) != 0 {
			t.Errorf("expected no columns for DML, got %d", len(result.Columns))
		}
	})

	t.Run("empty query returns error", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "")
		if err == nil {
			t.Error("expected error for empty query, got nil")
		}
	})

	t.Run("invalid SQL returns error", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "NOT VALID SQL")
		if err == nil {
			t.Error("expected error for invalid SQL, got nil")
		}
	})

	t.Run("SELECT with no rows returns empty rows", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE empty_t (id INTEGER)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}

		result, err := a.Query(ctx, "SELECT * FROM empty_t")
		if err != nil {
			t.Fatalf("SELECT failed: %v", err)
		}
		if len(result.Rows) != 0 {
			t.Errorf("expected 0 rows, got %d", len(result.Rows))
		}
		if len(result.Columns) != 1 {
			t.Errorf("expected 1 column, got %d", len(result.Columns))
		}
	})

	t.Run("INSERT RETURNING returns columns and rows", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}

		result, err := a.Query(ctx, "INSERT INTO t VALUES (1, 'hello') RETURNING id, v")
		if err != nil {
			t.Fatalf("INSERT RETURNING failed: %v", err)
		}
		if len(result.Columns) != 2 {
			t.Errorf("expected 2 columns, got %d", len(result.Columns))
		}
		if len(result.Rows) != 1 {
			t.Errorf("expected 1 row, got %d", len(result.Rows))
		}
		if result.Rows[0][0] != "1" || result.Rows[0][1] != "hello" {
			t.Errorf("unexpected row: %v", result.Rows[0])
		}
		if !strings.Contains(result.Message, "1 row(s) returned") {
			t.Errorf("unexpected message: %q", result.Message)
		}
	})

	t.Run("UPDATE RETURNING returns updated values", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE t (id INTEGER, v INTEGER)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		_, err = a.Query(ctx, "INSERT INTO t VALUES (1, 10)")
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}

		result, err := a.Query(ctx, "UPDATE t SET v=20 WHERE id=1 RETURNING v")
		if err != nil {
			t.Fatalf("UPDATE RETURNING failed: %v", err)
		}
		if len(result.Rows) != 1 || result.Rows[0][0] != "20" {
			t.Errorf("unexpected rows: %v", result.Rows)
		}
	})

	t.Run("DELETE RETURNING returns deleted rows", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE t (id INTEGER, v TEXT)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		_, err = a.Query(ctx, "INSERT INTO t VALUES (1, 'bye')")
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}

		result, err := a.Query(ctx, "DELETE FROM t WHERE id=1 RETURNING id, v")
		if err != nil {
			t.Fatalf("DELETE RETURNING failed: %v", err)
		}
		if len(result.Rows) != 1 || result.Rows[0][0] != "1" || result.Rows[0][1] != "bye" {
			t.Errorf("unexpected rows: %v", result.Rows)
		}
	})

	t.Run("BLOB column displays as hex", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE t (data BLOB)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		_, err = a.Query(ctx, "INSERT INTO t VALUES (X'DEADBEEF')")
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}

		result, err := a.Query(ctx, "SELECT data FROM t")
		if err != nil {
			t.Fatalf("SELECT failed: %v", err)
		}
		if len(result.Rows) != 1 || result.Rows[0][0] != "deadbeef" {
			t.Errorf("expected hex 'deadbeef', got %v", result.Rows)
		}
	})

	t.Run("whitespace-only query returns error", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "   ")
		if err == nil {
			t.Error("expected error for whitespace-only query, got nil")
		}
	})

	t.Run("NULL value SELECT returns NULL string", func(t *testing.T) {
		a := setup(t)
		result, err := a.Query(ctx, "SELECT NULL")
		if err != nil {
			t.Fatalf("SELECT NULL failed: %v", err)
		}
		if len(result.Rows) != 1 || result.Rows[0][0] != "NULL" {
			t.Errorf("expected 'NULL', got %v", result.Rows)
		}
	})
}
