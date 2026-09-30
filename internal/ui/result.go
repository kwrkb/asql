package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/dbutil"
	"github.com/kwrkb/asql/internal/ui/table"
)

// adjustColOffset ensures colCursor is within the visible column window.
// It marks the viewport dirty only when the offset actually moves: it runs on
// every syncViewport, so an unconditional mark would defeat the rebuild-skip
// cache (lastVisStart/lastVisEnd) entirely.
func (m *model) adjustColOffset() {
	prev := m.colOffset
	if m.colCursor < m.colOffset {
		m.colOffset = m.colCursor
	}
	_, visEnd := m.visibleColumnRange()
	for m.colCursor >= visEnd && m.colOffset < len(m.cachedColWidths)-1 {
		m.colOffset++
		_, visEnd = m.visibleColumnRange()
	}
	if m.colOffset != prev {
		m.viewportDirty = true
	}
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
		m.viewport.SetContent(framedPanel(m.table.View(), m.contentWidth(), m.resultsHeight(), panelBorder))
		return
	}

	// Ensure colCursor stays within the visible window (e.g. after resize)
	m.adjustColOffset()

	visStart, visEnd := m.visibleColumnRange()

	// Rebuild columns/rows only when the visible window, the column cursor, or
	// the header-highlight state changes. For row-only navigation (j/k) we skip
	// the expensive rebuild.
	highlight := m.mode == normalMode && (m.pinned == nil || m.comparePane == 1)
	rebuildNeeded := visStart != m.lastVisStart || visEnd != m.lastVisEnd ||
		m.colCursor != m.lastColCursor || highlight != m.lastHighlight || m.viewportDirty
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
		m.lastColCursor = m.colCursor
		m.lastHighlight = highlight
		m.viewportDirty = false
	}

	m.viewport.SetContent(framedPanel(m.table.View(), m.contentWidth(), m.resultsHeight(), panelBorder))
}

// framedPanel renders content in a bordered result panel that occupies w×h
// cells, border included. lipgloss v1 sizes Width and Height without the
// border, so passing w and h straight through renders a panel two cells wider
// and taller than asked: the viewport then clips its right and bottom edges,
// and in compare mode the second pane runs off the screen.
func framedPanel(content string, w, h int, border lipgloss.TerminalColor) string {
	style := lipgloss.NewStyle().
		Background(panelBackground).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1)
	return style.
		Width(max(w-style.GetHorizontalBorderSize(), 0)).
		Height(max(h-style.GetVerticalBorderSize(), 0)).
		Render(content)
}

func (m *model) applyResult(result db.QueryResult) {
	m.lastResult = result
	m.cachedCellWidths = make([]int, len(result.Columns))
	for i := range result.Columns {
		m.cachedCellWidths[i] = cellsWidth(result.Rows, i)
	}
	m.applyResultWithSort(result)
}

func columnWidth(title string, rows [][]string, idx int) int {
	return clampColumnWidth(max(lipgloss.Width(title), cellsWidth(rows, idx)))
}

// cellsWidth is the widest cell of column idx. Sorting reorders the rows
// without changing it, so applyResult measures it once per result and a sort
// reuses it rather than re-measuring every cell.
func cellsWidth(rows [][]string, idx int) int {
	width := 0
	for _, row := range rows {
		if idx >= len(row) {
			continue
		}
		width = max(width, lipgloss.Width(row[idx]))
	}
	return width
}

func clampColumnWidth(width int) int {
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
	// Sorting is display-only: m.lastResult keeps its original row order, so its
	// Rows and Kinds stay aligned for consumers that need them (bring, stats).
	// This local copy is reordered, so drop its Kinds rather than leave them
	// pointing at the wrong rows — see the invariant on db.QueryResult.Kinds.
	result.Kinds = nil
	m.applyResultWithSort(result)
	m.table.GotoTop()
}

// applyResultWithSort computes column widths, saves displayRows, and delegates rendering to syncViewport.
func (m *model) applyResultWithSort(result db.QueryResult) {
	if m.pinned != nil {
		// The pinned pane's diff highlighting compares against the active rows,
		// which are about to change. This sits above the message-only return
		// below, not after it: an UPDATE or DELETE run while compare mode is
		// open leaves the active side with no rows at all, and the pinned pane
		// would otherwise keep the highlighting computed against the result
		// before it — now that a rebuild can be skipped, forever.
		m.pinned.viewportDirty = true
	}
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
	if len(m.cachedCellWidths) != len(result.Columns) {
		// lastResult was set without applyResult; measure it now.
		m.cachedCellWidths = make([]int, len(result.Columns))
		for i := range result.Columns {
			m.cachedCellWidths[i] = cellsWidth(result.Rows, i)
		}
	}
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
		// Only the header changes under a sort (its ▲/▼ indicator); the
		// cells are the same set in a new order.
		cells := 0
		if i < len(m.cachedCellWidths) {
			cells = m.cachedCellWidths[i]
		}
		m.cachedColWidths[i] = clampColumnWidth(max(lipgloss.Width(header), cells))
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
