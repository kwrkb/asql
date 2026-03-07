package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) updateSidebar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.sidebar.open = false
		m.mode = normalMode
		m.setStatus("Normal mode", false)
		m.resize()
	case tea.KeyRunes:
		if msg.Alt {
			break
		}
		switch string(msg.Runes) {
		case "t":
			m.sidebar.open = false
			m.mode = normalMode
			m.setStatus("Normal mode", false)
			m.resize()
		case "j":
			moveCursor(&m.sidebar.cursor, len(m.sidebar.tables), 1)
		case "k":
			moveCursor(&m.sidebar.cursor, len(m.sidebar.tables), -1)
		}
	case tea.KeyDown:
		moveCursor(&m.sidebar.cursor, len(m.sidebar.tables), 1)
	case tea.KeyUp:
		moveCursor(&m.sidebar.cursor, len(m.sidebar.tables), -1)
	case tea.KeyEnter:
		if len(m.sidebar.tables) > 0 {
			name := m.sidebar.tables[m.sidebar.cursor]
			quoted := m.activeDB().QuoteIdentifier(name)
			query := fmt.Sprintf("SELECT * FROM %s LIMIT 100;", quoted)
			m.textarea.SetValue(query)
			m.sidebar.open = false
			m.mode = insertMode
			m.textarea.Focus()
			m.setStatus("Insert mode", false)
			m.resize()
		}
	}
	m.syncViewport()
	return m, nil
}

func (m model) renderSidebar() string {
	height := m.height - 1 // exclude status bar
	w := sidebarWidth

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor).
		Background(panelBackground).
		Width(w).
		Padding(0, 1)

	itemStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Background(panelBackground).
		Width(w).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(panelBackground).
		Background(accentColor).
		Bold(true).
		Width(w).
		Padding(0, 1)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Tables"))
	b.WriteByte('\n')
	lines := 1

	// Calculate scroll offset so cursor stays visible
	maxVisible := height - 2 // title line + border allowance
	scrollOffset := 0
	if maxVisible > 0 && m.sidebar.cursor >= maxVisible {
		scrollOffset = m.sidebar.cursor - maxVisible + 1
	}

	for i := scrollOffset; i < len(m.sidebar.tables); i++ {
		if lines >= height-1 {
			break
		}
		name := m.sidebar.tables[i]
		if i == m.sidebar.cursor {
			b.WriteString(selectedStyle.Render(name))
		} else {
			b.WriteString(itemStyle.Render(name))
		}
		b.WriteByte('\n')
		lines++
	}

	if len(m.sidebar.tables) == 0 {
		b.WriteString(itemStyle.Foreground(mutedTextColor).Render("(no tables)"))
		b.WriteByte('\n')
		lines++
	}

	// Fill remaining space
	emptyStyle := lipgloss.NewStyle().
		Background(panelBackground).
		Width(w)
	for lines < height {
		b.WriteString(emptyStyle.Render(""))
		b.WriteByte('\n')
		lines++
	}

	return lipgloss.NewStyle().
		Width(w).
		Height(height).
		Background(panelBackground).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(panelBorder).
		BorderRight(true).
		Render(strings.TrimRight(b.String(), "\n"))
}
