package ui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/dbutil"
)

// adjustColOffset ensures colCursor is within the visible column window.
func (m *model) adjustColOffset() {
	if m.colCursor < m.colOffset {
		m.colOffset = m.colCursor
	}
	_, visEnd := m.visibleColumnRange()
	for m.colCursor >= visEnd && m.colOffset < len(m.cachedColWidths)-1 {
		m.colOffset++
		_, visEnd = m.visibleColumnRange()
	}
	m.viewportDirty = true
}

// visibleColumnRange returns the range [start, end) of columns that fit within
// the available content width, starting from colOffset.
func (m *model) visibleColumnRange() (int, int) {
	if len(m.cachedColWidths) == 0 {
		return 0, 0
	}
	available := m.contentWidth() - 8 // border(2) + padding(2) + margin
	start := m.colOffset
	if start >= len(m.cachedColWidths) {
		start = 0
	}
	if available <= 0 {
		return start, min(start+1, len(m.cachedColWidths))
	}
	sum := 0
	for i := start; i < len(m.cachedColWidths); i++ {
		w := min(m.cachedColWidths[i], available) + 1 // column width + cell gap
		if sum+w > available && i > start {
			return start, i
		}
		sum += w
	}
	return start, len(m.cachedColWidths)
}

func (m *model) syncViewport() {
	if len(m.lastResult.Columns) == 0 || len(m.cachedColWidths) == 0 {
		// No windowing needed for message-only results
		panel := lipgloss.NewStyle().
			Width(max(m.contentWidth(), 0)).
			Height(max(m.resultsHeight(), 0)).
			Background(panelBackground).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(panelBorder).
			Padding(0, 1).
			Render(m.table.View())
		m.viewport.SetContent(panel)
		return
	}

	// Ensure colCursor stays within the visible window (e.g. after resize)
	m.adjustColOffset()

	visStart, visEnd := m.visibleColumnRange()

	// Rebuild columns/rows only when the visible window or column cursor changes.
	// For row-only navigation (j/k) we skip the expensive rebuild.
	rebuildNeeded := visStart != m.lastVisStart || visEnd != m.lastVisEnd || m.viewportDirty
	if rebuildNeeded {
		// Build windowed columns
		selectedStyle := lipgloss.NewStyle().Reverse(true)
		columns := make([]table.Column, 0, visEnd-visStart)
		for i := visStart; i < visEnd; i++ {
			header := sanitize(m.lastResult.Columns[i])
			if i < len(m.lastResult.ColumnTypes) && m.lastResult.ColumnTypes[i] != "" {
				shortType := dbutil.ShortenTypeName(sanitize(m.lastResult.ColumnTypes[i]))
				header = header + " " + typeStyle.Render(shortType)
			}
			if i == m.sortCol && m.sortDir != sortNone {
				header += sortIndicator(m.sortDir)
			}
			if m.mode == normalMode && i == m.colCursor && (m.pinned == nil || m.comparePane == 1) {
				header = selectedStyle.Render(header)
			}
			columns = append(columns, table.Column{Title: header, Width: min(m.cachedColWidths[i], max(m.contentWidth()-8, 1))})
		}

		// Build windowed rows with sanitized cell values
		rows := make([]table.Row, 0, len(m.displayRows))
		for rowIdx, row := range m.displayRows {
			windowed := make(table.Row, 0, visEnd-visStart)
			for i := visStart; i < visEnd; i++ {
				if i < len(row) {
					cell := sanitize(row[i])
					if m.activeCellDiff(rowIdx, i) {
						cell = diffCellStyle.Render(cell)
					}
					windowed = append(windowed, cell)
				} else {
					windowed = append(windowed, "")
				}
			}
			rows = append(rows, windowed)
		}

		// Preserve table cursor position across column changes
		cursor := m.table.Cursor()
		m.table.SetRows([]table.Row{})
		m.table.SetColumns(columns)
		m.table.SetRows(rows)
		m.table.SetCursor(cursor)
		m.lastVisStart = visStart
		m.lastVisEnd = visEnd
		m.viewportDirty = false
	}

	panel := lipgloss.NewStyle().
		Width(max(m.contentWidth(), 0)).
		Height(max(m.resultsHeight(), 0)).
		Background(panelBackground).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(panelBorder).
		Padding(0, 1).
		Render(m.table.View())
	m.viewport.SetContent(panel)
}

func (m *model) applyResult(result db.QueryResult) {
	m.lastResult = result
	m.applyResultWithSort(result)
}

func columnWidth(title string, rows [][]string, idx int) int {
	width := lipgloss.Width(title)
	for _, row := range rows {
		if idx >= len(row) {
			continue
		}
		width = max(width, lipgloss.Width(row[idx]))
	}

	if width < 12 {
		return 12
	}
	return min(width+2, 32)
}

func (m *model) toggleSort() {
	if m.colCursor == m.sortCol {
		switch m.sortDir {
		case sortNone:
			m.sortDir = sortAsc
		case sortAsc:
			m.sortDir = sortDesc
		case sortDesc:
			m.sortDir = sortNone
		}
	} else {
		m.sortCol = m.colCursor
		m.sortDir = sortAsc
	}
	m.applySortedResult()
}

func (m *model) applySortedResult() {
	result := m.lastResult
	result.Rows = sortedRows(m.lastResult.Rows, m.sortCol, m.sortDir)
	m.applyResultWithSort(result)
	m.table.GotoTop()
}

// applyResultWithSort computes column widths, saves displayRows, and delegates rendering to syncViewport.
func (m *model) applyResultWithSort(result db.QueryResult) {
	if len(result.Columns) == 0 {
		// Message-only result: set directly without windowing
		m.cachedColWidths = nil
		m.displayRows = nil
		columns := []table.Column{{Title: "Result", Width: max(m.width-6, 20)}}
		rows := []table.Row{{sanitize(result.Message)}}
		m.table.SetRows([]table.Row{})
		m.table.SetColumns(columns)
		m.table.SetRows(rows)
		m.setStatus(sanitize(result.Message), false)
		m.syncViewport()
		return
	}

	// Compute column widths
	m.cachedColWidths = make([]int, len(result.Columns))
	for i, title := range result.Columns {
		header := sanitize(title)
		if i < len(result.ColumnTypes) && result.ColumnTypes[i] != "" {
			shortType := dbutil.ShortenTypeName(sanitize(result.ColumnTypes[i]))
			header = header + " " + typeStyle.Render(shortType)
		}
		if i == m.sortCol && m.sortDir != sortNone {
			header += sortIndicator(m.sortDir)
		}
		m.cachedColWidths[i] = columnWidth(header, result.Rows, i)
	}

	// Save displayRows for windowing
	m.displayRows = make([]table.Row, 0, len(result.Rows))
	for _, row := range result.Rows {
		m.displayRows = append(m.displayRows, table.Row(row))
	}
	if len(m.displayRows) == 0 {
		sentinel := make(table.Row, len(result.Columns))
		sentinel[0] = "(no rows)"
		m.displayRows = []table.Row{sentinel}
	}

	// Reset colOffset if it exceeds new column count
	if m.colOffset >= len(result.Columns) {
		m.colOffset = 0
	}

	m.setStatus(sanitize(result.Message), false)
	m.viewportDirty = true
	m.syncViewport()
}
