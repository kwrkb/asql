package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var (
	testHeaders = []string{"id", "name", "email"}
	testRows    = [][]string{
		{"1", "Alice", "alice@example.com"},
		{"2", "Bob", "bob@example.com"},
	}
)

func TestFormatCSV(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
		want    string
	}{
		{
			name:    "basic",
			headers: testHeaders,
			rows:    testRows,
			want:    "id,name,email\n1,Alice,alice@example.com\n2,Bob,bob@example.com\n",
		},
		{
			name:    "empty rows",
			headers: []string{"col"},
			rows:    [][]string{},
			want:    "col\n",
		},
		{
			name:    "value with comma",
			headers: []string{"data"},
			rows:    [][]string{{"hello, world"}},
			want:    "data\n\"hello, world\"\n",
		},
		{
			name:    "value with quotes",
			headers: []string{"data"},
			rows:    [][]string{{`say "hi"`}},
			want:    "data\n\"say \"\"hi\"\"\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatCSV(tt.headers, tt.rows)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestFormatJSON(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
	}{
		{
			name:    "basic",
			headers: testHeaders,
			rows:    testRows,
		},
		{
			name:    "empty rows",
			headers: []string{"col"},
			rows:    [][]string{},
		},
		{
			name:    "short row padded",
			headers: []string{"a", "b"},
			rows:    [][]string{{"1"}},
		},
		{
			name:    "duplicate column names",
			headers: []string{"id", "name", "id"},
			rows:    [][]string{{"1", "Alice", "10"}},
		},
		{
			name:    "generated suffix collides with an existing header",
			headers: []string{"id", "id", "id_2"},
			rows:    [][]string{{"left", "right", "original"}},
		},
		{
			name:    "several existing suffixes taken",
			headers: []string{"v", "v", "v", "v_1", "v_3"},
			rows:    [][]string{{"a", "b", "c", "d", "e"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatJSON(tt.headers, tt.rows)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate it's valid JSON
			var records []map[string]string
			if err := json.Unmarshal([]byte(got), &records); err != nil {
				t.Fatalf("invalid JSON output: %v\n%s", err, got)
			}

			if len(records) != len(tt.rows) {
				t.Errorf("got %d records, want %d", len(records), len(tt.rows))
			}

			// Check record count matches row count and all keys exist
			if len(records) > 0 {
				if len(records[0]) != len(tt.headers) {
					t.Errorf("got %d keys, want %d (all columns should be preserved)", len(records[0]), len(tt.headers))
				}
				// Every cell must survive: a duplicate key would silently drop one.
				want := make(map[string]int)
				for _, v := range tt.rows[0] {
					want[v]++
				}
				have := make(map[string]int)
				for _, v := range records[0] {
					have[v]++
				}
				for v, n := range want {
					if have[v] != n {
						t.Errorf("value %q appears %d time(s), want %d\n%s", v, have[v], n, got)
					}
				}
			}
		})
	}
}

func TestDeduplicateHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		want    []string
	}{
		{
			name:    "all unique",
			headers: []string{"id", "name"},
			want:    []string{"id", "name"},
		},
		{
			name:    "plain duplicates",
			headers: []string{"id", "name", "id"},
			want:    []string{"id_1", "name", "id_2"},
		},
		{
			name:    "suffix collides with an existing header",
			headers: []string{"id", "id", "id_2"},
			want:    []string{"id_1", "id_3", "id_2"},
		},
		{
			name:    "several existing suffixes taken",
			headers: []string{"v", "v", "v", "v_1", "v_3"},
			want:    []string{"v_2", "v_4", "v_5", "v_1", "v_3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateHeaders(tt.headers)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			seen := make(map[string]bool, len(got))
			for _, k := range got {
				if seen[k] {
					t.Errorf("duplicate key %q in %v", k, got)
				}
				seen[k] = true
			}
		})
	}
}

func TestFormatMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		headers []string
		rows    [][]string
		check   func(t *testing.T, got string)
	}{
		{
			name:    "basic",
			headers: testHeaders,
			rows:    testRows,
			check: func(t *testing.T, got string) {
				lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
				if len(lines) != 4 { // header + separator + 2 rows
					t.Fatalf("got %d lines, want 4", len(lines))
				}
				if !strings.Contains(lines[0], "| id |") {
					t.Errorf("header line missing 'id': %s", lines[0])
				}
				if !strings.Contains(lines[1], "| --- |") {
					t.Errorf("separator line wrong: %s", lines[1])
				}
			},
		},
		{
			name:    "pipe escape",
			headers: []string{"data"},
			rows:    [][]string{{"a|b"}},
			check: func(t *testing.T, got string) {
				if !strings.Contains(got, `a\|b`) {
					t.Errorf("pipe not escaped: %s", got)
				}
			},
		},
		{
			name:    "empty rows",
			headers: []string{"col"},
			rows:    [][]string{},
			check: func(t *testing.T, got string) {
				lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
				if len(lines) != 2 { // header + separator only
					t.Fatalf("got %d lines, want 2", len(lines))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMarkdown(tt.headers, tt.rows)
			tt.check(t, got)
		})
	}
}

func TestSaveCSVFile(t *testing.T) {
	// Use temp dir to avoid polluting working directory
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	filename, err := SaveCSVFile(testHeaders, testRows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(filename, "result_") || !strings.HasSuffix(filename, ".csv") {
		t.Errorf("unexpected filename format: %s", filename)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, filename))
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}

	if !strings.Contains(string(content), "id,name,email") {
		t.Errorf("file content missing headers: %s", string(content))
	}
}
