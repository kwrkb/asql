package mysql

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
		{"show tables", "SHOW TABLES", true},
		{"show create", "SHOW CREATE TABLE t", true},
		{"describe", "DESCRIBE users", true},
		{"desc", "DESC users", true},
		{"explain", "EXPLAIN SELECT 1", true},
		{"with select", "WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"with delete", "WITH cte AS (SELECT 1) DELETE FROM t WHERE id IN (SELECT * FROM cte)", false},
		{"with update", "WITH cte AS (SELECT 1) UPDATE t SET a=1", false},
		{"with insert", "WITH cte AS (SELECT 1) INSERT INTO t SELECT * FROM cte", false},
		{"values", "VALUES ROW(1, 2)", true},
		{"table", "TABLE users", true},
		{"insert", "INSERT INTO t VALUES (1)", false},
		{"update", "UPDATE t SET a=1", false},
		{"delete", "DELETE FROM t", false},
		{"create", "CREATE TABLE t (id INT)", false},
		{"empty", "", false},
		{"comment then select", "-- comment\nSELECT 1", true},
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

func TestConvertDSN(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"full URL",
			"mysql://root:pass@127.0.0.1:3306/testdb",
			"root:pass@tcp(127.0.0.1:3306)/testdb?parseTime=true",
		},
		{
			"with existing params",
			"mysql://root:pass@127.0.0.1:3306/testdb?charset=utf8mb4",
			"root:pass@tcp(127.0.0.1:3306)/testdb?charset=utf8mb4&parseTime=true",
		},
		{
			"parseTime already set",
			"mysql://root@localhost:3306/db?parseTime=false",
			"root@tcp(localhost:3306)/db?parseTime=false",
		},
		{
			"no port",
			"mysql://root@localhost/db",
			"root@tcp(localhost)/db?parseTime=true",
		},
		{
			"no user",
			"mysql://localhost:3306/db",
			"@tcp(localhost:3306)/db?parseTime=true",
		},
		{
			"already go-sql-driver format",
			"root:pass@tcp(127.0.0.1:3306)/testdb",
			"root:pass@tcp(127.0.0.1:3306)/testdb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertDSN(tt.input)
			if got != tt.want {
				t.Errorf("convertDSN(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIntegration(t *testing.T) {
	dsn := os.Getenv("ASQL_MYSQL_DSN")
	if dsn == "" {
		t.Skip("ASQL_MYSQL_DSN not set, skipping MySQL integration tests")
	}

	a, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open(%q) failed: %v", dsn, err)
	}
	defer a.Close()

	ctx := context.Background()

	t.Run("Type", func(t *testing.T) {
		if a.Type() != "mysql" {
			t.Errorf("Type() = %q, want %q", a.Type(), "mysql")
		}
	})

	t.Run("SHOW TABLES", func(t *testing.T) {
		_, err := a.Tables(ctx)
		if err != nil {
			t.Fatalf("Tables() failed: %v", err)
		}
	})

	t.Run("Schema", func(t *testing.T) {
		schema, err := a.Schema(ctx)
		if err != nil {
			t.Fatalf("Schema() failed: %v", err)
		}
		// Schema may be empty if no tables exist
		_ = schema
	})

	t.Run("SELECT VERSION()", func(t *testing.T) {
		result, err := a.Query(ctx, "SELECT VERSION()")
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
