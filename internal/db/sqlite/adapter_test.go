package sqlite

import (
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
