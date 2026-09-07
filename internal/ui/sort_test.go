package ui

import (
	"strconv"
	"strings"
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

// RFC3339Nano trims trailing zeros and drops the fraction entirely on a whole
// second, so the strings a timestamp column yields do not compare lexically in
// chronological order: '.' (0x2E) sorts before 'Z' (0x5A), putting the later
// value first. compareValues has to read them as instants.
func TestCompareValuesOrdersTimestampsChronologically(t *testing.T) {
	const (
		whole    = "2024-01-15T12:00:00Z"
		fraction = "2024-01-15T12:00:00.1Z"
	)
	if strings.Compare(whole, fraction) < 0 {
		t.Fatal("premise gone: these two now compare lexically in the right order")
	}
	if got := compareValues(whole, fraction); got >= 0 {
		t.Errorf("compareValues(%q, %q) = %d, want < 0", whole, fraction, got)
	}
	if got := compareValues(fraction, whole); got <= 0 {
		t.Errorf("compareValues(%q, %q) = %d, want > 0", fraction, whole, got)
	}
	if got := compareValues(whole, whole); got != 0 {
		t.Errorf("compareValues(%q, %q) = %d, want 0", whole, whole, got)
	}

	rows := [][]string{{fraction}, {whole}}
	sorted := sortedRows(rows, 0, sortAsc)
	if sorted[0][0] != whole {
		t.Errorf("ascending sort put %q first, want %q", sorted[0][0], whole)
	}

	// A column that only looks date-shaped must keep falling through to the
	// lexical path rather than comparing as equal.
	if got := compareValues("2024-ab-cd", "2024-ab-ce"); got >= 0 {
		t.Errorf("compareValues on unparseable date-shaped text = %d, want < 0", got)
	}
}

// Integers past 2^53 lose their low bits in float64, which is what the
// comparison used to go through. The database's int64 and the display string
// both kept the difference; only the ordering lost it.
func TestCompareValuesOrdersIntegersExactly(t *testing.T) {
	const (
		lo = "9007199254740992" // 2^53
		hi = "9007199254740993" // 2^53 + 1, not representable as float64
	)
	lf, _ := strconv.ParseFloat(lo, 64)
	hf, _ := strconv.ParseFloat(hi, 64)
	if lf != hf {
		t.Fatal("premise gone: these two now differ as float64")
	}

	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"int64 past 2^53", hi, lo, 1},
		{"int64 past 2^53, reversed", lo, hi, -1},
		{"int64 extremes", "-9223372036854775808", "9223372036854775807", -1},
		{"uint64 past int64 (BIGINT UNSIGNED)", "18446744073709551615", "18446744073709551614", 1},
		{"decimal with more digits than a double keeps", "0.30000000000000000001", "0.3", 1},
		{"integer against a fraction that rounds onto it", hi, "9007199254740992.5", 1},
		{"equal integers", hi, hi, 0},
		{"same value, integer and decimal form", "42", "42.0", 0},
		{"ordinary floats still order as floats", "1.5", "2.25", -1},
		{"NaN ties without an exact form", "NaN", "NaN", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareValues(tt.a, tt.b)
			if (tt.want < 0 && got >= 0) || (tt.want > 0 && got <= 0) || (tt.want == 0 && got != 0) {
				t.Errorf("compareValues(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.want)
			}
		})
	}

	rows := [][]string{{hi}, {lo}}
	if asc := sortedRows(rows, 0, sortAsc); asc[0][0] != lo {
		t.Errorf("ascending sort put %q first, want %q", asc[0][0], lo)
	}
	if desc := sortedRows(rows, 0, sortDesc); desc[0][0] != hi {
		t.Errorf("descending sort put %q first, want %q", desc[0][0], hi)
	}
}
