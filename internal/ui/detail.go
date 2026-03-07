package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/asql/internal/db/dbutil"
)

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	numFields := len(m.lastResult.Columns)

	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		m.mode = normalMode
		m.setStatus("Normal mode", false)
	case tea.KeyDown:
		moveCursor(&m.detail.fieldCursor, numFields, 1)
	case tea.KeyUp:
		moveCursor(&m.detail.fieldCursor, numFields, -1)
	case tea.KeyRunes:
		if msg.Alt {
			break
		}
		switch string(msg.Runes) {
		case "q":
			m.mode = normalMode
			m.setStatus("Normal mode", false)
		case "j":
			moveCursor(&m.detail.fieldCursor, numFields, 1)
		case "k":
			moveCursor(&m.detail.fieldCursor, numFields, -1)
		case "n", "l":
			m.table.MoveDown(1)
			m.detail.fieldCursor = 0
			m.detail.scroll = 0
		case "N", "h":
			m.table.MoveUp(1)
			m.detail.fieldCursor = 0
			m.detail.scroll = 0
		}
	}

	m.syncViewport()
	return m, nil
}

func (m model) renderWithDetailOverlay(background string) string {
	modalWidth := calcModalWidth(m.width, 72)
	modalHeight := m.height - 6

	// Use displayRows (full columns) instead of m.table.Rows() (windowed)
	sourceRows := m.displayRows
	if len(sourceRows) == 0 {
		sourceRows = m.table.Rows()
	}
	rowIdx := m.table.Cursor()
	totalRows := len(sourceRows)
	if rowIdx < 0 || rowIdx >= totalRows {
		return background
	}
	row := sourceRows[rowIdx]

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(mutedTextColor)

	valueStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Width(modalWidth - 6)

	selectedLabelStyle := lipgloss.NewStyle().
		Foreground(accentColor).
		Bold(true)

	selectedValueStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Background(lipgloss.Color("#1E293B")).
		Width(modalWidth - 6)

	title := titleStyle.Render(fmt.Sprintf("Row %d/%d", rowIdx+1, totalRows))

	// Calculate scroll offset so cursor stays visible
	contentHeight := max(modalHeight-3, 1) // title + margins
	linesPerField := 3                     // label line + value line + separator
	maxVisibleFields := max(contentHeight/linesPerField, 1)
	if m.detail.fieldCursor >= m.detail.scroll+maxVisibleFields {
		m.detail.scroll = m.detail.fieldCursor - maxVisibleFields + 1
	}
	if m.detail.fieldCursor < m.detail.scroll {
		m.detail.scroll = m.detail.fieldCursor
	}

	var b strings.Builder
	linesRendered := 0
	for i := m.detail.scroll; i < len(m.lastResult.Columns); i++ {
		if linesRendered+linesPerField > contentHeight {
			break
		}

		colName := sanitize(m.lastResult.Columns[i])
		colType := ""
		if i < len(m.lastResult.ColumnTypes) && m.lastResult.ColumnTypes[i] != "" {
			colType = " " + dbutil.ShortenTypeName(sanitize(m.lastResult.ColumnTypes[i]))
		}

		val := ""
		if i < len(row) {
			val = sanitize(row[i])
		}

		if i == m.detail.fieldCursor {
			b.WriteString(selectedLabelStyle.Render(colName + colType))
			b.WriteByte('\n')
			b.WriteString(selectedValueStyle.Render(val))
		} else {
			b.WriteString(labelStyle.Render(colName + colType))
			b.WriteByte('\n')
			b.WriteString(valueStyle.Render(val))
		}
		b.WriteByte('\n')
		linesRendered += linesPerField
	}

	content := title + "\n" + strings.TrimRight(b.String(), "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight).
		Background(panelBackground)

	modal := boxStyle.Render(content)

	return overlayModal(m.width, background, modal)
}
