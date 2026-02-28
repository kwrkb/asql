package ui

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/sqly/internal/db"
)

type mode string

const (
	normalMode mode = "NORMAL"
	insertMode mode = "INSERT"
)

var (
	appBackground    = lipgloss.Color("#111827")
	panelBackground  = lipgloss.Color("#0F172A")
	panelBorder      = lipgloss.Color("#334155")
	statusBackground = lipgloss.Color("#1E293B")
	accentColor      = lipgloss.Color("#38BDF8")
	textColor        = lipgloss.Color("#E2E8F0")
	mutedTextColor   = lipgloss.Color("#94A3B8")
	successColor     = lipgloss.Color("#22C55E")
	errorColor       = lipgloss.Color("#F87171")
	keywordColor     = lipgloss.Color("#F59E0B")
)

type queryExecutedMsg struct {
	result db.QueryResult
	err    error
}

type model struct {
	db          db.DBAdapter
	dbPath      string
	mode        mode
	textarea    textarea.Model
	table       table.Model
	viewport    viewport.Model
	width       int
	height      int
	statusText  string
	statusError bool
}

func NewModel(adapter db.DBAdapter, dbPath string) tea.Model {
	input := textarea.New()
	input.Placeholder = "SELECT name FROM sqlite_master WHERE type = 'table';"
	input.Prompt = lipgloss.NewStyle().Foreground(keywordColor).Render("sql> ")
	input.Focus()
	input.ShowLineNumbers = true
	input.SetHeight(8)
	input.CharLimit = 0
	input.SetValue("-- Press Esc for NORMAL mode, Ctrl+Enter to execute.\nSELECT sqlite_version();")
	input.Cursor.Style = lipgloss.NewStyle().Foreground(accentColor)
	input.FocusedStyle.Base = lipgloss.NewStyle().
		Foreground(textColor).
		Background(panelBackground).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(panelBorder).
		Padding(0, 1)
	input.BlurredStyle.Base = input.FocusedStyle.Base.BorderForeground(mutedTextColor)
	input.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(lipgloss.Color("#172033"))
	input.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(mutedTextColor)
	input.FocusedStyle.EndOfBuffer = lipgloss.NewStyle().Foreground(panelBorder)
	input.FocusedStyle.Text = lipgloss.NewStyle().Foreground(textColor)
	input.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(mutedTextColor)
	input.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(keywordColor)
	input.BlurredStyle.Text = input.FocusedStyle.Text
	input.BlurredStyle.Placeholder = input.FocusedStyle.Placeholder
	input.BlurredStyle.Prompt = input.FocusedStyle.Prompt

	tbl := table.New(
		table.WithColumns([]table.Column{{Title: "Result", Width: 30}}),
		table.WithRows([]table.Row{{"No query executed yet"}}),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	tblStyles := table.DefaultStyles()
	tblStyles.Header = tblStyles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(panelBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(accentColor)
	tblStyles.Selected = tblStyles.Selected.
		Foreground(textColor).
		Background(lipgloss.Color("#1D4ED8")).
		Bold(false)
	tblStyles.Cell = tblStyles.Cell.Foreground(textColor)
	tbl.SetStyles(tblStyles)

	vp := viewport.New(0, 0)

	m := model{
		db:         adapter,
		dbPath:     dbPath,
		mode:       insertMode,
		textarea:   input,
		table:      tbl,
		viewport:   vp,
		statusText: "Ready",
	}
	m.syncViewport()
	return m
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case normalMode:
			return m.updateNormal(msg)
		case insertMode:
			return m.updateInsert(msg)
		}
	case queryExecutedMsg:
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
			return m, nil
		}
		m.applyResult(msg.result)
		return m, nil
	}

	var cmd tea.Cmd
	switch m.mode {
	case insertMode:
		m.textarea, cmd = m.textarea.Update(msg)
	case normalMode:
		m.table, cmd = m.table.Update(msg)
	}
	m.syncViewport()
	return m, cmd
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	editor := lipgloss.NewStyle().
		Width(m.width).
		Height(m.editorHeight()).
		Background(appBackground).
		Render(m.textarea.View())

	results := lipgloss.NewStyle().
		Width(m.width).
		Height(m.resultsHeight()).
		Background(appBackground).
		Render(m.viewport.View())

	status := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, editor, results, status)
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "i":
		m.mode = insertMode
		m.textarea.Focus()
		m.setStatus("Insert mode", false)
	case "j":
		m.table.MoveDown(1)
	case "k":
		m.table.MoveUp(1)
	}
	m.syncViewport()
	return m, nil
}

func (m model) updateInsert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = normalMode
		m.textarea.Blur()
		m.setStatus("Normal mode", false)
		m.syncViewport()
		return m, nil
	case "ctrl+enter", "ctrl+j":
		query := strings.TrimSpace(m.textarea.Value())
		m.setStatus("Executing query...", false)
		return m, executeQueryCmd(m.db, query)
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.syncViewport()
	return m, cmd
}

func (m *model) resize() {
	editorHeight := m.editorHeight()
	resultsHeight := m.resultsHeight()

	m.textarea.SetWidth(max(m.width-4, 20))
	m.textarea.SetHeight(max(editorHeight-2, 5))

	m.table.SetHeight(max(resultsHeight-4, 3))
	m.viewport.Width = m.width
	m.viewport.Height = resultsHeight
	m.syncViewport()
}

func (m *model) editorHeight() int {
	available := max(m.height-1, 6)
	return max(int(float64(available)*0.3), 7)
}

func (m *model) resultsHeight() int {
	available := max(m.height-1, 6)
	return max(available-m.editorHeight(), 4)
}

func (m *model) syncViewport() {
	panel := lipgloss.NewStyle().
		Width(max(m.width, 0)).
		Height(max(m.resultsHeight(), 0)).
		Background(panelBackground).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(panelBorder).
		Padding(0, 1).
		Render(m.table.View())
	m.viewport.SetContent(panel)
}

func (m *model) applyResult(result db.QueryResult) {
	columns := make([]table.Column, 0, len(result.Columns))
	rows := make([]table.Row, 0, len(result.Rows))

	if len(result.Columns) == 0 {
		columns = []table.Column{{Title: "Result", Width: max(m.width-6, 20)}}
		rows = []table.Row{{result.Message}}
	} else {
		for i, title := range result.Columns {
			width := columnWidth(title, result.Rows, i)
			columns = append(columns, table.Column{Title: title, Width: width})
		}
		for _, row := range result.Rows {
			rows = append(rows, table.Row(row))
		}
		if len(rows) == 0 {
			rows = []table.Row{{"(no rows)"}}
		}
	}

	m.table.SetColumns(columns)
	m.table.SetRows(rows)
	m.setStatus(result.Message, false)
	m.syncViewport()
}

func (m *model) setStatus(text string, isError bool) {
	m.statusText = text
	m.statusError = isError
}

func (m model) renderStatusBar() string {
	modeStyle := lipgloss.NewStyle().
		Foreground(panelBackground).
		Background(accentColor).
		Padding(0, 1).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Background(statusBackground).
		Padding(0, 1)
	if m.statusError {
		messageStyle = messageStyle.Foreground(errorColor)
	} else if strings.TrimSpace(m.statusText) != "" {
		messageStyle = messageStyle.Foreground(successColor)
	}

	pathStyle := lipgloss.NewStyle().
		Foreground(mutedTextColor).
		Background(statusBackground).
		Padding(0, 1)

	left := modeStyle.Render(string(m.mode))
	center := pathStyle.Render(m.dbPath)
	right := messageStyle.Render(m.statusText)

	bar := lipgloss.JoinHorizontal(lipgloss.Left, left, center, right)
	return lipgloss.NewStyle().
		Width(m.width).
		Background(statusBackground).
		Render(bar)
}

func executeQueryCmd(adapter db.DBAdapter, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := adapter.Query(ctx, query)
		return queryExecutedMsg{result: result, err: err}
	}
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
