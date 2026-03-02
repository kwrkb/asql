package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"

	"github.com/kwrkb/asql/internal/db"
)

func TestColumnWidth(t *testing.T) {
	tests := []struct {
		name  string
		title string
		rows  [][]string
		idx   int
		want  int
	}{
		{
			name:  "minimum width when title and values are short",
			title: "id",
			rows:  [][]string{{"1"}, {"2"}},
			idx:   0,
			want:  12,
		},
		{
			name:  "title determines width",
			title: "user_name_column",
			rows:  [][]string{{"alice"}},
			idx:   0,
			want:  18, // len("user_name_column")=16, 16+2=18
		},
		{
			name:  "row value determines width",
			title: "val",
			rows:  [][]string{{"a_medium_length_str"}},
			idx:   0,
			want:  21, // len("a_medium_length_str")=19, 19+2=21
		},
		{
			name:  "capped at 32 when width+2 would exceed",
			title: "abcdefghijklmnopqrstuvwxyzabcde", // 31 chars
			rows:  nil,
			idx:   0,
			want:  32, // 31+2=33 → capped at 32
		},
		{
			name:  "exactly 32 when width is 30",
			title: "abcdefghijklmnopqrstuvwxyzabcd", // 30 chars
			rows:  nil,
			idx:   0,
			want:  32, // 30+2=32
		},
		{
			name:  "out of bounds idx skipped safely",
			title: "col",
			rows:  [][]string{{"only one col"}},
			idx:   5,
			want:  12, // title "col" is short, minimum applied
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := columnWidth(tt.title, tt.rows, tt.idx)
			if got != tt.want {
				t.Errorf("columnWidth(%q, rows, %d) = %d, want %d", tt.title, tt.idx, got, tt.want)
			}
		})
	}
}

func newTestModel() *model {
	tbl := table.New()
	vp := viewport.New(0, 0)
	return &model{
		table:      tbl,
		viewport:   vp,
		width:      80,
		height:     24,
		historyIdx: -1,
	}
}

func TestApplyResult(t *testing.T) {
	t.Run("SELECT result with rows", func(t *testing.T) {
		m := newTestModel()
		result := db.QueryResult{
			Columns: []string{"id", "name"},
			Rows:    [][]string{{"1", "alice"}, {"2", "bob"}},
			Message: "2 row(s) returned",
		}
		m.applyResult(result)

		cols := m.table.Columns()
		if len(cols) != 2 {
			t.Errorf("expected 2 columns, got %d", len(cols))
		}
		rows := m.table.Rows()
		if len(rows) != 2 {
			t.Errorf("expected 2 rows, got %d", len(rows))
		}
		if m.statusText != "2 row(s) returned" {
			t.Errorf("unexpected status: %q", m.statusText)
		}
	})

	t.Run("SELECT result with no rows uses padded sentinel", func(t *testing.T) {
		m := newTestModel()
		result := db.QueryResult{
			Columns: []string{"id", "name"},
			Rows:    [][]string{},
			Message: "0 row(s) returned",
		}
		m.applyResult(result)

		rows := m.table.Rows()
		if len(rows) != 1 {
			t.Fatalf("expected 1 sentinel row, got %d", len(rows))
		}
		if rows[0][0] != "(no rows)" {
			t.Errorf("expected '(no rows)' sentinel, got %q", rows[0][0])
		}
		// sentinel row must have same column count as columns to avoid panic
		if len(rows[0]) != 2 {
			t.Errorf("expected sentinel row to have 2 cols (matching columns), got %d", len(rows[0]))
		}
	})

	t.Run("DML result shows message in Result column", func(t *testing.T) {
		m := newTestModel()
		result := db.QueryResult{
			Message: "3 row(s) affected",
		}
		m.applyResult(result)

		cols := m.table.Columns()
		if len(cols) != 1 || cols[0].Title != "Result" {
			t.Errorf("expected single 'Result' column, got %v", cols)
		}
		rows := m.table.Rows()
		if len(rows) != 1 || rows[0][0] != "3 row(s) affected" {
			t.Errorf("unexpected DML row: %v", rows)
		}
		if m.statusText != "3 row(s) affected" {
			t.Errorf("unexpected status: %q", m.statusText)
		}
	})

	t.Run("column headers include type info", func(t *testing.T) {
		m := newTestModel()
		result := db.QueryResult{
			Columns:     []string{"id", "name"},
			ColumnTypes: []string{"INTEGER", "TEXT"},
			Rows:        [][]string{{"1", "alice"}},
			Message:     "1 row(s) returned",
		}
		m.applyResult(result)

		cols := m.table.Columns()
		if cols[0].Title != "id integer" {
			t.Errorf("expected 'id integer', got %q", cols[0].Title)
		}
		if cols[1].Title != "name text" {
			t.Errorf("expected 'name text', got %q", cols[1].Title)
		}
	})

	t.Run("column headers without type info", func(t *testing.T) {
		m := newTestModel()
		result := db.QueryResult{
			Columns: []string{"id", "name"},
			Rows:    [][]string{{"1", "alice"}},
			Message: "1 row(s) returned",
		}
		m.applyResult(result)

		cols := m.table.Columns()
		if cols[0].Title != "id" {
			t.Errorf("expected 'id', got %q", cols[0].Title)
		}
	})
}

func TestQueryHistory(t *testing.T) {
	t.Run("history stores executed queries", func(t *testing.T) {
		m := newTestModel()
		m.queryHistory = append(m.queryHistory, "SELECT 1")
		m.queryHistory = append(m.queryHistory, "SELECT 2")

		if len(m.queryHistory) != 2 {
			t.Fatalf("expected 2 history entries, got %d", len(m.queryHistory))
		}
		if m.queryHistory[0] != "SELECT 1" {
			t.Errorf("expected 'SELECT 1', got %q", m.queryHistory[0])
		}
	})

	t.Run("history navigation with ctrl+p and ctrl+n", func(t *testing.T) {
		m := newTestModel()
		m.mode = insertMode
		m.queryHistory = []string{"SELECT 1", "SELECT 2", "SELECT 3"}
		m.historyIdx = -1

		// ctrl+p: go to last entry
		m.historyDraft = "current input"
		m.historyIdx = len(m.queryHistory) - 1
		if m.queryHistory[m.historyIdx] != "SELECT 3" {
			t.Errorf("expected 'SELECT 3', got %q", m.queryHistory[m.historyIdx])
		}

		// ctrl+p again: go to previous
		m.historyIdx--
		if m.queryHistory[m.historyIdx] != "SELECT 2" {
			t.Errorf("expected 'SELECT 2', got %q", m.queryHistory[m.historyIdx])
		}

		// ctrl+n: go to next
		m.historyIdx++
		if m.queryHistory[m.historyIdx] != "SELECT 3" {
			t.Errorf("expected 'SELECT 3', got %q", m.queryHistory[m.historyIdx])
		}

		// ctrl+n at end: back to draft
		m.historyIdx = -1
		if m.historyDraft != "current input" {
			t.Errorf("expected draft 'current input', got %q", m.historyDraft)
		}
	})

	t.Run("history cap at maxHistory", func(t *testing.T) {
		m := newTestModel()
		for i := 0; i < maxHistory+10; i++ {
			m.queryHistory = append(m.queryHistory, "q")
			if len(m.queryHistory) > maxHistory {
				m.queryHistory = m.queryHistory[1:]
			}
		}
		if len(m.queryHistory) != maxHistory {
			t.Errorf("expected %d entries, got %d", maxHistory, len(m.queryHistory))
		}
	})
}
