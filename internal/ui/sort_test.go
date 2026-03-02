package ui

import (
	"testing"
)

func TestSmartCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int // -1, 0, 1
	}{
		{"numeric ascending", "1", "2", -1},
		{"numeric descending", "10", "2", 1},
		{"numeric equal", "5", "5", 0},
		{"float comparison", "1.5", "2.3", -1},
		{"string comparison", "alice", "bob", -1},
		{"string equal", "same", "same", 0},
		{"null vs value", "NULL", "1", 1},
		{"value vs null", "1", "NULL", -1},
		{"null vs null", "NULL", "NULL", 0},
		{"mixed numeric and string", "abc", "123", 1}, // string > number in string compare
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := smartCompare(tt.a, tt.b)
			if (tt.want < 0 && got >= 0) || (tt.want > 0 && got <= 0) || (tt.want == 0 && got != 0) {
				t.Errorf("smartCompare(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestSortedRows(t *testing.T) {
	rows := [][]string{
		{"3", "charlie"},
		{"1", "alice"},
		{"NULL", "dave"},
		{"2", "bob"},
	}

	t.Run("sort none returns original", func(t *testing.T) {
		result := sortedRows(rows, 0, sortNone)
		if result[0][0] != "3" {
			t.Errorf("expected original order, got %v", result)
		}
	})

	t.Run("sort asc by first column", func(t *testing.T) {
		result := sortedRows(rows, 0, sortAsc)
		expected := []string{"1", "2", "3", "NULL"}
		for i, want := range expected {
			if result[i][0] != want {
				t.Errorf("row %d: expected %q, got %q", i, want, result[i][0])
			}
		}
	})

	t.Run("sort desc by first column", func(t *testing.T) {
		result := sortedRows(rows, 0, sortDesc)
		expected := []string{"3", "2", "1", "NULL"}
		for i, want := range expected {
			if result[i][0] != want {
				t.Errorf("row %d: expected %q, got %q", i, want, result[i][0])
			}
		}
	})

	t.Run("sort asc by second column (string)", func(t *testing.T) {
		result := sortedRows(rows, 1, sortAsc)
		expected := []string{"alice", "bob", "charlie", "dave"}
		for i, want := range expected {
			if result[i][1] != want {
				t.Errorf("row %d: expected %q, got %q", i, want, result[i][1])
			}
		}
	})

	t.Run("does not modify original", func(t *testing.T) {
		_ = sortedRows(rows, 0, sortAsc)
		if rows[0][0] != "3" {
			t.Error("original rows were modified")
		}
	})

	t.Run("empty rows", func(t *testing.T) {
		result := sortedRows([][]string{}, 0, sortAsc)
		if len(result) != 0 {
			t.Errorf("expected empty result, got %v", result)
		}
	})
}

func TestSortIndicator(t *testing.T) {
	if sortIndicator(sortNone) != "" {
		t.Error("sortNone should return empty string")
	}
	if sortIndicator(sortAsc) != " ▲" {
		t.Errorf("sortAsc: got %q", sortIndicator(sortAsc))
	}
	if sortIndicator(sortDesc) != " ▼" {
		t.Errorf("sortDesc: got %q", sortIndicator(sortDesc))
	}
}

func TestToggleSort(t *testing.T) {
	t.Run("same column cycles None->Asc->Desc->None", func(t *testing.T) {
		m := newTestModel()
		m.lastResult.Columns = []string{"id", "name"}
		m.lastResult.Rows = [][]string{{"1", "a"}, {"2", "b"}}
		m.colCursor = 0

		m.toggleSort()
		if m.sortDir != sortAsc {
			t.Errorf("expected Asc, got %d", m.sortDir)
		}

		m.toggleSort()
		if m.sortDir != sortDesc {
			t.Errorf("expected Desc, got %d", m.sortDir)
		}

		m.toggleSort()
		if m.sortDir != sortNone {
			t.Errorf("expected None, got %d", m.sortDir)
		}
	})

	t.Run("different column resets to Asc", func(t *testing.T) {
		m := newTestModel()
		m.lastResult.Columns = []string{"id", "name"}
		m.lastResult.Rows = [][]string{{"1", "a"}, {"2", "b"}}
		m.colCursor = 0
		m.toggleSort() // Asc on col 0

		m.colCursor = 1
		m.toggleSort() // should be Asc on col 1
		if m.sortDir != sortAsc || m.sortCol != 1 {
			t.Errorf("expected Asc on col 1, got dir=%d col=%d", m.sortDir, m.sortCol)
		}
	})
}
