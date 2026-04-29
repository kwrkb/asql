package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/asql/internal/snippet"
)

func (m model) enterSnippetNamingMode() (tea.Model, tea.Cmd) {
	query := strings.TrimSpace(m.textarea.Value())
	if query == "" {
		m.setStatus("No query to save", true)
		return m, nil
	}
	m.snippetSt.prevMode = m.mode
	m.mode = snippetMode
	m.snippetSt.naming = true
	m.snippetSt.input.Reset()
	m.snippetSt.input.Focus()
	m.textarea.Blur()
	m.setStatus("Save snippet", false)
	return m, textinput.Blink
}

func (m model) updateSnippet(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.snippetSt.naming {
		return m.updateSnippetNaming(msg)
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
			moveCursor(&m.snippetSt.cursor, len(m.snippetSt.items), 1)
		case "k":
			moveCursor(&m.snippetSt.cursor, len(m.snippetSt.items), -1)
		case "d":
			if len(m.snippetSt.items) > 0 && m.snippetSt.cursor < len(m.snippetSt.items) {
				newSnippets := append(m.snippetSt.items[:m.snippetSt.cursor], m.snippetSt.items[m.snippetSt.cursor+1:]...)
				if err := snippet.Save(newSnippets); err != nil {
					m.setStatus(fmt.Sprintf("Save failed: %v", err), true)
				} else {
					m.snippetSt.items = newSnippets
					if m.snippetSt.cursor >= len(m.snippetSt.items) && m.snippetSt.cursor > 0 {
						m.snippetSt.cursor--
					}
					m.setStatus("Snippet deleted", false)
				}
			}
		case "a":
			query := strings.TrimSpace(m.textarea.Value())
			if query == "" {
				m.setStatus("No query to save", true)
				return m, nil
			}
			m.snippetSt.naming = true
			m.snippetSt.input.Reset()
			m.snippetSt.input.Focus()
			return m, textinput.Blink
		}
	case tea.KeyDown:
		moveCursor(&m.snippetSt.cursor, len(m.snippetSt.items), 1)
	case tea.KeyUp:
		moveCursor(&m.snippetSt.cursor, len(m.snippetSt.items), -1)
	case tea.KeyEnter:
		if len(m.snippetSt.items) > 0 && m.snippetSt.cursor < len(m.snippetSt.items) {
			m.textarea.SetValue(m.snippetSt.items[m.snippetSt.cursor].Query)
			m.mode = insertMode
			m.textarea.Focus()
			m.setStatus("Snippet loaded", false)
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateSnippetNaming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.snippetSt.naming = false
		m.snippetSt.input.Blur()
		// If entered naming directly via Ctrl+S, return to the original mode
		if m.snippetSt.prevMode != "" {
			m.mode = m.snippetSt.prevMode
			m.snippetSt.prevMode = ""
			if m.mode == insertMode {
				m.textarea.Focus()
			}
			m.setStatus(string(m.mode)+" mode", false)
		}
		return m, nil
	case tea.KeyEnter:
		name := strings.TrimSpace(m.snippetSt.input.Value())
		if name == "" {
			return m, nil
		}
		query := strings.TrimSpace(m.textarea.Value())
		if query == "" {
			m.setStatus("No query to save", true)
			m.snippetSt.naming = false
			m.snippetSt.input.Blur()
			return m, nil
		}
		newSnippets := append(append([]snippet.Snippet{}, m.snippetSt.items...), snippet.Snippet{Name: name, Query: query})
		if err := snippet.Save(newSnippets); err != nil {
			m.setStatus(fmt.Sprintf("Save failed: %v", err), true)
			m.snippetSt.naming = false
			m.snippetSt.input.Blur()
			// Restore previous mode on failure (same as Esc path)
			if m.snippetSt.prevMode != "" {
				m.mode = m.snippetSt.prevMode
				m.snippetSt.prevMode = ""
				if m.mode == insertMode {
					m.textarea.Focus()
				}
			}
			return m, nil
		}
		m.snippetSt.items = newSnippets
		m.setStatus(fmt.Sprintf("Saved snippet: %s", name), false)
		m.snippetSt.naming = false
		m.snippetSt.input.Blur()
		m.snippetSt.cursor = len(m.snippetSt.items) - 1
		// If entered naming directly via Ctrl+S, return to the original mode
		if m.snippetSt.prevMode != "" {
			m.mode = m.snippetSt.prevMode
			m.snippetSt.prevMode = ""
			if m.mode == insertMode {
				m.textarea.Focus()
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.snippetSt.input, cmd = m.snippetSt.input.Update(msg)
	return m, cmd
}

func (m model) renderWithSnippetOverlay(background string) string {
	modalWidth := calcModalWidth(m.width, 50)
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

	queryPreviewStyle := lipgloss.NewStyle().
		Foreground(mutedTextColor).
		Background(panelBackground).
		Width(innerWidth).
		Padding(0, 1)

	var items strings.Builder

	if m.snippetSt.naming {
		items.WriteString(lipgloss.NewStyle().Foreground(textColor).Background(panelBackground).Render("Name: "))
		items.WriteString(m.snippetSt.input.View())
	} else if len(m.snippetSt.items) == 0 {
		items.WriteString(lipgloss.NewStyle().Foreground(mutedTextColor).Background(panelBackground).Render("(no saved queries)"))
	} else {
		maxVisible := max(min((m.height-8)/2, len(m.snippetSt.items)), 1)
		start := 0
		if m.snippetSt.cursor >= maxVisible {
			start = m.snippetSt.cursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(m.snippetSt.items))

		for i := start; i < end; i++ {
			s := m.snippetSt.items[i]
			label := sanitize(s.Name)
			if i == m.snippetSt.cursor {
				items.WriteString(selectedStyle.Render(label))
			} else {
				items.WriteString(itemStyle.Render(label))
			}
			// Show query preview: sanitize, flatten newlines to spaces, then truncate (rune-safe)
			preview := strings.Join(strings.Fields(sanitize(s.Query)), " ")
			maxPreview := modalWidth - 10
			runes := []rune(preview)
			if maxPreview > 0 && len(runes) > maxPreview {
				preview = string(runes[:maxPreview]) + "..."
			}
			items.WriteByte('\n')
			items.WriteString(queryPreviewStyle.Render(preview))
			if i < end-1 {
				items.WriteByte('\n')
			}
		}
	}

	title := "Saved Queries"
	if m.snippetSt.naming {
		title = "Save Query"
	}

	var footer string
	if !m.snippetSt.naming {
		footer = "\n" + lipgloss.NewStyle().Foreground(mutedTextColor).Background(panelBackground).Render("Enter:load d:delete a:add Esc:close")
	}

	content := titleStyle.Render(title) + "\n" + items.String() + footer
	modal := boxStyle.Render(content)

	return overlayModal(m.width, background, modal)
}
