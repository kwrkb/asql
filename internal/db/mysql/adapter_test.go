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

// TestBuildConfig asserts on the driver Config, not on a DSN string: the
// Config is what the connection is actually made from, so a string-level
// assertion can pass while the connection still authenticates as someone else.
func TestBuildConfig(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantUser      string
		wantPass      string
		wantDBName    string
		wantAddr      string
		wantParseTime bool
		// Substrings the driver's own rendering of the config must contain.
		// Some params (charset, loc, tls) land in dedicated Config fields
		// rather than in Params, so FormatDSN is the one place all of them are
		// observable.
		wantDSNContains []string
	}{
		{
			name: "full URL", input: "mysql://root:pass@127.0.0.1:3306/testdb",
			wantUser: "root", wantPass: "pass", wantDBName: "testdb",
			wantAddr: "127.0.0.1:3306", wantParseTime: true,
		},
		{
			name:     "with existing params",
			input:    "mysql://root:pass@127.0.0.1:3306/testdb?charset=utf8mb4&readTimeout=5s",
			wantUser: "root", wantPass: "pass", wantDBName: "testdb",
			wantAddr: "127.0.0.1:3306", wantParseTime: true,
			wantDSNContains: []string{"charset=utf8mb4", "readTimeout=5s", "parseTime=true"},
		},
		{
			name: "parseTime already set", input: "mysql://root@localhost:3306/db?parseTime=false",
			wantUser: "root", wantDBName: "db", wantAddr: "localhost:3306", wantParseTime: false,
		},
		{
			name: "no port", input: "mysql://root@localhost/db",
			wantUser: "root", wantDBName: "db", wantAddr: "localhost:3306", wantParseTime: true,
		},
		{
			name: "no user", input: "mysql://localhost:3306/db",
			wantDBName: "db", wantAddr: "localhost:3306", wantParseTime: true,
		},
		{
			name: "no host", input: "mysql://root@/db",
			wantUser: "root", wantDBName: "db", wantAddr: "127.0.0.1:3306", wantParseTime: true,
		},
		{
			name: "already go-sql-driver format", input: "root:pass@tcp(127.0.0.1:3306)/testdb",
			wantUser: "root", wantPass: "pass", wantDBName: "testdb",
			wantAddr: "127.0.0.1:3306", wantParseTime: false,
		},

		// Credentials the URL form escapes but the driver DSN form does not.
		{
			name: "at sign in password", input: "mysql://root:p%40ss@127.0.0.1:3306/testdb",
			wantUser: "root", wantPass: "p@ss", wantDBName: "testdb",
			wantAddr: "127.0.0.1:3306", wantParseTime: true,
		},
		{
			name: "colon in password", input: "mysql://root:p%3Ass@127.0.0.1:3306/testdb",
			wantUser: "root", wantPass: "p:ss", wantDBName: "testdb",
			wantAddr: "127.0.0.1:3306", wantParseTime: true,
		},
		{
			name: "percent in password", input: "mysql://root:p%25ss@127.0.0.1:3306/testdb",
			wantUser: "root", wantPass: "p%ss", wantDBName: "testdb",
			wantAddr: "127.0.0.1:3306", wantParseTime: true,
		},
		{
			name: "slash in password", input: "mysql://root:p%2Fss@127.0.0.1:3306/testdb",
			wantUser: "root", wantPass: "p/ss", wantDBName: "testdb",
			wantAddr: "127.0.0.1:3306", wantParseTime: true,
		},
		{
			name: "at sign in user", input: "mysql://ad%40min:pass@127.0.0.1:3306/testdb",
			wantUser: "ad@min", wantPass: "pass", wantDBName: "testdb",
			wantAddr: "127.0.0.1:3306", wantParseTime: true,
		},
		// The case the DSN string form cannot express at all: the driver would
		// split on the first colon and authenticate as "first"/"last:secret".
		{
			name: "colon in user", input: "mysql://first%3Alast:secret@127.0.0.1:3306/testdb",
			wantUser: "first:last", wantPass: "secret", wantDBName: "testdb",
			wantAddr: "127.0.0.1:3306", wantParseTime: true,
		},
		{
			name: "colon in user and password", input: "mysql://a%3Ab:c%3Ad@127.0.0.1:3306/testdb",
			wantUser: "a:b", wantPass: "c:d", wantDBName: "testdb",
			wantAddr: "127.0.0.1:3306", wantParseTime: true,
		},
		{
			name: "percent in dbname", input: "mysql://root:pass@127.0.0.1:3306/test%25db",
			wantUser: "root", wantPass: "pass", wantDBName: "test%db",
			wantAddr: "127.0.0.1:3306", wantParseTime: true,
		},
		{
			name: "no password", input: "mysql://root@127.0.0.1:3306/testdb",
			wantUser: "root", wantDBName: "testdb",
			wantAddr: "127.0.0.1:3306", wantParseTime: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := buildConfig(tt.input)
			if err != nil {
				t.Fatalf("buildConfig(%q) failed: %v", tt.input, err)
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
			if cfg.Addr != tt.wantAddr {
				t.Errorf("Addr = %q, want %q", cfg.Addr, tt.wantAddr)
			}
			if cfg.Net != "tcp" {
				t.Errorf("Net = %q, want %q", cfg.Net, "tcp")
			}
			if cfg.ParseTime != tt.wantParseTime {
				t.Errorf("ParseTime = %v, want %v", cfg.ParseTime, tt.wantParseTime)
			}
			formatted := cfg.FormatDSN()
			for _, want := range tt.wantDSNContains {
				if !strings.Contains(formatted, want) {
					t.Errorf("FormatDSN() = %q, want it to contain %q", formatted, want)
				}
			}
		})
	}
}

// The config must survive the driver's own normalize/FormatDSN round trip,
// which is what NewConnector puts it through.
func TestBuildConfig_AcceptedByConnector(t *testing.T) {
	cfg, err := buildConfig("mysql://first%3Alast:p%40ss@127.0.0.1:3306/testdb")
	if err != nil {
		t.Fatalf("buildConfig failed: %v", err)
	}
	if _, err := gomysql.NewConnector(cfg); err != nil {
		t.Fatalf("NewConnector failed: %v", err)
	}
}

func TestBuildConfig_InvalidDSN(t *testing.T) {
	if _, err := buildConfig("root:pass@tcp(127.0.0.1:3306)"); err == nil {
		t.Error("buildConfig() expected an error for a DSN with no '/', got nil")
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
