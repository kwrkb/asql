package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/snippet"
)

func newSnippetTestModel(snippets []snippet.Snippet) *model {
	m := newTestModel()
	m.mode = snippetMode
	m.snippetSt.items = snippets
	m.snippetSt.cursor = 0
	m.snippetSt.input = textinput.New()
	m.snippetSt.input.CharLimit = 100
	m.textarea = textarea.New()
	return m
}

func runeMsg(r string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(r)}
}

func TestSnippet_NavigationJK(t *testing.T) {
	snippets := []snippet.Snippet{
		{Name: "a", Query: "SELECT 1"},
		{Name: "b", Query: "SELECT 2"},
		{Name: "c", Query: "SELECT 3"},
	}
	m := newSnippetTestModel(snippets)

	// j moves cursor down
	result, _ := m.updateSnippet(runeMsg("j"))
	rm := result.(model)
	if rm.snippetSt.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", rm.snippetSt.cursor)
	}

	// j again
	m.snippetSt.cursor = 1
	result, _ = m.updateSnippet(runeMsg("j"))
	rm = result.(model)
	if rm.snippetSt.cursor != 2 {
		t.Errorf("expected cursor=2 after j, got %d", rm.snippetSt.cursor)
	}

	// j at bottom boundary
	m.snippetSt.cursor = 2
	result, _ = m.updateSnippet(runeMsg("j"))
	rm = result.(model)
	if rm.snippetSt.cursor != 2 {
		t.Errorf("expected cursor=2 at boundary, got %d", rm.snippetSt.cursor)
	}

	// k moves cursor up
	m.snippetSt.cursor = 2
	result, _ = m.updateSnippet(runeMsg("k"))
	rm = result.(model)
	if rm.snippetSt.cursor != 1 {
		t.Errorf("expected cursor=1 after k, got %d", rm.snippetSt.cursor)
	}

	// k at top boundary
	m.snippetSt.cursor = 0
	result, _ = m.updateSnippet(runeMsg("k"))
	rm = result.(model)
	if rm.snippetSt.cursor != 0 {
		t.Errorf("expected cursor=0 at boundary, got %d", rm.snippetSt.cursor)
	}
}

func TestSnippet_NavigationArrows(t *testing.T) {
	snippets := []snippet.Snippet{
		{Name: "a", Query: "SELECT 1"},
		{Name: "b", Query: "SELECT 2"},
	}
	m := newSnippetTestModel(snippets)

	// Down arrow
	result, _ := m.updateSnippet(tea.KeyMsg{Type: tea.KeyDown})
	rm := result.(model)
	if rm.snippetSt.cursor != 1 {
		t.Errorf("expected cursor=1 after down, got %d", rm.snippetSt.cursor)
	}

	// Up arrow
	m.snippetSt.cursor = 1
	result, _ = m.updateSnippet(tea.KeyMsg{Type: tea.KeyUp})
	rm = result.(model)
	if rm.snippetSt.cursor != 0 {
		t.Errorf("expected cursor=0 after up, got %d", rm.snippetSt.cursor)
	}
}

func TestSnippet_EnterLoadsQuery(t *testing.T) {
	snippets := []snippet.Snippet{
		{Name: "first", Query: "SELECT 1"},
		{Name: "second", Query: "SELECT 2"},
	}
	m := newSnippetTestModel(snippets)
	m.snippetSt.cursor = 1

	result, _ := m.updateSnippet(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(model)

	if rm.mode != insertMode {
		t.Errorf("expected insertMode, got %q", rm.mode)
	}
	if rm.textarea.Value() != "SELECT 2" {
		t.Errorf("expected textarea='SELECT 2', got %q", rm.textarea.Value())
	}
}

func TestSnippet_EnterOnEmptyList(t *testing.T) {
	m := newSnippetTestModel(nil)

	result, _ := m.updateSnippet(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(model)

	// Should remain in snippet mode
	if rm.mode != snippetMode {
		t.Errorf("expected snippetMode on empty list, got %q", rm.mode)
	}
}

func TestSnippet_EscReturnsToNormal(t *testing.T) {
	m := newSnippetTestModel([]snippet.Snippet{{Name: "a", Query: "SELECT 1"}})

	result, _ := m.updateSnippet(tea.KeyMsg{Type: tea.KeyEsc})
	rm := result.(model)

	if rm.mode != normalMode {
		t.Errorf("expected normalMode, got %q", rm.mode)
	}
}

func TestSnippet_AWithEmptyEditor(t *testing.T) {
	m := newSnippetTestModel(nil)
	// textarea is empty by default

	result, _ := m.updateSnippet(runeMsg("a"))
	rm := result.(model)

	if rm.snippetSt.naming {
		t.Error("expected snippetNaming=false when editor is empty")
	}
	if !rm.statusError {
		t.Error("expected error status for empty editor")
	}
}

func TestSnippet_AEntersNaming(t *testing.T) {
	m := newSnippetTestModel(nil)
	m.textarea.SetValue("SELECT 1")

	result, _ := m.updateSnippet(runeMsg("a"))
	rm := result.(model)

	if !rm.snippetSt.naming {
		t.Error("expected snippetNaming=true")
	}
}

func TestSnippetNaming_EscCancels(t *testing.T) {
	m := newSnippetTestModel(nil)
	m.snippetSt.naming = true

	result, _ := m.updateSnippetNaming(tea.KeyMsg{Type: tea.KeyEsc})
	rm := result.(model)

	if rm.snippetSt.naming {
		t.Error("expected snippetNaming=false after Esc")
	}
}

func TestSnippetNaming_EscReturnsToPrevMode(t *testing.T) {
	m := newSnippetTestModel(nil)
	m.snippetSt.naming = true
	m.snippetSt.prevMode = insertMode

	result, _ := m.updateSnippetNaming(tea.KeyMsg{Type: tea.KeyEsc})
	rm := result.(model)

	if rm.mode != insertMode {
		t.Errorf("expected insertMode, got %q", rm.mode)
	}
	if rm.snippetSt.prevMode != "" {
		t.Error("expected snippetPrevMode to be cleared")
	}
}

func TestSnippetNaming_EmptyNameIgnored(t *testing.T) {
	m := newSnippetTestModel(nil)
	m.snippetSt.naming = true
	m.snippetSt.input.SetValue("  ")
	m.textarea.SetValue("SELECT 1")

	result, _ := m.updateSnippetNaming(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(model)

	// Should remain in naming mode
	if !rm.snippetSt.naming {
		t.Error("expected to stay in naming when name is empty")
	}
	if len(rm.snippetSt.items) != 0 {
		t.Errorf("expected no snippets added, got %d", len(rm.snippetSt.items))
	}
}

func TestSnippetNaming_SaveSuccess(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := newSnippetTestModel(nil)
	m.snippetSt.naming = true
	m.snippetSt.input.SetValue("my query")
	m.textarea.SetValue("SELECT 1")

	result, _ := m.updateSnippetNaming(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(model)

	if rm.snippetSt.naming {
		t.Error("expected snippetNaming=false after save")
	}
	if len(rm.snippetSt.items) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(rm.snippetSt.items))
	}
	if rm.snippetSt.items[0].Name != "my query" || rm.snippetSt.items[0].Query != "SELECT 1" {
		t.Errorf("unexpected snippet: %+v", rm.snippetSt.items[0])
	}
	if rm.snippetSt.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", rm.snippetSt.cursor)
	}
}

func TestSnippetNaming_SaveReturnsToPrevMode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := newSnippetTestModel(nil)
	m.snippetSt.naming = true
	m.snippetSt.prevMode = insertMode
	m.snippetSt.input.SetValue("test")
	m.textarea.SetValue("SELECT 1")

	result, _ := m.updateSnippetNaming(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(model)

	if rm.mode != insertMode {
		t.Errorf("expected insertMode after save, got %q", rm.mode)
	}
	if rm.snippetSt.prevMode != "" {
		t.Error("expected snippetPrevMode cleared")
	}
}

func TestSnippet_DeleteOnEmptyList(t *testing.T) {
	m := newSnippetTestModel(nil)

	result, _ := m.updateSnippet(runeMsg("d"))
	rm := result.(model)

	if len(rm.snippetSt.items) != 0 {
		t.Error("expected no change on empty list delete")
	}
}

func TestSnippet_DeleteUpdatesState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	snippets := []snippet.Snippet{
		{Name: "a", Query: "SELECT 1"},
		{Name: "b", Query: "SELECT 2"},
		{Name: "c", Query: "SELECT 3"},
	}
	m := newSnippetTestModel(snippets)
	m.snippetSt.cursor = 1

	result, _ := m.updateSnippet(runeMsg("d"))
	rm := result.(model)

	if len(rm.snippetSt.items) != 2 {
		t.Fatalf("expected 2 snippets after delete, got %d", len(rm.snippetSt.items))
	}
	if rm.snippetSt.items[0].Name != "a" || rm.snippetSt.items[1].Name != "c" {
		t.Errorf("unexpected remaining snippets: %+v", rm.snippetSt.items)
	}
}

func TestSnippet_DeleteLastAdjustsCursor(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	snippets := []snippet.Snippet{
		{Name: "a", Query: "SELECT 1"},
		{Name: "b", Query: "SELECT 2"},
	}
	m := newSnippetTestModel(snippets)
	m.snippetSt.cursor = 1 // pointing at last item

	result, _ := m.updateSnippet(runeMsg("d"))
	rm := result.(model)

	if len(rm.snippetSt.items) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(rm.snippetSt.items))
	}
	if rm.snippetSt.cursor != 0 {
		t.Errorf("expected cursor adjusted to 0, got %d", rm.snippetSt.cursor)
	}
}
