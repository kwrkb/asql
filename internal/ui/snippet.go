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
	m.snippetPrevMode = m.mode
	m.mode = snippetMode
	m.snippetNaming = true
	m.snippetInput.Reset()
	m.snippetInput.Focus()
	m.textarea.Blur()
	m.setStatus("Save snippet", false)
	return m, textinput.Blink
}

func (m model) updateSnippet(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.snippetNaming {
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
			if len(m.snippets) > 0 && m.snippetCursor < len(m.snippets)-1 {
				m.snippetCursor++
			}
		case "k":
			if m.snippetCursor > 0 {
				m.snippetCursor--
			}
		case "d":
			if len(m.snippets) > 0 && m.snippetCursor < len(m.snippets) {
				newSnippets := append(m.snippets[:m.snippetCursor], m.snippets[m.snippetCursor+1:]...)
				if err := snippet.Save(newSnippets); err != nil {
					m.setStatus(fmt.Sprintf("Save failed: %v", err), true)
				} else {
					m.snippets = newSnippets
					if m.snippetCursor >= len(m.snippets) && m.snippetCursor > 0 {
						m.snippetCursor--
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
			m.snippetNaming = true
			m.snippetInput.Reset()
			m.snippetInput.Focus()
			return m, textinput.Blink
		}
	case tea.KeyDown:
		if len(m.snippets) > 0 && m.snippetCursor < len(m.snippets)-1 {
			m.snippetCursor++
		}
	case tea.KeyUp:
		if m.snippetCursor > 0 {
			m.snippetCursor--
		}
	case tea.KeyEnter:
		if len(m.snippets) > 0 && m.snippetCursor < len(m.snippets) {
			m.textarea.SetValue(m.snippets[m.snippetCursor].Query)
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
		m.snippetNaming = false
		m.snippetInput.Blur()
		// If entered naming directly via Ctrl+S, return to the original mode
		if m.snippetPrevMode != "" {
			m.mode = m.snippetPrevMode
			m.snippetPrevMode = ""
			if m.mode == insertMode {
				m.textarea.Focus()
			}
			m.setStatus(string(m.mode)+" mode", false)
		}
		return m, nil
	case tea.KeyEnter:
		name := strings.TrimSpace(m.snippetInput.Value())
		if name == "" {
			return m, nil
		}
		query := strings.TrimSpace(m.textarea.Value())
		if query == "" {
			m.setStatus("No query to save", true)
			m.snippetNaming = false
			m.snippetInput.Blur()
			return m, nil
		}
		newSnippets := append(append([]snippet.Snippet{}, m.snippets...), snippet.Snippet{Name: name, Query: query})
		if err := snippet.Save(newSnippets); err != nil {
			m.setStatus(fmt.Sprintf("Save failed: %v", err), true)
			m.snippetNaming = false
			m.snippetInput.Blur()
			// Restore previous mode on failure (same as Esc path)
			if m.snippetPrevMode != "" {
				m.mode = m.snippetPrevMode
				m.snippetPrevMode = ""
				if m.mode == insertMode {
					m.textarea.Focus()
				}
			}
			return m, nil
		}
		m.snippets = newSnippets
		m.setStatus(fmt.Sprintf("Saved snippet: %s", name), false)
		m.snippetNaming = false
		m.snippetInput.Blur()
		m.snippetCursor = len(m.snippets) - 1
		// If entered naming directly via Ctrl+S, return to the original mode
		if m.snippetPrevMode != "" {
			m.mode = m.snippetPrevMode
			m.snippetPrevMode = ""
			if m.mode == insertMode {
				m.textarea.Focus()
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.snippetInput, cmd = m.snippetInput.Update(msg)
	return m, cmd
}

func (m model) renderWithSnippetOverlay(background string) string {
	modalWidth := min(m.width-4, 50)
	if modalWidth < 20 {
		modalWidth = 20
	}

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
		Width(modalWidth - 6).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(panelBackground).
		Background(accentColor).
		Bold(true).
		Width(modalWidth - 6).
		Padding(0, 1)

	queryPreviewStyle := lipgloss.NewStyle().
		Foreground(mutedTextColor).
		Background(panelBackground).
		Width(modalWidth - 6).
		Padding(0, 1)

	var items strings.Builder

	if m.snippetNaming {
		items.WriteString(lipgloss.NewStyle().Foreground(textColor).Background(panelBackground).Render("Name: "))
		items.WriteString(m.snippetInput.View())
	} else if len(m.snippets) == 0 {
		items.WriteString(lipgloss.NewStyle().Foreground(mutedTextColor).Background(panelBackground).Render("(no saved queries)"))
	} else {
		maxVisible := max(min((m.height-8)/2, len(m.snippets)), 1)
		start := 0
		if m.snippetCursor >= maxVisible {
			start = m.snippetCursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(m.snippets))

		for i := start; i < end; i++ {
			s := m.snippets[i]
			label := sanitize(s.Name)
			if i == m.snippetCursor {
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
	if m.snippetNaming {
		title = "Save Query"
	}

	var footer string
	if !m.snippetNaming {
		footer = "\n" + lipgloss.NewStyle().Foreground(mutedTextColor).Background(panelBackground).Render("Enter:load d:delete a:add Esc:close")
	}

	content := titleStyle.Render(title) + "\n" + items.String() + footer
	modal := boxStyle.Render(content)

	bgH := lipgloss.Height(background)

	return lipgloss.Place(m.width, bgH, lipgloss.Center, lipgloss.Center, modal,
		lipgloss.WithWhitespaceBackground(appBackground))
}
