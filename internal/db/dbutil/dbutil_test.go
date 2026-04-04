package dbutil

import (
	"testing"
	"time"
)

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
		{"binary blob", []byte{0xDE, 0xAD, 0xBE, 0xEF}, "deadbeef"},
		{"empty byte slice", []byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringifyValue(tt.value)
			if got != tt.want {
				t.Errorf("StringifyValue(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

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
		{"hash comment", "# comment\nSELECT 1", "select"},
		{"hash comment only", "# nothing", ""},
		{"hash then block comment", "# line\n/* block */\nDELETE FROM t", "delete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LeadingKeyword(tt.query)
			if got != tt.want {
				t.Errorf("LeadingKeyword(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestShortenTypeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"INTEGER", "int"},
		{"integer", "int"},
		{"Integer", "int"},
		{"UNSIGNED BIGINT", "ubigint"},
		{"TIMESTAMPTZ", "tstz"},
		{"TEXT", "text"},
		{"VARCHAR", "varchar"},
		{"INT4", "int4"},
		{"BOOL", "bool"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ShortenTypeName(tt.input)
			if got != tt.want {
				t.Errorf("ShortenTypeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestContainsReturning(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		dialect Dialect
		want    bool
	}{
		// Basic cases
		{"plain returning", "INSERT INTO t VALUES (1) RETURNING id", Dialect{}, true},
		{"no returning", "INSERT INTO t VALUES (1)", Dialect{}, false},
		{"returning in string", "INSERT INTO t VALUES ('RETURNING')", Dialect{}, false},
		{"returning in double quote", `INSERT INTO t VALUES (1) WHERE "RETURNING" = 1`, Dialect{}, false},
		{"returning in line comment", "INSERT INTO t -- RETURNING\nVALUES (1)", Dialect{}, false},
		{"returning in block comment", "INSERT INTO t /* RETURNING */ VALUES (1)", Dialect{}, false},
		{"case insensitive", "insert into t values (1) returning id", Dialect{}, true},
		{"partial match", "INSERT INTO returning_table VALUES (1)", Dialect{}, false},

		// SQLite dialect: bracket + backtick
		{"bracket quoted", "INSERT INTO [RETURNING] VALUES (1)", Dialect{BracketQuote: true}, false},
		{"backtick quoted", "INSERT INTO `RETURNING` VALUES (1)", Dialect{BacktickQuote: true}, false},
		{"bracket with real returning", "INSERT INTO [t] VALUES (1) RETURNING id", Dialect{BracketQuote: true}, true},

		// PostgreSQL dialect: dollar-quoted
		{"dollar quoted", "INSERT INTO t VALUES ($$RETURNING$$)", Dialect{DollarQuote: true}, false},
		{"named dollar tag", "INSERT INTO t VALUES ($fn$RETURNING$fn$)", Dialect{DollarQuote: true}, false},
		{"dollar with real returning", "INSERT INTO t VALUES ($$hello$$) RETURNING id", Dialect{DollarQuote: true}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsReturning(tt.query, tt.dialect)
			if got != tt.want {
				t.Errorf("ContainsReturning(%q, %+v) = %v, want %v", tt.query, tt.dialect, got, tt.want)
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

func TestCteBodyKeyword(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"cte select", "WITH cte AS (SELECT 1) SELECT * FROM cte", "select"},
		{"cte insert", "WITH cte AS (SELECT 1) INSERT INTO t SELECT * FROM cte", "insert"},
		{"cte update", "WITH cte AS (SELECT 1) UPDATE t SET a=1", "update"},
		{"cte delete", "WITH cte AS (SELECT 1) DELETE FROM t", "delete"},
		{"recursive cte", "WITH RECURSIVE cte AS (SELECT 1) SELECT * FROM cte", "select"},
		{"multiple ctes", "WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a, b", "select"},
		{"nested parens", "WITH cte AS (SELECT (1+2) FROM t) DELETE FROM t2", "delete"},
		{"string in cte", "WITH cte AS (SELECT 'hello)world' FROM t) SELECT * FROM cte", "select"},
		{"comment in cte", "WITH cte AS (/* ) */ SELECT 1) SELECT * FROM cte", "select"},
		{"cte values", "WITH cte AS (SELECT 1) VALUES (1, 2)", "values"},
		{"cte table", "WITH cte AS (SELECT 1) TABLE users", "table"},
		{"cte explain", "WITH cte AS (SELECT 1) EXPLAIN SELECT 1", "explain"},
		{"empty", "", ""},
		{"only with", "WITH", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CteBodyKeyword(tt.query)
			if got != tt.want {
				t.Errorf("CteBodyKeyword(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}
