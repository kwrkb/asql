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
	m.snippets = snippets
	m.snippetCursor = 0
	m.snippetInput = textinput.New()
	m.snippetInput.CharLimit = 100
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
	if rm.snippetCursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", rm.snippetCursor)
	}

	// j again
	m.snippetCursor = 1
	result, _ = m.updateSnippet(runeMsg("j"))
	rm = result.(model)
	if rm.snippetCursor != 2 {
		t.Errorf("expected cursor=2 after j, got %d", rm.snippetCursor)
	}

	// j at bottom boundary
	m.snippetCursor = 2
	result, _ = m.updateSnippet(runeMsg("j"))
	rm = result.(model)
	if rm.snippetCursor != 2 {
		t.Errorf("expected cursor=2 at boundary, got %d", rm.snippetCursor)
	}

	// k moves cursor up
	m.snippetCursor = 2
	result, _ = m.updateSnippet(runeMsg("k"))
	rm = result.(model)
	if rm.snippetCursor != 1 {
		t.Errorf("expected cursor=1 after k, got %d", rm.snippetCursor)
	}

	// k at top boundary
	m.snippetCursor = 0
	result, _ = m.updateSnippet(runeMsg("k"))
	rm = result.(model)
	if rm.snippetCursor != 0 {
		t.Errorf("expected cursor=0 at boundary, got %d", rm.snippetCursor)
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
	if rm.snippetCursor != 1 {
		t.Errorf("expected cursor=1 after down, got %d", rm.snippetCursor)
	}

	// Up arrow
	m.snippetCursor = 1
	result, _ = m.updateSnippet(tea.KeyMsg{Type: tea.KeyUp})
	rm = result.(model)
	if rm.snippetCursor != 0 {
		t.Errorf("expected cursor=0 after up, got %d", rm.snippetCursor)
	}
}

func TestSnippet_EnterLoadsQuery(t *testing.T) {
	snippets := []snippet.Snippet{
		{Name: "first", Query: "SELECT 1"},
		{Name: "second", Query: "SELECT 2"},
	}
	m := newSnippetTestModel(snippets)
	m.snippetCursor = 1

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

	if rm.snippetNaming {
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

	if !rm.snippetNaming {
		t.Error("expected snippetNaming=true")
	}
}

func TestSnippetNaming_EscCancels(t *testing.T) {
	m := newSnippetTestModel(nil)
	m.snippetNaming = true

	result, _ := m.updateSnippetNaming(tea.KeyMsg{Type: tea.KeyEsc})
	rm := result.(model)

	if rm.snippetNaming {
		t.Error("expected snippetNaming=false after Esc")
	}
}

func TestSnippetNaming_EscReturnsToPrevMode(t *testing.T) {
	m := newSnippetTestModel(nil)
	m.snippetNaming = true
	m.snippetPrevMode = insertMode

	result, _ := m.updateSnippetNaming(tea.KeyMsg{Type: tea.KeyEsc})
	rm := result.(model)

	if rm.mode != insertMode {
		t.Errorf("expected insertMode, got %q", rm.mode)
	}
	if rm.snippetPrevMode != "" {
		t.Error("expected snippetPrevMode to be cleared")
	}
}

func TestSnippetNaming_EmptyNameIgnored(t *testing.T) {
	m := newSnippetTestModel(nil)
	m.snippetNaming = true
	m.snippetInput.SetValue("  ")
	m.textarea.SetValue("SELECT 1")

	result, _ := m.updateSnippetNaming(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(model)

	// Should remain in naming mode
	if !rm.snippetNaming {
		t.Error("expected to stay in naming when name is empty")
	}
	if len(rm.snippets) != 0 {
		t.Errorf("expected no snippets added, got %d", len(rm.snippets))
	}
}

func TestSnippetNaming_SaveSuccess(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := newSnippetTestModel(nil)
	m.snippetNaming = true
	m.snippetInput.SetValue("my query")
	m.textarea.SetValue("SELECT 1")

	result, _ := m.updateSnippetNaming(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(model)

	if rm.snippetNaming {
		t.Error("expected snippetNaming=false after save")
	}
	if len(rm.snippets) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(rm.snippets))
	}
	if rm.snippets[0].Name != "my query" || rm.snippets[0].Query != "SELECT 1" {
		t.Errorf("unexpected snippet: %+v", rm.snippets[0])
	}
	if rm.snippetCursor != 0 {
		t.Errorf("expected cursor=0, got %d", rm.snippetCursor)
	}
}

func TestSnippetNaming_SaveReturnsToPrevMode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	m := newSnippetTestModel(nil)
	m.snippetNaming = true
	m.snippetPrevMode = insertMode
	m.snippetInput.SetValue("test")
	m.textarea.SetValue("SELECT 1")

	result, _ := m.updateSnippetNaming(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(model)

	if rm.mode != insertMode {
		t.Errorf("expected insertMode after save, got %q", rm.mode)
	}
	if rm.snippetPrevMode != "" {
		t.Error("expected snippetPrevMode cleared")
	}
}

func TestSnippet_DeleteOnEmptyList(t *testing.T) {
	m := newSnippetTestModel(nil)

	result, _ := m.updateSnippet(runeMsg("d"))
	rm := result.(model)

	if len(rm.snippets) != 0 {
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
	m.snippetCursor = 1

	result, _ := m.updateSnippet(runeMsg("d"))
	rm := result.(model)

	if len(rm.snippets) != 2 {
		t.Fatalf("expected 2 snippets after delete, got %d", len(rm.snippets))
	}
	if rm.snippets[0].Name != "a" || rm.snippets[1].Name != "c" {
		t.Errorf("unexpected remaining snippets: %+v", rm.snippets)
	}
}

func TestSnippet_DeleteLastAdjustsCursor(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	snippets := []snippet.Snippet{
		{Name: "a", Query: "SELECT 1"},
		{Name: "b", Query: "SELECT 2"},
	}
	m := newSnippetTestModel(snippets)
	m.snippetCursor = 1 // pointing at last item

	result, _ := m.updateSnippet(runeMsg("d"))
	rm := result.(model)

	if len(rm.snippets) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(rm.snippets))
	}
	if rm.snippetCursor != 0 {
		t.Errorf("expected cursor adjusted to 0, got %d", rm.snippetCursor)
	}
}
