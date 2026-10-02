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

	// The scroll is settled here, not in the render: View has a value
	// receiver, so a render that moved it would lose the move every frame.
	m.detail.scroll = m.detailScroll()
	m.syncViewport()
	return m, nil
}

// detailContentWidth is the value column's width inside the modal: the width
// minus the border (2) and the horizontal padding (4).
func (m model) detailContentWidth() int {
	return max(calcModalWidth(m.width, 72)-6, 10)
}

// detailModalHeight is the modal's height without its border. Capping it at
// m.height-2 keeps the bordered modal on the screen.
func (m model) detailModalHeight() int {
	return max(m.height-2, 1)
}

// detailFieldsHeight is how many lines the fields may take: the modal height
// minus the vertical padding (2) and the title with its margin (2).
func (m model) detailFieldsHeight() int {
	return max(m.detailModalHeight()-4, 2)
}

// detailRow is the row the overlay shows, from displayRows (full columns, in
// display order) rather than m.table.Rows() (windowed columns).
func (m model) detailRow() (row []string, idx, total int, ok bool) {
	sourceRows := m.displayRows
	if len(sourceRows) == 0 {
		sourceRows = m.table.Rows()
	}
	idx = m.table.Cursor()
	if idx < 0 || idx >= len(sourceRows) {
		return nil, idx, len(sourceRows), false
	}
	return sourceRows[idx], idx, len(sourceRows), true
}

// detailValueLines renders field i's value at the value column's width.
// A long value wraps, so a field is one label line plus as many lines as its
// value takes — not a fixed height.
func (m model) detailValueLines(row []string, i int, selected bool) []string {
	val := ""
	if i < len(row) {
		val = sanitize(row[i])
	}
	style := lipgloss.NewStyle().Foreground(textColor).Width(m.detailContentWidth())
	if selected {
		style = style.Background(lipgloss.Color("#1E293B"))
	}
	return strings.Split(style.Render(val), "\n")
}

// detailScroll returns the first field to draw so that the cursor field is on
// screen, measured with the fields' real heights. It starts from the stored
// scroll so the view does not jump while the cursor moves within it.
func (m model) detailScroll() int {
	row, _, _, ok := m.detailRow()
	cursor := m.detail.fieldCursor
	scroll := min(m.detail.scroll, cursor)
	if !ok {
		return max(scroll, 0)
	}
	budget := m.detailFieldsHeight()
	used := 0
	for i := scroll; i <= cursor; i++ {
		used += 1 + len(m.detailValueLines(row, i, i == cursor))
	}
	for used > budget && scroll < cursor {
		used -= 1 + len(m.detailValueLines(row, scroll, false))
		scroll++
	}
	return max(scroll, 0)
}

func (m model) renderWithDetailOverlay(background string) string {
	row, rowIdx, totalRows, ok := m.detailRow()
	if !ok {
		return background
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(mutedTextColor)

	selectedLabelStyle := lipgloss.NewStyle().
		Foreground(accentColor).
		Bold(true)

	title := titleStyle.Render(fmt.Sprintf("Row %d/%d", rowIdx+1, totalRows))

	// detailScroll is pure, so calling it here only covers a stored scroll
	// that a resize left stale; updateDetail is what keeps it.
	budget := m.detailFieldsHeight()
	var lines []string
	for i := m.detailScroll(); i < len(m.lastResult.Columns); i++ {
		colName := sanitize(m.lastResult.Columns[i])
		colType := ""
		if i < len(m.lastResult.ColumnTypes) && m.lastResult.ColumnTypes[i] != "" {
			colType = " " + dbutil.ShortenTypeName(sanitize(m.lastResult.ColumnTypes[i]))
		}
		selected := i == m.detail.fieldCursor
		label := labelStyle.Render(colName + colType)
		if selected {
			label = selectedLabelStyle.Render(colName + colType)
		}
		value := m.detailValueLines(row, i, selected)

		room := budget - len(lines)
		if 1+len(value) > room {
			if len(lines) > 0 {
				break
			}
			// The first field alone is taller than the modal: show as much
			// of its value as fits rather than nothing.
			value = value[:max(room-1, 0)]
		}
		lines = append(lines, truncateCells(label, m.detailContentWidth()))
		lines = append(lines, value...)
	}

	content := title + "\n" + strings.Join(lines, "\n")

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 2).
		Width(calcModalWidth(m.width, 72)).
		Height(m.detailModalHeight()).
		Background(panelBackground)

	modal := boxStyle.Render(content)

	return overlayModal(m.width, background, modal)
}
