package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/profile"
)

func newProfileModel(profiles []profile.Profile) *model {
	m := newTestModel()
	m.mode = profileMode
	m.profileSt.items = profiles
	m.profileSt.cursor = 0
	m.profileSt.input = textinput.New()
	m.profileSt.input.CharLimit = 100
	return m
}

func TestProfile_EscReturnsNormal(t *testing.T) {
	m := newProfileModel(nil)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(model)
	if result.mode != normalMode {
		t.Errorf("mode = %v, want normalMode", result.mode)
	}
}

func TestProfile_JMovesDown(t *testing.T) {
	profiles := []profile.Profile{
		{Name: "prod", DSN: "sqlite://prod.db"},
		{Name: "dev", DSN: "sqlite://dev.db"},
		{Name: "test", DSN: "sqlite://test.db"},
	}
	m := newProfileModel(profiles)
	updated, _ := m.Update(runeMsg("j"))
	result := updated.(model)
	if result.profileSt.cursor != 1 {
		t.Errorf("cursor = %d, want 1", result.profileSt.cursor)
	}
}

func TestProfile_KMovesUp(t *testing.T) {
	profiles := []profile.Profile{
		{Name: "a", DSN: "sqlite://a.db"},
		{Name: "b", DSN: "sqlite://b.db"},
	}
	m := newProfileModel(profiles)
	m.profileSt.cursor = 1
	updated, _ := m.Update(runeMsg("k"))
	result := updated.(model)
	if result.profileSt.cursor != 0 {
		t.Errorf("cursor = %d, want 0", result.profileSt.cursor)
	}
}

func TestProfile_DownArrow(t *testing.T) {
	profiles := []profile.Profile{
		{Name: "a", DSN: "sqlite://a.db"},
		{Name: "b", DSN: "sqlite://b.db"},
	}
	m := newProfileModel(profiles)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	result := updated.(model)
	if result.profileSt.cursor != 1 {
		t.Errorf("cursor = %d, want 1", result.profileSt.cursor)
	}
}

func TestProfile_UpArrow(t *testing.T) {
	profiles := []profile.Profile{
		{Name: "a", DSN: "sqlite://a.db"},
		{Name: "b", DSN: "sqlite://b.db"},
	}
	m := newProfileModel(profiles)
	m.profileSt.cursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	result := updated.(model)
	if result.profileSt.cursor != 0 {
		t.Errorf("cursor = %d, want 0", result.profileSt.cursor)
	}
}

func TestProfile_CursorBoundary(t *testing.T) {
	profiles := []profile.Profile{
		{Name: "a", DSN: "sqlite://a.db"},
		{Name: "b", DSN: "sqlite://b.db"},
	}
	m := newProfileModel(profiles)

	t.Run("k at top stays at 0", func(t *testing.T) {
		m.profileSt.cursor = 0
		updated, _ := m.Update(runeMsg("k"))
		result := updated.(model)
		if result.profileSt.cursor != 0 {
			t.Errorf("cursor at top after k = %d, want 0", result.profileSt.cursor)
		}
	})

	t.Run("j at bottom stays at len-1", func(t *testing.T) {
		m.profileSt.cursor = 1
		updated, _ := m.Update(runeMsg("j"))
		result := updated.(model)
		if result.profileSt.cursor != 1 {
			t.Errorf("cursor at bottom after j = %d, want 1", result.profileSt.cursor)
		}
	})
}

func TestProfile_AddWithNoConnectionErrors(t *testing.T) {
	m := newProfileModel(nil)
	m.rawDSN = "" // no active connection
	updated, _ := m.Update(runeMsg("a"))
	result := updated.(model)
	if !result.statusError {
		t.Error("expected statusError=true when no active connection")
	}
	if result.profileSt.naming {
		t.Error("naming should not be activated when no connection")
	}
}

func TestProfile_AddEntersNamingMode(t *testing.T) {
	m := newProfileModel(nil)
	m.rawDSN = "sqlite://test.db"
	updated, _ := m.Update(runeMsg("a"))
	result := updated.(model)
	if !result.profileSt.naming {
		t.Error("profileSt.naming should be true after 'a' with active connection")
	}
}

func TestProfileNaming_EscCancels(t *testing.T) {
	m := newProfileModel(nil)
	m.profileSt.naming = true
	m.profileSt.input.Focus()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(model)
	if result.profileSt.naming {
		t.Error("profileSt.naming should be false after Esc")
	}
}

func TestProfileNaming_EmptyEnterNoop(t *testing.T) {
	m := newProfileModel(nil)
	m.profileSt.naming = true
	m.profileSt.input.SetValue("")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(model)
	if !result.profileSt.naming {
		t.Error("naming should remain true when Enter with empty name")
	}
}

func TestProfile_AltKeyIgnored(t *testing.T) {
	profiles := []profile.Profile{
		{Name: "a", DSN: "sqlite://a.db"},
		{Name: "b", DSN: "sqlite://b.db"},
	}
	m := newProfileModel(profiles)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j"), Alt: true})
	result := updated.(model)
	if result.profileSt.cursor != 0 {
		t.Errorf("Alt+j should not move cursor, got %d", result.profileSt.cursor)
	}
}
