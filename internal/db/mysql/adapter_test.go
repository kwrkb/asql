package mysql

import (
	"context"
	"os"
	"strings"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
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
		{
			"percent-encoded password is decoded",
			"mysql://root:p%40ss@127.0.0.1:3306/testdb",
			"root:p@ss@tcp(127.0.0.1:3306)/testdb?parseTime=true",
		},
		{
			"percent-encoded user is decoded",
			"mysql://ad%40min:pass@127.0.0.1:3306/testdb",
			"ad@min:pass@tcp(127.0.0.1:3306)/testdb?parseTime=true",
		},
		{
			"dbname keeps its escaped form",
			"mysql://root:pass@127.0.0.1:3306/test%25db",
			"root:pass@tcp(127.0.0.1:3306)/test%25db?parseTime=true",
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

// TestConvertDSNParsesBackToCredentials checks the converted DSN against the
// driver's own parser: it is the parser that decides what the credentials and
// dbname actually end up being, so asserting on the string alone can pass while
// the connection still authenticates with the wrong password.
func TestConvertDSNParsesBackToCredentials(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantUser   string
		wantPass   string
		wantDBName string
	}{
		{"plain", "mysql://root:pass@127.0.0.1:3306/testdb", "root", "pass", "testdb"},
		{"at sign in password", "mysql://root:p%40ss@127.0.0.1:3306/testdb", "root", "p@ss", "testdb"},
		{"colon in password", "mysql://root:p%3Ass@127.0.0.1:3306/testdb", "root", "p:ss", "testdb"},
		{"percent in password", "mysql://root:p%25ss@127.0.0.1:3306/testdb", "root", "p%ss", "testdb"},
		{"slash in password", "mysql://root:p%2Fss@127.0.0.1:3306/testdb", "root", "p/ss", "testdb"},
		{"at sign in user", "mysql://ad%40min:pass@127.0.0.1:3306/testdb", "ad@min", "pass", "testdb"},
		{"percent in dbname", "mysql://root:pass@127.0.0.1:3306/test%25db", "root", "pass", "test%db"},
		{"no password", "mysql://root@127.0.0.1:3306/testdb", "root", "", "testdb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := gomysql.ParseDSN(convertDSN(tt.url))
			if err != nil {
				t.Fatalf("ParseDSN(%q) failed: %v", convertDSN(tt.url), err)
			}
			if cfg.User != tt.wantUser {
				t.Errorf("User = %q, want %q", cfg.User, tt.wantUser)
			}
			if cfg.Passwd != tt.wantPass {
				t.Errorf("Passwd = %q, want %q", cfg.Passwd, tt.wantPass)
			}
			if cfg.DBName != tt.wantDBName {
				t.Errorf("DBName = %q, want %q", cfg.DBName, tt.wantDBName)
			}
			if cfg.Addr != "127.0.0.1:3306" {
				t.Errorf("Addr = %q, want %q", cfg.Addr, "127.0.0.1:3306")
			}
		})
	}
}

func TestType(t *testing.T) {
	a := &Adapter{}
	if got := a.Type(); got != "mysql" {
		t.Errorf("Type() = %q, want %q", got, "mysql")
	}
}

func TestQuoteIdentifier(t *testing.T) {
	a := &Adapter{}
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "users", "`users`"},
		{"backtick escape", "us`ers", "`us``ers`"},
		{"reserved word", "select", "`select`"},
		{"empty string", "", "``"},
		{"multiple backticks", "a``b", "`a````b`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := a.QuoteIdentifier(tt.input)
			if got != tt.want {
				t.Errorf("QuoteIdentifier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestOpen_ErrorPaths(t *testing.T) {
	// port 1 is always connection refused — fails fast without timeout
	_, err := Open("mysql://root@127.0.0.1:1/db")
	if err == nil {
		t.Error("Open() expected error for unreachable host, got nil")
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
