package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) updateHistorySearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		if msg.Type == tea.KeyEnter && len(m.histSearch.results) > 0 && m.histSearch.cursor < len(m.histSearch.results) {
			idx := m.histSearch.results[m.histSearch.cursor]
			m.textarea.SetValue(m.queryHistory[idx])
			m.historyIdx = -1
			m.historyDraft = ""
		}
		m.histSearch.results = nil
		m.histSearch.cursor = 0
		m.mode = insertMode
		m.textarea.Focus()
		m.histSearch.input.Blur()
		m.setStatus("Insert mode", false)
		return m, nil
	case tea.KeyCtrlP, tea.KeyUp:
		if m.histSearch.cursor > 0 {
			m.histSearch.cursor--
		}
		return m, nil
	case tea.KeyCtrlN, tea.KeyDown:
		if m.histSearch.cursor < len(m.histSearch.results)-1 {
			m.histSearch.cursor++
		}
		return m, nil
	case tea.KeyCtrlR:
		// Move to next match (wrap around)
		if len(m.histSearch.results) > 1 {
			m.histSearch.cursor = (m.histSearch.cursor + 1) % len(m.histSearch.results)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.histSearch.input, cmd = m.histSearch.input.Update(msg)
	m.filterHistory(m.histSearch.input.Value())
	return m, cmd
}

func (m *model) filterHistory(query string) {
	m.histSearch.results = m.histSearch.results[:0]
	q := strings.ToLower(query)
	// Reverse order: newest first
	for i := len(m.queryHistory) - 1; i >= 0; i-- {
		if q == "" || strings.Contains(strings.ToLower(m.queryHistory[i]), q) {
			m.histSearch.results = append(m.histSearch.results, i)
		}
	}
	if m.histSearch.cursor >= len(m.histSearch.results) {
		m.histSearch.cursor = max(len(m.histSearch.results)-1, 0)
	}
}

func (m model) enterHistorySearchMode() (tea.Model, tea.Cmd) {
	if len(m.queryHistory) == 0 {
		return m, nil
	}
	m.mode = historySearchMode
	m.histSearch.input.Reset()
	m.histSearch.input.Focus()
	m.textarea.Blur()
	m.filterHistory("")
	m.histSearch.cursor = 0
	m.setStatus("History search", false)
	return m, textinput.Blink
}

func (m model) renderWithHistorySearchOverlay(background string) string {
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

	var items strings.Builder

	items.WriteString(lipgloss.NewStyle().Foreground(textColor).Background(panelBackground).Render("> "))
	items.WriteString(m.histSearch.input.View())
	items.WriteByte('\n')

	if len(m.histSearch.results) == 0 {
		items.WriteString(lipgloss.NewStyle().Foreground(mutedTextColor).Background(panelBackground).Render("(no matches)"))
	} else {
		maxVisible := max(min((m.height-10)/2, len(m.histSearch.results)), 1)
		start := 0
		if m.histSearch.cursor >= maxVisible {
			start = m.histSearch.cursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(m.histSearch.results))

		for i := start; i < end; i++ {
			histIdx := m.histSearch.results[i]
			// Flatten newlines, sanitize, then truncate
			preview := strings.Join(strings.Fields(sanitize(m.queryHistory[histIdx])), " ")
			maxLen := modalWidth - 10
			runes := []rune(preview)
			if maxLen > 0 && len(runes) > maxLen {
				preview = string(runes[:maxLen]) + "..."
			}
			if i == m.histSearch.cursor {
				items.WriteString(selectedStyle.Render(preview))
			} else {
				items.WriteString(itemStyle.Render(preview))
			}
			if i < end-1 {
				items.WriteByte('\n')
			}
		}
	}

	footer := "\n" + lipgloss.NewStyle().Foreground(mutedTextColor).Background(panelBackground).Render("Enter:select C-p/C-n:nav Esc:cancel")

	content := titleStyle.Render("History Search") + "\n" + items.String() + footer
	modal := boxStyle.Render(content)

	return overlayModal(m.width, background, modal)
}
