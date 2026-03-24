package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/db"
)

func TestComputeColumnStats_Basic(t *testing.T) {
	result := db.QueryResult{
		Columns:     []string{"id", "name"},
		ColumnTypes: []string{"INTEGER", "TEXT"},
		Rows: [][]string{
			{"1", "alice"},
			{"2", "NULL"},
			{"3", "alice"},
			{"NULL", "bob"},
		},
	}
	stats := computeColumnStats(result)

	if len(stats) != 2 {
		t.Fatalf("len(stats) = %d, want 2", len(stats))
	}

	t.Run("id column", func(t *testing.T) {
		s := stats[0]
		if s.Name != "id" {
			t.Errorf("Name = %q, want %q", s.Name, "id")
		}
		if s.Type != "INTEGER" {
			t.Errorf("Type = %q, want %q", s.Type, "INTEGER")
		}
		if s.NullCnt != 1 {
			t.Errorf("NullCnt = %d, want 1", s.NullCnt)
		}
		if s.Distinct != 3 {
			t.Errorf("Distinct = %d, want 3", s.Distinct)
		}
		if s.Min != "1" {
			t.Errorf("Min = %q, want %q", s.Min, "1")
		}
		if s.Max != "3" {
			t.Errorf("Max = %q, want %q", s.Max, "3")
		}
	})

	t.Run("name column", func(t *testing.T) {
		s := stats[1]
		if s.NullCnt != 1 {
			t.Errorf("NullCnt = %d, want 1", s.NullCnt)
		}
		if s.Distinct != 2 {
			t.Errorf("Distinct = %d, want 2", s.Distinct)
		}
		if s.Min != "alice" {
			t.Errorf("Min = %q, want %q", s.Min, "alice")
		}
		if s.Max != "bob" {
			t.Errorf("Max = %q, want %q", s.Max, "bob")
		}
	})
}

func TestComputeColumnStats_AllNulls(t *testing.T) {
	result := db.QueryResult{
		Columns: []string{"val"},
		Rows:    [][]string{{"NULL"}, {"NULL"}},
	}
	stats := computeColumnStats(result)
	s := stats[0]
	if s.NullCnt != 2 {
		t.Errorf("NullCnt = %d, want 2", s.NullCnt)
	}
	if s.NullRate != 1.0 {
		t.Errorf("NullRate = %f, want 1.0", s.NullRate)
	}
	if s.Distinct != 0 {
		t.Errorf("Distinct = %d, want 0", s.Distinct)
	}
	if s.Min != "" || s.Max != "" {
		t.Errorf("Min/Max should be empty, got %q/%q", s.Min, s.Max)
	}
}

func TestComputeColumnStats_NoNulls(t *testing.T) {
	result := db.QueryResult{
		Columns: []string{"x"},
		Rows:    [][]string{{"a"}, {"b"}, {"c"}},
	}
	stats := computeColumnStats(result)
	s := stats[0]
	if s.NullCnt != 0 {
		t.Errorf("NullCnt = %d, want 0", s.NullCnt)
	}
	if s.NullRate != 0.0 {
		t.Errorf("NullRate = %f, want 0.0", s.NullRate)
	}
	if s.Distinct != 3 {
		t.Errorf("Distinct = %d, want 3", s.Distinct)
	}
}

func TestComputeColumnStats_EmptyResult(t *testing.T) {
	result := db.QueryResult{
		Columns: []string{"x"},
		Rows:    nil,
	}
	stats := computeColumnStats(result)
	if len(stats) != 1 {
		t.Fatalf("len(stats) = %d, want 1", len(stats))
	}
	s := stats[0]
	if s.NullCnt != 0 || s.Distinct != 0 {
		t.Errorf("empty result: NullCnt=%d Distinct=%d, want 0/0", s.NullCnt, s.Distinct)
	}
}

func TestComputeColumnStats_SingleValue(t *testing.T) {
	result := db.QueryResult{
		Columns: []string{"x"},
		Rows:    [][]string{{"hello"}},
	}
	stats := computeColumnStats(result)
	s := stats[0]
	if s.Distinct != 1 {
		t.Errorf("Distinct = %d, want 1", s.Distinct)
	}
	if s.Min != "hello" || s.Max != "hello" {
		t.Errorf("Min=%q Max=%q, want hello/hello", s.Min, s.Max)
	}
}

func TestComputeColumnStats_NumericMinMax(t *testing.T) {
	result := db.QueryResult{
		Columns:     []string{"price"},
		ColumnTypes: []string{"INTEGER"},
		Rows:        [][]string{{"2"}, {"10"}, {"3"}, {"1"}, {"20"}},
	}
	stats := computeColumnStats(result)
	s := stats[0]
	if s.Min != "1" {
		t.Errorf("Min = %q, want %q (numeric comparison)", s.Min, "1")
	}
	if s.Max != "20" {
		t.Errorf("Max = %q, want %q (numeric comparison)", s.Max, "20")
	}
}

func newStatsModel() *model {
	m := newTestModel()
	m.lastResult = db.QueryResult{
		Columns:     []string{"id", "name", "email"},
		ColumnTypes: []string{"INTEGER", "TEXT", "TEXT"},
		Rows: [][]string{
			{"1", "alice", "a@b.c"},
			{"2", "bob", "NULL"},
		},
	}
	return m
}

func TestStats_DKeyEntersStatsMode(t *testing.T) {
	m := newStatsModel()
	m.mode = normalMode
	updated, _ := m.Update(runeMsg("d"))
	result := updated.(model)
	if result.mode != statsMode {
		t.Errorf("mode = %v, want statsMode", result.mode)
	}
	if len(result.statsSt.stats) != 3 {
		t.Errorf("stats len = %d, want 3", len(result.statsSt.stats))
	}
}

func TestStats_DKeyNoResults(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	updated, _ := m.Update(runeMsg("d"))
	result := updated.(model)
	if result.mode != normalMode {
		t.Errorf("mode = %v, want normalMode (no results)", result.mode)
	}
}

func TestStats_EscCloses(t *testing.T) {
	m := newStatsModel()
	m.mode = statsMode
	m.statsSt.stats = computeColumnStats(m.lastResult)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(model)
	if result.mode != normalMode {
		t.Errorf("mode = %v, want normalMode", result.mode)
	}
}

func TestStats_QCloses(t *testing.T) {
	m := newStatsModel()
	m.mode = statsMode
	m.statsSt.stats = computeColumnStats(m.lastResult)
	updated, _ := m.Update(runeMsg("q"))
	result := updated.(model)
	if result.mode != normalMode {
		t.Errorf("mode = %v, want normalMode", result.mode)
	}
}

func TestStats_NavigationJK(t *testing.T) {
	m := newStatsModel()
	m.mode = statsMode
	m.statsSt.stats = computeColumnStats(m.lastResult)
	m.statsSt.cursor = 0

	t.Run("j moves down", func(t *testing.T) {
		updated, _ := m.Update(runeMsg("j"))
		result := updated.(model)
		if result.statsSt.cursor != 1 {
			t.Errorf("cursor = %d, want 1", result.statsSt.cursor)
		}
	})

	t.Run("k moves up", func(t *testing.T) {
		m.statsSt.cursor = 2
		updated, _ := m.Update(runeMsg("k"))
		result := updated.(model)
		if result.statsSt.cursor != 1 {
			t.Errorf("cursor = %d, want 1", result.statsSt.cursor)
		}
	})

	t.Run("boundary top", func(t *testing.T) {
		m.statsSt.cursor = 0
		updated, _ := m.Update(runeMsg("k"))
		result := updated.(model)
		if result.statsSt.cursor != 0 {
			t.Errorf("cursor = %d, want 0", result.statsSt.cursor)
		}
	})

	t.Run("boundary bottom", func(t *testing.T) {
		m.statsSt.cursor = 2
		updated, _ := m.Update(runeMsg("j"))
		result := updated.(model)
		if result.statsSt.cursor != 2 {
			t.Errorf("cursor = %d, want 2", result.statsSt.cursor)
		}
	})
}

func TestStats_NavigationArrows(t *testing.T) {
	m := newStatsModel()
	m.mode = statsMode
	m.statsSt.stats = computeColumnStats(m.lastResult)
	m.statsSt.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	result := updated.(model)
	if result.statsSt.cursor != 1 {
		t.Errorf("Down: cursor = %d, want 1", result.statsSt.cursor)
	}

	m.statsSt.cursor = 1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	result = updated.(model)
	if result.statsSt.cursor != 0 {
		t.Errorf("Up: cursor = %d, want 0", result.statsSt.cursor)
	}
}

func TestStats_AltKeyIgnored(t *testing.T) {
	m := newStatsModel()
	m.mode = statsMode
	m.statsSt.stats = computeColumnStats(m.lastResult)
	m.statsSt.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j"), Alt: true})
	result := updated.(model)
	if result.statsSt.cursor != 0 {
		t.Errorf("Alt+j should not move cursor, got %d", result.statsSt.cursor)
	}
}

func TestStats_RenderOverlay(t *testing.T) {
	m := newStatsModel()
	m.mode = statsMode
	m.statsSt.stats = computeColumnStats(m.lastResult)
	background := "test background"
	rendered := m.renderWithStatsOverlay(background)
	if rendered == background {
		t.Error("renderWithStatsOverlay should modify the background")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"hello", 1, "…"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
