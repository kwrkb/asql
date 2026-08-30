package ui

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

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
	updated, cmd := m.Update(runeMsg("d"))
	result := updated.(model)
	if result.mode != statsMode {
		t.Errorf("mode = %v, want statsMode", result.mode)
	}
	if !result.statsSt.loading {
		t.Error("loading should be true while stats are being computed")
	}
	// Simulate async stats computation completing
	if cmd != nil {
		msg := cmd()
		updated, _ = result.Update(msg)
		result = updated.(model)
	}
	if result.statsSt.loading {
		t.Error("loading should be false after stats are computed")
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

// Column widths were measured in bytes while fmt's "%-Ns" pads by rune count,
// so a single wide-character column name skewed the whole table.
func TestStats_RenderOverlayAlignsMultibyteColumns(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.lastResult = db.QueryResult{
		Columns:     []string{"日本語の列名", "id"},
		ColumnTypes: []string{"TEXT", "INTEGER"},
		Rows: [][]string{
			{"日本語のとても長い値です", "1"},
			{"あ", "2"},
			{"い", "9"},
		},
	}
	m.mode = statsMode
	m.statsSt.stats = computeColumnStats(m.lastResult)
	m.statsSt.cursor = 1 // the numeric column, so its histogram renders too

	rendered := m.renderWithStatsOverlay("background")
	if !utf8.ValidString(rendered) {
		t.Error("stats overlay emitted invalid UTF-8")
	}

	lines := strings.Split(rendered, "\n")
	col := func(needle string) int {
		for _, ln := range lines {
			plain := ansi.Strip(ln)
			if idx := strings.Index(plain, needle); idx >= 0 {
				return ansi.StringWidth(plain[:idx])
			}
		}
		t.Fatalf("no rendered line contains %q", needle)
		return -1
	}

	// The type column must start at the same cell on the multi-byte row and the
	// ASCII row.
	if got, want := col("TEXT"), col("INTEGER"); got != want {
		t.Errorf("type column starts at cell %d on the multi-byte row and %d on the ASCII row", got, want)
	}

	// The histogram indent is written as nameW+typeW+7 rather than derived from
	// the row, so it drifts too if the widths stop being cell counts.
	wantIndent := col("INTEGER") + ansi.StringWidth("INTEGER") + 2
	if got := col("█"); got != wantIndent {
		t.Errorf("histogram bars start at cell %d, want %d (under the NULL%% column)", got, wantIndent)
	}
}

func TestComputeColumnStats_NullCountUsesKinds(t *testing.T) {
	// Two cells display as "NULL": one is a real SQL NULL, one is the four
	// characters. Only the first should count toward the NULL rate.
	result := db.QueryResult{
		Columns: []string{"name"},
		Rows:    [][]string{{"NULL"}, {"NULL"}, {"alice"}, {"bob"}},
		Kinds: [][]db.Kind{
			{db.KindNull},
			{db.KindText},
			{db.KindText},
			{db.KindText},
		},
	}

	stats := computeColumnStats(result)
	if stats[0].NullCnt != 1 {
		t.Errorf("NullCnt = %d, want 1 (the literal text NULL is not a NULL)", stats[0].NullCnt)
	}
	if stats[0].NullRate != 0.25 {
		t.Errorf("NullRate = %v, want 0.25", stats[0].NullRate)
	}
	// The literal "NULL" text is a distinct value alongside alice and bob.
	if stats[0].Distinct != 3 {
		t.Errorf("Distinct = %d, want 3", stats[0].Distinct)
	}
}

func TestComputeColumnStats_NullCountFallsBackWithoutKinds(t *testing.T) {
	result := db.QueryResult{
		Columns: []string{"name"},
		Rows:    [][]string{{"NULL"}, {"NULL"}, {"alice"}, {"bob"}},
	}

	stats := computeColumnStats(result)
	if stats[0].NullCnt != 2 {
		t.Errorf("NullCnt = %d, want 2 (string-sentinel fallback)", stats[0].NullCnt)
	}
}

func TestComputeColumnStats_MinMaxOrdersLiteralNullAsText(t *testing.T) {
	// With kinds available the literal text "NULL" is not a NULL, so it must
	// also be ordered as text: lexically "NULL" < "zzz".
	result := db.QueryResult{
		Columns: []string{"name"},
		Rows:    [][]string{{"NULL"}, {"zzz"}},
		Kinds:   [][]db.Kind{{db.KindText}, {db.KindText}},
	}

	stats := computeColumnStats(result)
	if stats[0].NullCnt != 0 {
		t.Fatalf("NullCnt = %d, want 0", stats[0].NullCnt)
	}
	if stats[0].Min != "NULL" || stats[0].Max != "zzz" {
		t.Errorf("Min/Max = %q/%q, want NULL/zzz", stats[0].Min, stats[0].Max)
	}
}

func TestCompareValues_IgnoresTheNullSentinelRule(t *testing.T) {
	if compareValues("NULL", "zzz") >= 0 {
		t.Error(`compareValues("NULL", "zzz") should order NULL first, as plain text`)
	}
	if smartCompare("NULL", "zzz") <= 0 {
		t.Error(`smartCompare("NULL", "zzz") should still order the sentinel last`)
	}
	// Numeric comparison is unchanged in both.
	if compareValues("9", "10") >= 0 || smartCompare("9", "10") >= 0 {
		t.Error("9 should compare below 10 numerically")
	}
}

// A modal is clamped to the real screen width, so on a terminal narrower than
// the border-plus-padding overhead the derived content width went negative and
// strings.Repeat panicked.
func TestStats_RenderOverlayNarrowTerminal(t *testing.T) {
	for w := 1; w <= 10; w++ {
		t.Run(fmt.Sprintf("width%d", w), func(t *testing.T) {
			m := newStatsModel()
			m.width, m.height = w, 24
			m.mode = statsMode
			m.statsSt.stats = computeColumnStats(m.lastResult)

			// The loading branch renders its own modal from calcModalWidth too.
			m.statsSt.loading = true
			_ = m.renderWithStatsOverlay("background")

			m.statsSt.loading = false
			_ = m.renderWithStatsOverlay("background")
		})
	}
}

// The header line and the data rows built their column offsets independently,
// so every column from Column onward sat one cell left of the values beneath
// it: a data row spends cursor(2) + space(1) before the name, the header spent
// only two spaces.
func TestStats_RenderOverlayAlignsHeaderWithRows(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 100, 30
	m.lastResult = db.QueryResult{
		Columns:     []string{"name", "id"},
		ColumnTypes: []string{"TEXT", "INTEGER"},
		Rows:        [][]string{{"alice", "1"}, {"bob", "2"}},
	}
	m.mode = statsMode
	m.statsSt.stats = computeColumnStats(m.lastResult)

	lines := strings.Split(m.renderWithStatsOverlay("background"), "\n")
	// line returns the sole rendered row containing marker, stripped of styling.
	// "Distinct" only appears in the header and "TEXT" only in the row for the
	// name column, so each picks out one line.
	line := func(marker string) string {
		for _, ln := range lines {
			if plain := ansi.Strip(ln); strings.Contains(plain, marker) {
				return plain
			}
		}
		t.Fatalf("no rendered line contains %q", marker)
		return ""
	}
	startOf := func(text, needle string) int {
		idx := strings.Index(text, needle)
		if idx < 0 {
			t.Fatalf("%q not found in %q", needle, text)
		}
		return ansi.StringWidth(text[:idx])
	}

	header, row := line("Distinct"), line("TEXT")
	if got, want := startOf(header, "Column"), startOf(row, "name"); got != want {
		t.Errorf("header Column starts at cell %d, the column name at %d", got, want)
	}
	if got, want := startOf(header, "Type"), startOf(row, "TEXT"); got != want {
		t.Errorf("header Type starts at cell %d, the type value at %d", got, want)
	}
	// NULL% and its values are right-aligned in the same 6-cell field, so their
	// ends are what must line up.
	if got, want := startOf(header, "NULL%")+len("NULL%"), startOf(row, "0.0%")+len("0.0%"); got != want {
		t.Errorf("header NULL%% ends at cell %d, the value at %d", got, want)
	}
}
