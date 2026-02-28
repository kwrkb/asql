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
