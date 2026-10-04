package ui

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/export"
)

var exportOptions = []string{
	"Copy as CSV",
	"Copy as JSON",
	"Copy as Markdown",
	"Save to File (CSV)",
}

func (m model) updateExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = normalMode
		m.setStatus("Normal mode", false)
		return m, nil
	case tea.KeyRunes:
		if msg.Alt {
			break
		}
		switch string(msg.Runes) {
		case "j":
			moveCursor(&m.exportSt.cursor, len(exportOptions), 1)
		case "k":
			moveCursor(&m.exportSt.cursor, len(exportOptions), -1)
		}
	case tea.KeyDown:
		moveCursor(&m.exportSt.cursor, len(exportOptions), 1)
	case tea.KeyUp:
		moveCursor(&m.exportSt.cursor, len(exportOptions), -1)
	case tea.KeyEnter:
		m.executeExport()
		return m, nil
	}
	return m, nil
}

// exportSource returns what the user is looking at: the focused compare pane,
// in its on-screen sort order. The result's own rows would ignore both — an
// `s` sort, and a pinned pane that has focus. The rows are re-sorted here
// rather than read from displayRows, which holds a "(no rows)" sentinel for
// an empty result that must not reach the file.
func (m *model) exportSource() (headers []string, rows [][]string) {
	result, col, dir := m.exportPane()
	return result.Columns, sortedRows(result.Rows, col, dir)
}

// exportPane picks the pane exportSource reads, without sorting it — enough
// to ask whether there is anything to export.
func (m *model) exportPane() (result db.QueryResult, sortCol int, sortDir sortOrder) {
	if p := m.pinned; p != nil && m.comparePane == 0 {
		return p.result, p.sortCol, p.sortDir
	}
	return m.lastResult, m.sortCol, m.sortDir
}

func (m *model) executeExport() {
	headers, rows := m.exportSource()

	defer func() { m.mode = normalMode }()

	switch m.exportSt.cursor {
	case 0: // CSV to clipboard
		content, err := export.FormatCSV(headers, rows)
		if err != nil {
			m.setStatus(fmt.Sprintf("Export failed: %v", err), true)
			return
		}
		if err := clipboard.WriteAll(content); err != nil {
			m.setStatus(fmt.Sprintf("Clipboard failed: %v", err), true)
			return
		}
		m.setStatus("Copied as CSV to clipboard!", false)

	case 1: // JSON to clipboard
		content, err := export.FormatJSON(headers, rows)
		if err != nil {
			m.setStatus(fmt.Sprintf("Export failed: %v", err), true)
			return
		}
		if err := clipboard.WriteAll(content); err != nil {
			m.setStatus(fmt.Sprintf("Clipboard failed: %v", err), true)
			return
		}
		m.setStatus("Copied as JSON to clipboard!", false)

	case 2: // Markdown to clipboard
		content := export.FormatMarkdown(headers, rows)
		if err := clipboard.WriteAll(content); err != nil {
			m.setStatus(fmt.Sprintf("Clipboard failed: %v", err), true)
			return
		}
		m.setStatus("Copied as Markdown to clipboard!", false)

	case 3: // Save CSV file
		filename, err := export.SaveCSVFile(headers, rows)
		if err != nil {
			m.setStatus(fmt.Sprintf("Export failed: %v", err), true)
			return
		}
		m.setStatus(fmt.Sprintf("Saved to %s", filename), false)
	}
}

func (m model) renderWithExportOverlay(background string) string {
	modalWidth := calcModalWidth(m.width, 40)
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
	for i, opt := range exportOptions {
		if i == m.exportSt.cursor {
			items.WriteString(selectedStyle.Render(opt))
		} else {
			items.WriteString(itemStyle.Render(opt))
		}
		if i < len(exportOptions)-1 {
			items.WriteByte('\n')
		}
	}

	content := titleStyle.Render("Export Results") + "\n" + items.String()
	modal := boxStyle.Render(content)

	return overlayModal(m.width, background, modal)
}
