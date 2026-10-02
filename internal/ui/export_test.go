package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/db"
)

func TestExport_NavigationJK(t *testing.T) {
	m := newTestModel()
	m.mode = exportMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
	}

	// j moves down
	result, _ := m.updateExport(runeMsg("j"))
	rm := result.(model)
	if rm.exportSt.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", rm.exportSt.cursor)
	}

	// j again
	m.exportSt.cursor = 1
	result, _ = m.updateExport(runeMsg("j"))
	rm = result.(model)
	if rm.exportSt.cursor != 2 {
		t.Errorf("expected cursor=2, got %d", rm.exportSt.cursor)
	}

	// k moves up
	m.exportSt.cursor = 2
	result, _ = m.updateExport(runeMsg("k"))
	rm = result.(model)
	if rm.exportSt.cursor != 1 {
		t.Errorf("expected cursor=1 after k, got %d", rm.exportSt.cursor)
	}
}

func TestExport_NavigationArrows(t *testing.T) {
	m := newTestModel()
	m.mode = exportMode

	result, _ := m.updateExport(tea.KeyMsg{Type: tea.KeyDown})
	rm := result.(model)
	if rm.exportSt.cursor != 1 {
		t.Errorf("expected cursor=1, got %d", rm.exportSt.cursor)
	}

	m.exportSt.cursor = 1
	result, _ = m.updateExport(tea.KeyMsg{Type: tea.KeyUp})
	rm = result.(model)
	if rm.exportSt.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", rm.exportSt.cursor)
	}
}

func TestExport_BoundaryTop(t *testing.T) {
	m := newTestModel()
	m.mode = exportMode
	m.exportSt.cursor = 0

	result, _ := m.updateExport(runeMsg("k"))
	rm := result.(model)
	if rm.exportSt.cursor != 0 {
		t.Errorf("expected cursor=0 at boundary, got %d", rm.exportSt.cursor)
	}
}

func TestExport_BoundaryBottom(t *testing.T) {
	m := newTestModel()
	m.mode = exportMode
	m.exportSt.cursor = len(exportOptions) - 1

	result, _ := m.updateExport(runeMsg("j"))
	rm := result.(model)
	if rm.exportSt.cursor != len(exportOptions)-1 {
		t.Errorf("expected cursor=%d at boundary, got %d", len(exportOptions)-1, rm.exportSt.cursor)
	}
}

func TestExport_EscReturnsToNormal(t *testing.T) {
	m := newTestModel()
	m.mode = exportMode

	result, _ := m.updateExport(tea.KeyMsg{Type: tea.KeyEsc})
	rm := result.(model)

	if rm.mode != normalMode {
		t.Errorf("expected normalMode, got %q", rm.mode)
	}
}

// Issue #90 item 5: export wrote m.lastResult as the query returned it, not
// what the screen showed — an `s` sort was ignored, and so was a focused
// pinned pane in compare mode.
func TestExportSource_FollowsSortAndFocusedPane(t *testing.T) {
	m := newTestModel()
	m.applyResult(db.QueryResult{
		Columns: []string{"n"},
		Rows:    [][]string{{"2"}, {"10"}, {"1"}},
	})
	m.colCursor = 0
	m.toggleSort() // ascending

	headers, rows := m.exportSource()
	if len(headers) != 1 || headers[0] != "n" {
		t.Fatalf("headers = %v", headers)
	}
	if got := []string{rows[0][0], rows[1][0], rows[2][0]}; got[0] != "1" || got[1] != "2" || got[2] != "10" {
		t.Errorf("rows = %v, want the displayed ascending order 1, 2, 10", got)
	}

	// Pin it, then run a query whose result has other columns.
	m.pinned = m.pinCurrentResult()
	m.applyResult(db.QueryResult{Columns: []string{"other"}, Rows: [][]string{{"x"}}})
	m.focusComparePane(0)
	headers, rows = m.exportSource()
	if headers[0] != "n" || rows[0][0] != "1" {
		t.Errorf("with the pinned pane focused, got headers %v rows %v; want the pinned, sorted result", headers, rows)
	}

	m.focusComparePane(1)
	if headers, _ = m.exportSource(); headers[0] != "other" {
		t.Errorf("with the active pane focused, headers = %v, want [other]", headers)
	}
}

// An empty result exports its header alone: the "(no rows)" sentinel shown
// on screen is not data.
func TestExportSource_EmptyResultHasNoSentinel(t *testing.T) {
	m := newTestModel()
	m.applyResult(db.QueryResult{Columns: []string{"id"}, Rows: [][]string{}})
	if _, rows := m.exportSource(); len(rows) != 0 {
		t.Errorf("rows = %v, want none", rows)
	}
}
