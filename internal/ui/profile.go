package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/profile"
)

func (m model) updateProfile(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.profileSt.naming {
		return m.updateProfileNaming(msg)
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.mode = normalMode
		m.textarea.Blur()
		m.setStatus("Normal mode", false)
		return m, nil
	case tea.KeyRunes:
		if msg.Alt {
			break
		}
		switch string(msg.Runes) {
		case "j":
			moveCursor(&m.profileSt.cursor, len(m.profileSt.items), 1)
		case "k":
			moveCursor(&m.profileSt.cursor, len(m.profileSt.items), -1)
		case "d":
			if len(m.profileSt.items) > 0 && m.profileSt.cursor < len(m.profileSt.items) {
				newProfiles := append(m.profileSt.items[:m.profileSt.cursor], m.profileSt.items[m.profileSt.cursor+1:]...)
				if err := profile.Save(newProfiles); err != nil {
					m.setStatus(fmt.Sprintf("Save failed: %v", err), true)
				} else {
					m.profileSt.items = newProfiles
					if m.profileSt.cursor >= len(m.profileSt.items) && m.profileSt.cursor > 0 {
						m.profileSt.cursor--
					}
					m.setStatus("Profile deleted", false)
				}
			}
		case "x":
			if len(m.profileSt.items) > 0 && m.profileSt.cursor < len(m.profileSt.items) {
				return m.switchProfile(m.profileSt.items[m.profileSt.cursor], true)
			}
		case "a":
			if m.rawDSN == "" {
				m.setStatus("No active connection to save", true)
				return m, nil
			}
			m.profileSt.naming = true
			m.profileSt.input.Reset()
			m.profileSt.input.Focus()
			return m, textinput.Blink
		}
	case tea.KeyDown:
		moveCursor(&m.profileSt.cursor, len(m.profileSt.items), 1)
	case tea.KeyUp:
		moveCursor(&m.profileSt.cursor, len(m.profileSt.items), -1)
	case tea.KeyEnter:
		if len(m.profileSt.items) > 0 && m.profileSt.cursor < len(m.profileSt.items) {
			return m.switchProfile(m.profileSt.items[m.profileSt.cursor], false)
		}
	}
	return m, nil
}

func (m model) updateProfileNaming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.profileSt.naming = false
		m.profileSt.input.Blur()
		return m, nil
	case tea.KeyEnter:
		name := strings.TrimSpace(m.profileSt.input.Value())
		if name == "" {
			return m, nil
		}
		newProfiles := profile.Upsert(m.profileSt.items, profile.Profile{Name: name, DSN: m.rawDSN})
		if err := profile.Save(newProfiles); err != nil {
			m.setStatus(fmt.Sprintf("Save failed: %v", err), true)
			m.profileSt.naming = false
			m.profileSt.input.Blur()
			return m, nil
		}
		m.profileSt.items = newProfiles
		m.setStatus(fmt.Sprintf("Saved profile: %s", name), false)
		m.profileSt.naming = false
		m.profileSt.input.Blur()
		m.profileSt.cursor = len(m.profileSt.items) - 1
		return m, nil
	}

	var cmd tea.Cmd
	m.profileSt.input, cmd = m.profileSt.input.Update(msg)
	return m, cmd
}

func (m model) switchProfile(p profile.Profile, reExecute bool) (tea.Model, tea.Cmd) {
	if m.connMgr.IsActive(p.DSN) {
		m.mode = normalMode
		m.textarea.Blur()
		if reExecute {
			query := strings.TrimSpace(m.textarea.Value())
			if query != "" {
				m.setStatus("Re-executing query...", false)
				return m, m.prepareAndExecuteQuery(query)
			}
		}
		m.setStatus(fmt.Sprintf("Already connected to %s", sanitize(p.Name)), false)
		return m, nil
	}
	m.setStatus(fmt.Sprintf("Connecting to %s...", sanitize(p.Name)), false)
	name := p.Name
	dsn := p.DSN
	cm := m.connMgr
	return m, func() tea.Msg {
		err := cm.Switch(name, dsn)
		return connSwitchedMsg{err: err, reExecute: reExecute}
	}
}

func (m model) renderWithProfileOverlay(background string) string {
	modalWidth := calcModalWidth(m.width, 60)
	innerWidth := max(modalWidth-6, 1)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor).
		MarginBottom(1)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 2).
		Width(modalWidth).
		Background(panelBackground)

	itemStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Background(panelBackground).
		Width(innerWidth).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(panelBackground).
		Background(accentColor).
		Bold(true).
		Width(innerWidth).
		Padding(0, 1)

	dsnPreviewStyle := lipgloss.NewStyle().
		Foreground(mutedTextColor).
		Background(panelBackground).
		Width(innerWidth).
		Padding(0, 1)

	var items strings.Builder

	if m.profileSt.naming {
		items.WriteString(lipgloss.NewStyle().Foreground(textColor).Background(panelBackground).Render("Name: "))
		items.WriteString(m.profileSt.input.View())
	} else if len(m.profileSt.items) == 0 {
		items.WriteString(lipgloss.NewStyle().Foreground(mutedTextColor).Background(panelBackground).Render("(no saved profiles)"))
	} else {
		maxVisible := max(min((m.height-8)/2, len(m.profileSt.items)), 1)
		start := 0
		if m.profileSt.cursor >= maxVisible {
			start = m.profileSt.cursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(m.profileSt.items))

		for i := start; i < end; i++ {
			p := m.profileSt.items[i]
			label := sanitize(p.Name)
			// Show connection status markers
			if m.connMgr.IsActive(p.DSN) {
				label = "\u25b6 " + label // ▶ active
			} else if m.connMgr.IsConnected(p.DSN) {
				label = "\u25cf " + label // ● connected
			} else {
				label = "  " + label
			}
			if i == m.profileSt.cursor {
				items.WriteString(selectedStyle.Render(label))
			} else {
				items.WriteString(itemStyle.Render(label))
			}
			// Show masked DSN preview
			preview := sanitize(db.MaskDSN(p.DSN))
			maxPreview := modalWidth - 10
			runes := []rune(preview)
			if maxPreview > 0 && len(runes) > maxPreview {
				preview = string(runes[:maxPreview]) + "..."
			}
			items.WriteByte('\n')
			items.WriteString(dsnPreviewStyle.Render(preview))
			if i < end-1 {
				items.WriteByte('\n')
			}
		}
	}

	title := "Connection Profiles"
	if m.profileSt.naming {
		title = "Save Profile"
	}

	var footer string
	if !m.profileSt.naming {
		footer = "\n" + lipgloss.NewStyle().Foreground(mutedTextColor).Background(panelBackground).Render("Enter:connect x:switch+exec d:delete a:add Esc:close")
	}

	content := titleStyle.Render(title) + "\n" + items.String() + footer
	modal := boxStyle.Render(content)

	return overlayModal(m.width, background, modal)
}
