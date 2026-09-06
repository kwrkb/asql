package ui

import (
	"testing"
)

func TestWordAtCursor(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		row        int
		runeCol    int
		wantPrefix string
		wantStart  int
	}{
		{"end of word", "SELECT us", 0, 9, "us", 7},
		{"middle of word", "SELECT users FROM", 0, 11, "user", 7},
		{"line start", "hello", 0, 3, "hel", 0},
		{"after space", "SELECT ", 0, 7, "", 7},
		{"multiline second row", "SELECT *\nFROM us", 1, 7, "us", 14},
		{"with underscore", "user_name", 0, 9, "user_name", 0},
		{"with digits", "table1", 0, 6, "table1", 0},
		{"dot prefix", "users.na", 0, 8, "users.na", 0},
		{"after dot", "users.", 0, 6, "users.", 0},
		{"empty text", "", 0, 0, "", 0},
		{"empty line", "\n", 0, 0, "", 0},
		// runeCol counts runes, startPos counts bytes: 16 runes / 20 bytes,
		// the "ci" prefix starts at byte 18.
		{"wide runes before word", "name='日本' AND ci", 0, 16, "ci", 18},
		{"cursor right after wide runes", "日本", 0, 2, "", 6},
		{"runeCol clamped to line length", "日本", 0, 99, "", 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, startPos := wordAtCursor(tt.text, tt.row, tt.runeCol)
			if prefix != tt.wantPrefix {
				t.Errorf("prefix = %q, want %q", prefix, tt.wantPrefix)
			}
			if startPos != tt.wantStart {
				t.Errorf("startPos = %d, want %d", startPos, tt.wantStart)
			}
		})
	}
}

func TestDetectContext(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		startPos int
		want     completionContext
	}{
		{"after FROM", "SELECT * FROM ", 14, contextTable},
		{"after JOIN", "SELECT * FROM t JOIN ", 21, contextTable},
		{"after INTO", "INSERT INTO ", 12, contextTable},
		{"after UPDATE", "UPDATE ", 7, contextTable},
		{"after SELECT", "SELECT ", 7, contextColumn},
		{"after WHERE", "SELECT * FROM t WHERE ", 22, contextColumn},
		{"after AND", "WHERE a = 1 AND ", 16, contextColumn},
		{"after OR", "WHERE a = 1 OR ", 15, contextColumn},
		{"after SET", "UPDATE t SET ", 13, contextColumn},
		{"after BY", "ORDER BY ", 9, contextColumn},
		{"after ON", "JOIN t2 ON ", 11, contextColumn},
		{"unknown context", "VALUES (", 8, contextUnknown},
		{"empty text", "", 0, contextUnknown},
		{"after comma in SELECT", "SELECT a, ", 10, contextColumn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectContext(tt.text, tt.startPos)
			if got != tt.want {
				t.Errorf("detectContext(%q, %d) = %d, want %d", tt.text, tt.startPos, got, tt.want)
			}
		})
	}
}

func TestFilterByPrefix(t *testing.T) {
	items := []string{"users", "user_roles", "products", "orders"}

	tests := []struct {
		name   string
		prefix string
		want   int
	}{
		{"match two", "user", 2},
		{"match one", "prod", 1},
		{"no match", "xyz", 0},
		{"empty prefix", "", 4},
		{"case insensitive", "USER", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterByPrefix(items, tt.prefix)
			if len(got) != tt.want {
				t.Errorf("filterByPrefix(%v, %q) returned %d items, want %d", items, tt.prefix, len(got), tt.want)
			}
		})
	}
}

func TestDetectTableFromContext(t *testing.T) {
	tables := []string{"users", "orders", "products"}

	tests := []struct {
		name   string
		text   string
		prefix string
		want   string
	}{
		{"dot prefix", "SELECT users.", "users.na", "users"},
		{"FROM clause", "SELECT * FROM users WHERE ", "", "users"},
		{"JOIN clause", "SELECT * FROM orders JOIN users ON ", "", "users"},
		{"unknown table", "SELECT foo.", "foo.bar", ""},
		{"no context", "SELECT ", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectTableFromContext(tt.text, tt.prefix, tables)
			if got != tt.want {
				t.Errorf("detectTableFromContext(%q, %q, ...) = %q, want %q", tt.text, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestDedup(t *testing.T) {
	input := []string{"id", "name", "ID", "email", "Name"}
	got := dedup(input)
	if len(got) != 3 {
		t.Errorf("dedup returned %d items, want 3: %v", len(got), got)
	}
}
