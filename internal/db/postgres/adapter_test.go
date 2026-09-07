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
		{"with select", "WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"with delete", "WITH cte AS (SELECT 1) DELETE FROM t WHERE id IN (SELECT * FROM cte)", false},
		{"with delete returning", "WITH cte AS (SELECT 1) DELETE FROM t WHERE id IN (SELECT * FROM cte) RETURNING *", true},
		{"with update", "WITH cte AS (SELECT 1) UPDATE t SET a=1", false},
		{"with insert returning", "WITH cte AS (SELECT 1) INSERT INTO t SELECT * FROM cte RETURNING id", true},
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

func TestType(t *testing.T) {
	a := &Adapter{}
	if got := a.Type(); got != "postgres" {
		t.Errorf("Type() = %q, want %q", got, "postgres")
	}
}

func TestQuoteIdentifier(t *testing.T) {
	a := &Adapter{}
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "users", `"users"`},
		{"double-quote escape", `us"ers`, `"us""ers"`},
		{"reserved word", "select", `"select"`},
		{"empty string", "", `""`},
		{"multiple double-quotes", `a""b`, `"a""""b"`},
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
	_, err := Open("postgres://root@127.0.0.1:1/db")
	if err == nil {
		t.Error("Open() expected error for unreachable host, got nil")
	}
}

// A url.Parse failure must not leak the password. Two places carry it:
// url.Error embeds the raw URL, and its cause embeds the offending escape
// sequence — which can be the whole password. withParam runs before any
// connection is attempted, so `asql --readonly <bad postgres URL>` reaches this
// on stderr, and the profile switch reaches it in the TUI status line.
func TestWithParam_ParseErrorRedactsPassword(t *testing.T) {
	for _, tc := range []struct {
		name    string
		dsn     string
		secrets []string
	}{
		// The plain report of the issue: a bad port, and the whole password
		// printed back inside the url.Error's copy of the URL.
		{"bad port", "postgres://alice:review-secret@localhost:bad/db", []string{"review-secret"}},
		// The escape is part of a longer password: the URL carries "p%ss" and
		// the cause carries "%ss", and "%ss" alone is most of the secret.
		{"escape inside the password", "postgres://alice:p%ss@127.0.0.1:5432/prod", []string{"p%ss", "%ss"}},
		// The escape *is* the password: redacting only the URL leaves it whole
		// in the cause.
		{"escape is the password", "postgres://alice:%ss@127.0.0.1:5432/prod", []string{"%ss"}},
		// net/url reads userinfo up to the last '@', so the password is
		// "sec@ret" and the failure is the port. The masked DSN must not carry
		// the tail of the password either.
		{"at-sign in the password", "postgres://alice:sec@ret@127.0.0.1:bad/prod", []string{"sec@ret", "ret@"}},
		// A literal '/' in the password is only reachable here — url.Parse
		// rejects it — and it is what MaskDSN's primary pattern cannot match.
		{"slash in the password", "postgres://alice:se/cret@127.0.0.1:bad/prod", []string{"se/cret", "cret"}},
		// The password is a query parameter rather than userinfo.
		{"password as a query parameter", "postgres://alice@127.0.0.1:bad/prod?password=review-secret", []string{"review-secret"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, call := range []struct {
				via string
				err error
			}{
				{"withParam", func() error {
					_, err := withParam(tc.dsn, "default_transaction_read_only", "on")
					return err
				}()},
				// OpenReadonly is the real entry point and must not re-wrap the
				// error back into something that carries the raw DSN.
				{"OpenReadonly", func() error {
					_, err := OpenReadonly(tc.dsn)
					return err
				}()},
			} {
				if call.err == nil {
					t.Fatalf("%s() expected an error for an unparseable URL, got nil", call.via)
				}
				msg := call.err.Error()
				for _, secret := range tc.secrets {
					if strings.Contains(msg, secret) {
						t.Errorf("%s() error leaks %q: %q", call.via, secret, msg)
					}
				}
				if !strings.Contains(msg, "***") {
					t.Errorf("%s() error does not show the DSN masked: %q", call.via, msg)
				}
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
