package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFilterHistory(t *testing.T) {
	t.Run("empty query returns all in reverse order", func(t *testing.T) {
		m := newTestModel()
		m.queryHistory = []string{"SELECT 1", "SELECT 2", "SELECT 3"}
		m.filterHistory("")

		if len(m.historySearchResults) != 3 {
			t.Fatalf("expected 3 results, got %d", len(m.historySearchResults))
		}
		// Newest first: index 2, 1, 0
		if m.historySearchResults[0] != 2 {
			t.Errorf("expected first result index=2 (newest), got %d", m.historySearchResults[0])
		}
		if m.historySearchResults[2] != 0 {
			t.Errorf("expected last result index=0 (oldest), got %d", m.historySearchResults[2])
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		m := newTestModel()
		m.queryHistory = []string{"SELECT * FROM users", "INSERT INTO orders", "select count(*) from users"}
		m.filterHistory("select")

		if len(m.historySearchResults) != 2 {
			t.Fatalf("expected 2 results, got %d", len(m.historySearchResults))
		}
		// Newest match first
		if m.historySearchResults[0] != 2 {
			t.Errorf("expected first match index=2, got %d", m.historySearchResults[0])
		}
		if m.historySearchResults[1] != 0 {
			t.Errorf("expected second match index=0, got %d", m.historySearchResults[1])
		}
	})

	t.Run("no matches", func(t *testing.T) {
		m := newTestModel()
		m.queryHistory = []string{"SELECT 1", "SELECT 2"}
		m.filterHistory("DELETE")

		if len(m.historySearchResults) != 0 {
			t.Errorf("expected 0 results, got %d", len(m.historySearchResults))
		}
	})

	t.Run("cursor clamped when results shrink", func(t *testing.T) {
		m := newTestModel()
		m.queryHistory = []string{"SELECT 1", "SELECT 2", "INSERT 1"}
		m.historySearchCursor = 2
		m.filterHistory("INSERT")

		if m.historySearchCursor != 0 {
			t.Errorf("expected cursor clamped to 0, got %d", m.historySearchCursor)
		}
	})
}

func TestHistorySearch_EnterSelects(t *testing.T) {
	m := newTestModel()
	m.mode = insertMode
	m.queryHistory = []string{"SELECT 1", "SELECT 2", "SELECT 3"}

	// Enter history search mode
	result, _ := m.enterHistorySearchMode()
	rm := result.(model)

	if rm.mode != historySearchMode {
		t.Fatalf("expected historySearchMode, got %q", rm.mode)
	}

	// Select cursor 0 (newest = "SELECT 3")
	rm.historySearchCursor = 0
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ = rm.updateHistorySearch(msg)
	rm = result.(model)

	if rm.mode != insertMode {
		t.Errorf("expected insertMode after Enter, got %q", rm.mode)
	}
	if rm.textarea.Value() != "SELECT 3" {
		t.Errorf("expected textarea='SELECT 3', got %q", rm.textarea.Value())
	}
}

func TestHistorySearch_EscCancels(t *testing.T) {
	m := newTestModel()
	m.mode = insertMode
	m.queryHistory = []string{"SELECT 1"}
	m.textarea.SetValue("original query")

	result, _ := m.enterHistorySearchMode()
	rm := result.(model)

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	result, _ = rm.updateHistorySearch(msg)
	rm = result.(model)

	if rm.mode != insertMode {
		t.Errorf("expected insertMode after Esc, got %q", rm.mode)
	}
	if rm.textarea.Value() != "original query" {
		t.Errorf("expected textarea unchanged, got %q", rm.textarea.Value())
	}
}

func TestHistorySearch_CtrlREmptyHistory(t *testing.T) {
	m := newTestModel()
	m.mode = insertMode
	m.queryHistory = nil

	result, cmd := m.enterHistorySearchMode()
	rm := result.(model)

	if rm.mode != insertMode {
		t.Errorf("expected insertMode (no-op) with empty history, got %q", rm.mode)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd with empty history")
	}
}

func TestInsert_CtrlLClearsTextarea(t *testing.T) {
	m := newTestModel()
	m.mode = insertMode
	m.textarea.SetValue("SELECT * FROM users")
	m.historyIdx = 2
	m.historyDraft = "some draft"

	msg := tea.KeyMsg{Type: tea.KeyCtrlL}
	result, _ := m.updateInsert(msg)
	rm := result.(model)

	if rm.textarea.Value() != "" {
		t.Errorf("expected empty textarea after Ctrl+L, got %q", rm.textarea.Value())
	}
	if rm.historyIdx != -1 {
		t.Errorf("expected historyIdx=-1 after Ctrl+L, got %d", rm.historyIdx)
	}
	if rm.historyDraft != "" {
		t.Errorf("expected historyDraft='' after Ctrl+L, got %q", rm.historyDraft)
	}
}

func TestHistorySearch_CtrlRCycles(t *testing.T) {
	m := newTestModel()
	m.queryHistory = []string{"SELECT 1", "SELECT 2", "SELECT 3"}

	result, _ := m.enterHistorySearchMode()
	rm := result.(model)

	// Starts at 0
	if rm.historySearchCursor != 0 {
		t.Fatalf("expected cursor=0, got %d", rm.historySearchCursor)
	}

	// Ctrl+R cycles forward
	msg := tea.KeyMsg{Type: tea.KeyCtrlR}
	result, _ = rm.updateHistorySearch(msg)
	rm = result.(model)
	if rm.historySearchCursor != 1 {
		t.Errorf("expected cursor=1 after Ctrl+R, got %d", rm.historySearchCursor)
	}

	// Ctrl+R again
	result, _ = rm.updateHistorySearch(msg)
	rm = result.(model)
	if rm.historySearchCursor != 2 {
		t.Errorf("expected cursor=2 after Ctrl+R, got %d", rm.historySearchCursor)
	}

	// Ctrl+R wraps around
	result, _ = rm.updateHistorySearch(msg)
	rm = result.(model)
	if rm.historySearchCursor != 0 {
		t.Errorf("expected cursor=0 (wrap), got %d", rm.historySearchCursor)
	}
}

func TestHistorySearch_CursorNavigation(t *testing.T) {
	m := newTestModel()
	m.queryHistory = []string{"SELECT 1", "SELECT 2", "SELECT 3"}

	result, _ := m.enterHistorySearchMode()
	rm := result.(model)

	// Ctrl+N moves down
	msg := tea.KeyMsg{Type: tea.KeyCtrlN}
	result, _ = rm.updateHistorySearch(msg)
	rm = result.(model)
	if rm.historySearchCursor != 1 {
		t.Errorf("expected cursor=1 after Ctrl+N, got %d", rm.historySearchCursor)
	}

	// Ctrl+P moves up
	msg = tea.KeyMsg{Type: tea.KeyCtrlP}
	result, _ = rm.updateHistorySearch(msg)
	rm = result.(model)
	if rm.historySearchCursor != 0 {
		t.Errorf("expected cursor=0 after Ctrl+P, got %d", rm.historySearchCursor)
	}

	// Ctrl+P at top stays at 0
	result, _ = rm.updateHistorySearch(msg)
	rm = result.(model)
	if rm.historySearchCursor != 0 {
		t.Errorf("expected cursor=0 at boundary, got %d", rm.historySearchCursor)
	}
}
