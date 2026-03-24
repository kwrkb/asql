package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newInsertModel() *model {
	m := newTestModel()
	m.mode = insertMode
	m.textarea.Focus()
	return m
}

func TestInsert_EscReturnsNormal(t *testing.T) {
	m := newInsertModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(model)
	if result.mode != normalMode {
		t.Errorf("mode = %v, want normalMode", result.mode)
	}
}

func TestInsert_CtrlLClearsEditor(t *testing.T) {
	m := newInsertModel()
	m.textarea.SetValue("SELECT 1")
	m.historyIdx = 2
	m.historyDraft = "old draft"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	result := updated.(model)
	if result.textarea.Value() != "" {
		t.Errorf("textarea.Value() = %q, want empty", result.textarea.Value())
	}
	if result.historyIdx != -1 {
		t.Errorf("historyIdx = %d, want -1", result.historyIdx)
	}
	if result.historyDraft != "" {
		t.Errorf("historyDraft = %q, want empty", result.historyDraft)
	}
}

func TestInsert_CtrlPNavigatesHistoryBack(t *testing.T) {
	m := newInsertModel()
	m.queryHistory = []string{"SELECT 1", "SELECT 2", "SELECT 3"}
	m.historyIdx = -1
	m.textarea.SetValue("current")

	// First Ctrl+P: jump to last entry and save draft
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	result := updated.(model)
	if result.historyIdx != 2 {
		t.Errorf("historyIdx = %d, want 2", result.historyIdx)
	}
	if result.textarea.Value() != "SELECT 3" {
		t.Errorf("value = %q, want %q", result.textarea.Value(), "SELECT 3")
	}
	if result.historyDraft != "current" {
		t.Errorf("historyDraft = %q, want %q", result.historyDraft, "current")
	}

	// Second Ctrl+P: go further back
	*m = result
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	result = updated.(model)
	if result.historyIdx != 1 {
		t.Errorf("historyIdx = %d, want 1", result.historyIdx)
	}
	if result.textarea.Value() != "SELECT 2" {
		t.Errorf("value = %q, want %q", result.textarea.Value(), "SELECT 2")
	}

	// Ctrl+P at boundary stays at 0
	*m = result
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP}) // now at 0
	result = updated.(model)
	*m = result
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP}) // should stay at 0
	result = updated.(model)
	if result.historyIdx != 0 {
		t.Errorf("historyIdx at boundary = %d, want 0", result.historyIdx)
	}
}

func TestInsert_CtrlNNavigatesHistoryForward(t *testing.T) {
	m := newInsertModel()
	m.queryHistory = []string{"SELECT 1", "SELECT 2"}
	m.historyIdx = 0
	m.historyDraft = "my draft"
	m.textarea.SetValue("SELECT 1")

	// Ctrl+N: move forward
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	result := updated.(model)
	if result.historyIdx != 1 {
		t.Errorf("historyIdx = %d, want 1", result.historyIdx)
	}
	if result.textarea.Value() != "SELECT 2" {
		t.Errorf("value = %q, want %q", result.textarea.Value(), "SELECT 2")
	}

	// Ctrl+N at end: return to draft
	*m = result
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	result = updated.(model)
	if result.historyIdx != -1 {
		t.Errorf("historyIdx = %d, want -1 (draft)", result.historyIdx)
	}
	if result.textarea.Value() != "my draft" {
		t.Errorf("value = %q, want %q", result.textarea.Value(), "my draft")
	}
}

func TestInsert_CtrlPEmptyHistoryNoop(t *testing.T) {
	m := newInsertModel()
	m.queryHistory = nil
	m.textarea.SetValue("hello")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	result := updated.(model)
	if result.textarea.Value() != "hello" {
		t.Errorf("value changed with empty history: %q", result.textarea.Value())
	}
}

func TestInsert_CtrlNWithoutHistoryNoop(t *testing.T) {
	m := newInsertModel()
	m.historyIdx = -1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	result := updated.(model)
	if result.historyIdx != -1 {
		t.Errorf("historyIdx = %d, want -1 (no change)", result.historyIdx)
	}
}

func TestInsert_CtrlJReturnsCmd(t *testing.T) {
	m := newInsertModel()
	m.textarea.SetValue("SELECT 1")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if cmd == nil {
		t.Error("CtrlJ should return a non-nil command (query execution)")
	}
}

func TestInsert_CompletionTabCyclesForward(t *testing.T) {
	m := newInsertModel()
	m.completion.active = true
	m.completion.items = []string{"users", "posts", "comments"}
	m.completion.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	result := updated.(model)
	if result.completion.cursor != 1 {
		t.Errorf("cursor = %d, want 1", result.completion.cursor)
	}

	// Wrap around
	*m = result
	m.completion.cursor = 2
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	result = updated.(model)
	if result.completion.cursor != 0 {
		t.Errorf("cursor after wrap = %d, want 0", result.completion.cursor)
	}
}

func TestInsert_CompletionEscCloses(t *testing.T) {
	m := newInsertModel()
	m.completion.active = true
	m.completion.items = []string{"users"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(model)
	if result.completion.active {
		t.Error("completion should be closed after Esc")
	}
	// Esc in completion closes popup; mode stays insertMode
	if result.mode != insertMode {
		t.Errorf("mode = %v, want insertMode (Esc closes completion only)", result.mode)
	}
}

func TestInsert_CompletionCtrlNMovesDown(t *testing.T) {
	m := newInsertModel()
	m.completion.active = true
	m.completion.items = []string{"a", "b", "c"}
	m.completion.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	result := updated.(model)
	if result.completion.cursor != 1 {
		t.Errorf("cursor = %d, want 1", result.completion.cursor)
	}
}

func TestInsert_CompletionCtrlPMovesUp(t *testing.T) {
	m := newInsertModel()
	m.completion.active = true
	m.completion.items = []string{"a", "b", "c"}
	m.completion.cursor = 2

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	result := updated.(model)
	if result.completion.cursor != 1 {
		t.Errorf("cursor = %d, want 1", result.completion.cursor)
	}
}
