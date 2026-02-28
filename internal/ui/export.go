package ui

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/sqly/internal/export"
)

var exportOptions = []string{
	"Copy as CSV",
	"Copy as JSON",
	"Copy as Markdown",
	"Save to File (CSV)",
}

func (m model) updateExport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = normalMode
		m.setStatus("Normal mode", false)
		return m, nil
	case "j", "down":
		if m.exportCursor < len(exportOptions)-1 {
			m.exportCursor++
		}
	case "k", "up":
		if m.exportCursor > 0 {
			m.exportCursor--
		}
	case "enter":
		m.executeExport()
		return m, nil
	}
	return m, nil
}

func (m *model) executeExport() {
	headers := m.lastResult.Columns
	rows := m.lastResult.Rows

	defer func() { m.mode = normalMode }()

	switch m.exportCursor {
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
	modalWidth := min(m.width-4, 40)
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
	for i, opt := range exportOptions {
		if i == m.exportCursor {
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

	bgH := lipgloss.Height(background)

	return lipgloss.Place(m.width, bgH, lipgloss.Center, lipgloss.Center, modal,
		lipgloss.WithWhitespaceBackground(appBackground))
}
