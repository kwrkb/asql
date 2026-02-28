package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestReturnsRows(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"select", "SELECT 1", true},
		{"show", "SHOW search_path", true},
		{"explain", "EXPLAIN SELECT 1", true},
		{"with", "WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"values", "VALUES (1, 2)", true},
		{"table", "TABLE users", true},
		{"insert", "INSERT INTO t VALUES (1)", false},
		{"update", "UPDATE t SET a=1", false},
		{"delete", "DELETE FROM t", false},
		{"create", "CREATE TABLE t (id INT)", false},
		{"empty", "", false},
		{"insert returning", "INSERT INTO t VALUES (1) RETURNING id", true},
		{"update returning", "UPDATE t SET a=1 RETURNING a", true},
		{"delete returning", "DELETE FROM t WHERE id=1 RETURNING *", true},
		{"returning in string", "INSERT INTO t VALUES ('returning')", false},
		{"returning in double-quoted id", `INSERT INTO "returning" VALUES (1)`, false},
		{"returning in dollar-quoted", "INSERT INTO t VALUES ($$returning$$)", false},
		{"returning in tagged dollar-quote", "INSERT INTO t VALUES ($tag$returning$tag$)", false},
		{"comment then select", "-- comment\nSELECT 1", true},
		{"returning in line comment", "INSERT INTO t VALUES (1) -- RETURNING id", false},
		{"returning in block comment", "INSERT INTO t VALUES (1) /* RETURNING */", false},
		{"partial match", "INSERT INTO t_returning_log VALUES (1)", false},
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

func TestContainsReturning(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"basic returning", "INSERT INTO t VALUES (1) RETURNING id", true},
		{"no returning", "INSERT INTO t VALUES (1)", false},
		{"in string literal", "INSERT INTO t VALUES ('returning')", false},
		{"in double-quoted id", `INSERT INTO "returning" VALUES (1)`, false},
		{"in dollar-quoted", "INSERT INTO t VALUES ($$returning$$)", false},
		{"in tagged dollar-quote", "INSERT INTO t VALUES ($fn$returning$fn$)", false},
		{"in line comment", "INSERT INTO t -- RETURNING\n VALUES (1)", false},
		{"in block comment", "INSERT INTO t /* RETURNING */ VALUES (1)", false},
		{"partial match", "INSERT INTO returning_log VALUES (1)", false},
		{"mixed case", "INSERT INTO t VALUES (1) Returning id", true},
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

func TestParseDollarTag(t *testing.T) {
	tests := []struct {
		name  string
		query string
		pos   int
		want  string
	}{
		{"empty tag", "$$hello$$", 0, "$$"},
		{"named tag", "$fn$hello$fn$", 0, "$fn$"},
		{"not dollar", "hello", 0, ""},
		{"unclosed", "$fn", 0, ""},
		{"invalid char", "$a b$", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDollarTag(tt.query, tt.pos)
			if got != tt.want {
				t.Errorf("parseDollarTag(%q, %d) = %q, want %q", tt.query, tt.pos, got, tt.want)
			}
		})
	}
}

func TestIntegration(t *testing.T) {
	dsn := os.Getenv("ASQL_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("ASQL_POSTGRES_DSN not set, skipping PostgreSQL integration tests")
	}

	a, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open(%q) failed: %v", dsn, err)
	}
	defer a.Close()

	ctx := context.Background()

	t.Run("Type", func(t *testing.T) {
		if a.Type() != "postgres" {
			t.Errorf("Type() = %q, want %q", a.Type(), "postgres")
		}
	})

	t.Run("Tables", func(t *testing.T) {
		_, err := a.Tables(ctx)
		if err != nil {
			t.Fatalf("Tables() failed: %v", err)
		}
	})

	t.Run("Schema", func(t *testing.T) {
		_, err := a.Schema(ctx)
		if err != nil {
			t.Fatalf("Schema() failed: %v", err)
		}
	})

	t.Run("SELECT version()", func(t *testing.T) {
		result, err := a.Query(ctx, "SELECT version()")
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if len(result.Rows) != 1 {
			t.Errorf("expected 1 row, got %d", len(result.Rows))
		}
		if !strings.Contains(result.Message, "1 row(s) returned") {
			t.Errorf("unexpected message: %q", result.Message)
		}
	})
}
