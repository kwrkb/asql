package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/db/sqlite"
)

func newSidebarModel(tables []string) *model {
	m := newTestModel()
	m.mode = sidebarMode
	m.sidebar.open = true
	m.sidebar.tables = tables
	m.sidebar.cursor = 0
	return m
}

func TestSidebar_EscClosesAndReturnsNormal(t *testing.T) {
	m := newSidebarModel([]string{"users", "posts"})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(model)
	if result.mode != normalMode {
		t.Errorf("mode = %v, want normalMode", result.mode)
	}
	if result.sidebar.open {
		t.Error("sidebar.open should be false after Esc")
	}
}

func TestSidebar_TClosesAndReturnsNormal(t *testing.T) {
	m := newSidebarModel([]string{"users"})
	updated, _ := m.Update(runeMsg("t"))
	result := updated.(model)
	if result.mode != normalMode {
		t.Errorf("mode = %v, want normalMode", result.mode)
	}
	if result.sidebar.open {
		t.Error("sidebar.open should be false after 't'")
	}
}

func TestSidebar_JMovesDown(t *testing.T) {
	m := newSidebarModel([]string{"a", "b", "c"})
	updated, _ := m.Update(runeMsg("j"))
	result := updated.(model)
	if result.sidebar.cursor != 1 {
		t.Errorf("cursor = %d, want 1", result.sidebar.cursor)
	}
}

func TestSidebar_KMovesUp(t *testing.T) {
	m := newSidebarModel([]string{"a", "b", "c"})
	m.sidebar.cursor = 2
	updated, _ := m.Update(runeMsg("k"))
	result := updated.(model)
	if result.sidebar.cursor != 1 {
		t.Errorf("cursor = %d, want 1", result.sidebar.cursor)
	}
}

func TestSidebar_DownArrow(t *testing.T) {
	m := newSidebarModel([]string{"a", "b"})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	result := updated.(model)
	if result.sidebar.cursor != 1 {
		t.Errorf("cursor = %d, want 1", result.sidebar.cursor)
	}
}

func TestSidebar_UpArrow(t *testing.T) {
	m := newSidebarModel([]string{"a", "b"})
	m.sidebar.cursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	result := updated.(model)
	if result.sidebar.cursor != 0 {
		t.Errorf("cursor = %d, want 0", result.sidebar.cursor)
	}
}

func TestSidebar_CursorBoundary(t *testing.T) {
	m := newSidebarModel([]string{"a", "b", "c"})

	// k at top stays at 0
	updated, _ := m.Update(runeMsg("k"))
	result := updated.(model)
	if result.sidebar.cursor != 0 {
		t.Errorf("cursor at top after k = %d, want 0", result.sidebar.cursor)
	}

	// j at bottom stays at len-1
	m.sidebar.cursor = 2
	updated, _ = m.Update(runeMsg("j"))
	result = updated.(model)
	if result.sidebar.cursor != 2 {
		t.Errorf("cursor at bottom after j = %d, want 2", result.sidebar.cursor)
	}
}

func TestSidebar_EnterInsertsQuery(t *testing.T) {
	adapter, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	t.Cleanup(func() { adapter.Close() })

	m := NewModel(adapter, "test.db", "test.db", "test", nil, nil, nil)
	m.mode = sidebarMode
	m.sidebar.open = true
	m.sidebar.tables = []string{"users"}
	m.sidebar.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(model)

	if result.mode != insertMode {
		t.Errorf("mode = %v, want insertMode", result.mode)
	}
	if result.sidebar.open {
		t.Error("sidebar.open should be false after Enter")
	}
	val := result.textarea.Value()
	if !strings.Contains(val, "users") {
		t.Errorf("textarea value %q does not contain table name", val)
	}
	if !strings.Contains(val, "SELECT") {
		t.Errorf("textarea value %q does not contain SELECT", val)
	}
}

func TestSidebar_EnterEmptyTablesNoop(t *testing.T) {
	m := newSidebarModel(nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(model)
	if result.mode != sidebarMode {
		t.Errorf("mode = %v, want sidebarMode (no tables, Enter should be noop)", result.mode)
	}
	if !result.sidebar.open {
		t.Error("sidebar.open should remain true when Enter on empty list")
	}
}

func TestSidebar_AltKeyIgnored(t *testing.T) {
	m := newSidebarModel([]string{"a", "b"})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j"), Alt: true})
	result := updated.(model)
	if result.sidebar.cursor != 0 {
		t.Errorf("Alt+j should not move cursor, got %d", result.sidebar.cursor)
	}
}
