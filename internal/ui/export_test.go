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
