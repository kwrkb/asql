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
		if msg.Type == tea.KeyEnter && len(m.historySearchResults) > 0 && m.historySearchCursor < len(m.historySearchResults) {
			idx := m.historySearchResults[m.historySearchCursor]
			m.textarea.SetValue(m.queryHistory[idx])
			m.historyIdx = -1
			m.historyDraft = ""
		}
		m.historySearchResults = nil
		m.historySearchCursor = 0
		m.mode = insertMode
		m.textarea.Focus()
		m.historySearchInput.Blur()
		m.setStatus("Insert mode", false)
		return m, nil
	case tea.KeyCtrlP, tea.KeyUp:
		if m.historySearchCursor > 0 {
			m.historySearchCursor--
		}
		return m, nil
	case tea.KeyCtrlN, tea.KeyDown:
		if m.historySearchCursor < len(m.historySearchResults)-1 {
			m.historySearchCursor++
		}
		return m, nil
	case tea.KeyCtrlR:
		// Move to next match (wrap around)
		if len(m.historySearchResults) > 1 {
			m.historySearchCursor = (m.historySearchCursor + 1) % len(m.historySearchResults)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.historySearchInput, cmd = m.historySearchInput.Update(msg)
	m.filterHistory(m.historySearchInput.Value())
	return m, cmd
}

func (m *model) filterHistory(query string) {
	m.historySearchResults = m.historySearchResults[:0]
	q := strings.ToLower(query)
	// Reverse order: newest first
	for i := len(m.queryHistory) - 1; i >= 0; i-- {
		if q == "" || strings.Contains(strings.ToLower(m.queryHistory[i]), q) {
			m.historySearchResults = append(m.historySearchResults, i)
		}
	}
	if m.historySearchCursor >= len(m.historySearchResults) {
		m.historySearchCursor = max(len(m.historySearchResults)-1, 0)
	}
}

func (m model) enterHistorySearchMode() (tea.Model, tea.Cmd) {
	if len(m.queryHistory) == 0 {
		return m, nil
	}
	m.mode = historySearchMode
	m.historySearchInput.Reset()
	m.historySearchInput.Focus()
	m.textarea.Blur()
	m.filterHistory("")
	m.historySearchCursor = 0
	m.setStatus("History search", false)
	return m, textinput.Blink
}

func (m model) renderWithHistorySearchOverlay(background string) string {
	modalWidth := min(m.width-4, 60)
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

	var items strings.Builder

	// Search input — adapt width to modal
	m.historySearchInput.Width = max(modalWidth-10, 10)
	items.WriteString(lipgloss.NewStyle().Foreground(textColor).Background(panelBackground).Render("> "))
	items.WriteString(m.historySearchInput.View())
	items.WriteByte('\n')

	if len(m.historySearchResults) == 0 {
		items.WriteString(lipgloss.NewStyle().Foreground(mutedTextColor).Background(panelBackground).Render("(no matches)"))
	} else {
		maxVisible := max(min((m.height-10)/2, len(m.historySearchResults)), 1)
		start := 0
		if m.historySearchCursor >= maxVisible {
			start = m.historySearchCursor - maxVisible + 1
		}
		end := min(start+maxVisible, len(m.historySearchResults))

		for i := start; i < end; i++ {
			histIdx := m.historySearchResults[i]
			// Flatten newlines, sanitize, then truncate
			preview := strings.Join(strings.Fields(sanitize(m.queryHistory[histIdx])), " ")
			maxLen := modalWidth - 10
			runes := []rune(preview)
			if maxLen > 0 && len(runes) > maxLen {
				preview = string(runes[:maxLen]) + "..."
			}
			if i == m.historySearchCursor {
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

	bgH := lipgloss.Height(background)

	return lipgloss.Place(m.width, bgH, lipgloss.Center, lipgloss.Center, modal,
		lipgloss.WithWhitespaceBackground(appBackground))
}
