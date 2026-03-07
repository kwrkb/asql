package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/db"
)

func TestNormal_IEntersInsertMode(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode

	result, _ := m.updateNormal(runeMsg("i"))
	rm := result.(model)

	if rm.mode != insertMode {
		t.Errorf("expected insertMode, got %q", rm.mode)
	}
}

func TestNormal_QQuitsProgram(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode

	_, cmd := m.updateNormal(runeMsg("q"))
	if cmd == nil {
		t.Error("expected non-nil cmd (tea.Quit)")
	}
}

func TestNormal_TOpensSidebar(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.width = 120 // wide enough for sidebar

	result, _ := m.updateNormal(runeMsg("t"))
	rm := result.(model)

	if rm.mode != sidebarMode {
		t.Errorf("expected sidebarMode, got %q", rm.mode)
	}
	if !rm.sidebar.open {
		t.Error("expected sidebar to be open")
	}
}

func TestNormal_TTooNarrow(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.width = 30 // too narrow

	result, _ := m.updateNormal(runeMsg("t"))
	rm := result.(model)

	if rm.mode != normalMode {
		t.Errorf("expected normalMode (too narrow), got %q", rm.mode)
	}
	if rm.sidebar.open {
		t.Error("expected sidebar to remain closed")
	}
}

func TestNormal_EEntersExportMode(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
	}

	result, _ := m.updateNormal(runeMsg("e"))
	rm := result.(model)

	if rm.mode != exportMode {
		t.Errorf("expected exportMode, got %q", rm.mode)
	}
	if rm.exportSt.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", rm.exportSt.cursor)
	}
}

func TestNormal_EWithNoResults(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode

	result, _ := m.updateNormal(runeMsg("e"))
	rm := result.(model)

	if rm.mode != normalMode {
		t.Errorf("expected normalMode when no results, got %q", rm.mode)
	}
	if !rm.statusError {
		t.Error("expected error status")
	}
}

func TestNormal_PEntersProfileMode(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode

	result, _ := m.updateNormal(runeMsg("P"))
	rm := result.(model)

	if rm.mode != profileMode {
		t.Errorf("expected profileMode, got %q", rm.mode)
	}
}

func TestNormal_SEntersSnippetMode(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode

	result, _ := m.updateNormal(runeMsg("S"))
	rm := result.(model)

	if rm.mode != snippetMode {
		t.Errorf("expected snippetMode, got %q", rm.mode)
	}
}

func TestNormal_EnterEntersDetailMode(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "alice"}},
	}
	m.applyResult(m.lastResult)

	result, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(model)

	if rm.mode != detailMode {
		t.Errorf("expected detailMode, got %q", rm.mode)
	}
}

func TestNormal_JKNavigatesRows(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}, {"2"}, {"3"}},
	}
	m.applyResult(m.lastResult)

	// j moves down
	result, _ := m.updateNormal(runeMsg("j"))
	rm := result.(model)
	if rm.table.Cursor() != 1 {
		t.Errorf("expected cursor=1 after j, got %d", rm.table.Cursor())
	}
}

func TestNormal_HLNavigatesColumns(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id", "name", "email"},
		Rows:    [][]string{{"1", "alice", "a@b.com"}},
	}
	m.applyResult(m.lastResult)

	// l moves right
	result, _ := m.updateNormal(runeMsg("l"))
	rm := result.(model)
	if rm.colCursor != 1 {
		t.Errorf("expected colCursor=1 after l, got %d", rm.colCursor)
	}

	// h moves left
	m.colCursor = 1
	result, _ = m.updateNormal(runeMsg("h"))
	rm = result.(model)
	if rm.colCursor != 0 {
		t.Errorf("expected colCursor=0 after h, got %d", rm.colCursor)
	}

	// h at left boundary
	m.colCursor = 0
	result, _ = m.updateNormal(runeMsg("h"))
	rm = result.(model)
	if rm.colCursor != 0 {
		t.Errorf("expected colCursor=0 at boundary, got %d", rm.colCursor)
	}
}

func TestNormal_RReExecuteNoQuery(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.textarea.SetValue("")

	result, _ := m.updateNormal(runeMsg("R"))
	rm := result.(model)

	if rm.statusError != true {
		t.Error("expected error status for empty query")
	}
}

func TestNormal_CComparePinUnpin(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.width = 120
	m.lastResult = db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "alice"}},
	}
	m.applyResult(m.lastResult)

	// c pins
	result, _ := m.updateNormal(runeMsg("c"))
	rm := result.(model)
	if rm.pinned == nil {
		t.Fatal("expected pinned to be set")
	}
	if rm.comparePane != 1 {
		t.Errorf("expected comparePane=1 (right focus), got %d", rm.comparePane)
	}

	// c again unpins
	m2 := rm
	result, _ = m2.updateNormal(runeMsg("c"))
	rm = result.(model)
	if rm.pinned != nil {
		t.Error("expected pinned to be nil after toggle off")
	}
	if rm.comparePane != 0 {
		t.Errorf("expected comparePane=0, got %d", rm.comparePane)
	}
}

func TestNormal_CCompareNoResults(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.width = 120

	result, _ := m.updateNormal(runeMsg("c"))
	rm := result.(model)
	if rm.pinned != nil {
		t.Error("expected no pin with no results")
	}
}

func TestNormal_TabSwitchesComparePane(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.width = 120
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
	}
	m.applyResult(m.lastResult)

	// Pin first
	result, _ := m.updateNormal(runeMsg("c"))
	rm := result.(model)
	if rm.comparePane != 1 {
		t.Fatalf("expected comparePane=1 after pin, got %d", rm.comparePane)
	}

	// Tab switches to left pane
	m2 := rm
	result, _ = m2.updateNormal(tea.KeyMsg{Type: tea.KeyTab})
	rm = result.(model)
	if rm.comparePane != 0 {
		t.Errorf("expected comparePane=0 after Tab, got %d", rm.comparePane)
	}

	// Tab again switches to right pane
	m3 := rm
	result, _ = m3.updateNormal(tea.KeyMsg{Type: tea.KeyTab})
	rm = result.(model)
	if rm.comparePane != 1 {
		t.Errorf("expected comparePane=1 after second Tab, got %d", rm.comparePane)
	}
}

func TestNormal_TabWithoutCompareIsNoop(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode

	result, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyTab})
	rm := result.(model)

	if rm.mode != normalMode {
		t.Errorf("expected normalMode after Tab without compare, got %q", rm.mode)
	}
}

func TestNormal_ArrowKeysNavigation(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "a"}, {"2", "b"}},
	}
	m.applyResult(m.lastResult)

	// Right arrow
	result, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRight})
	rm := result.(model)
	if rm.colCursor != 1 {
		t.Errorf("expected colCursor=1 after right, got %d", rm.colCursor)
	}

	// Left arrow
	m.colCursor = 1
	result, _ = m.updateNormal(tea.KeyMsg{Type: tea.KeyLeft})
	rm = result.(model)
	if rm.colCursor != 0 {
		t.Errorf("expected colCursor=0 after left, got %d", rm.colCursor)
	}
}
