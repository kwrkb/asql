package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/sqly/internal/ai"
	"github.com/kwrkb/sqly/internal/db"
)

type mode string

const (
	normalMode  mode = "NORMAL"
	insertMode  mode = "INSERT"
	sidebarMode mode = "SIDEBAR"
	aiMode      mode = "AI"
	exportMode  mode = "EXPORT"

	queryTimeout = 5 * time.Second
	sidebarWidth = 25
	minWidthForSidebar = 60
)

const (
	appBackground    lipgloss.Color = "#111827"
	panelBackground  lipgloss.Color = "#0F172A"
	panelBorder      lipgloss.Color = "#334155"
	statusBackground lipgloss.Color = "#1E293B"
	accentColor      lipgloss.Color = "#38BDF8"
	textColor        lipgloss.Color = "#E2E8F0"
	mutedTextColor   lipgloss.Color = "#94A3B8"
	successColor     lipgloss.Color = "#22C55E"
	errorColor       lipgloss.Color = "#F87171"
	keywordColor     lipgloss.Color = "#F59E0B"
)

type queryExecutedMsg struct {
	result db.QueryResult
	err    error
}

type tablesLoadedMsg struct {
	tables []string
	err    error
}

type aiResponseMsg struct {
	sql string
	err error
}

type model struct {
	db           db.DBAdapter
	dbPath       string
	mode         mode
	textarea     textarea.Model
	table        table.Model
	viewport     viewport.Model
	width         int
	height        int
	statusText    string
	statusError   bool
	sidebarOpen   bool
	sidebarTables []string
	sidebarCursor int
	aiEnabled     bool
	aiClient      *ai.Client
	aiInput       textinput.Model
	aiSpinner     spinner.Model
	aiLoading     bool
	aiError       string
	lastResult    db.QueryResult
	exportCursor  int
	modeStyle     lipgloss.Style
	messageStyle  lipgloss.Style
	pathStyle     lipgloss.Style
}

func NewModel(adapter db.DBAdapter, dbPath string, aiClient *ai.Client) tea.Model {
	input := textarea.New()
	input.Placeholder = "SELECT name FROM sqlite_master WHERE type = 'table';"
	input.Prompt = lipgloss.NewStyle().Foreground(keywordColor).Render("sql> ")
	input.Focus()
	input.ShowLineNumbers = true
	input.SetHeight(8)
	input.CharLimit = 0
	input.SetValue("-- Press Esc for NORMAL mode, Ctrl+Enter (or Ctrl+J) to execute.\nSELECT sqlite_version();")
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

	aiIn := textinput.New()
	aiIn.Placeholder = "Describe what you want to query..."
	aiIn.CharLimit = 500
	aiIn.Width = 50

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(accentColor)

	m := model{
		db:         adapter,
		dbPath:     dbPath,
		mode:       insertMode,
		textarea:   input,
		table:      tbl,
		viewport:   vp,
		statusText: "Ready",
		aiEnabled:  aiClient != nil,
		aiClient:   aiClient,
		aiInput:    aiIn,
		aiSpinner:  sp,
	}
	m.modeStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(accentColor).Foreground(panelBackground)
	m.messageStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(textColor).Background(statusBackground)
	m.pathStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(mutedTextColor).Background(statusBackground)
	m.syncViewport()
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, loadTablesCmd(m.db))
}

func loadTablesCmd(adapter db.DBAdapter) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
		defer cancel()
		tables, err := adapter.Tables(ctx)
		return tablesLoadedMsg{tables: tables, err: err}
	}
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
		case sidebarMode:
			return m.updateSidebar(msg)
		case aiMode:
			return m.updateAI(msg)
		case exportMode:
			return m.updateExport(msg)
		}
	case aiResponseMsg:
		m.aiLoading = false
		if msg.err != nil {
			m.aiError = msg.err.Error()
			return m, nil
		}
		m.textarea.SetValue(msg.sql)
		m.mode = insertMode
		m.textarea.Focus()
		m.setStatus("AI generated SQL — review before executing", false)
		return m, nil
	case spinner.TickMsg:
		if m.aiLoading {
			var cmd tea.Cmd
			m.aiSpinner, cmd = m.aiSpinner.Update(msg)
			return m, cmd
		}
		return m, nil
	case tablesLoadedMsg:
		if msg.err == nil {
			m.sidebarTables = msg.tables
			if m.sidebarCursor >= len(msg.tables) {
				m.sidebarCursor = max(len(msg.tables)-1, 0)
			}
		}
		return m, nil
	case queryExecutedMsg:
		if msg.err != nil {
			errMsg := msg.err.Error()
			if errors.Is(msg.err, context.DeadlineExceeded) {
				errMsg = fmt.Sprintf("query timed out after %s", queryTimeout)
			}
			m.setStatus(errMsg, true)
			return m, nil
		}
		m.lastResult = msg.result
		m.applyResult(msg.result)
		return m, loadTablesCmd(m.db)
	}

	var cmd tea.Cmd
	switch m.mode {
	case insertMode:
		m.textarea, cmd = m.textarea.Update(msg)
	case normalMode:
		m.table, cmd = m.table.Update(msg)
	case sidebarMode:
		// no passthrough needed
	}
	m.syncViewport()
	return m, cmd
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	contentWidth := m.contentWidth()

	editor := lipgloss.NewStyle().
		Width(contentWidth).
		Height(m.editorHeight()).
		Background(appBackground).
		Render(m.textarea.View())

	results := lipgloss.NewStyle().
		Width(contentWidth).
		Height(m.resultsHeight()).
		Background(appBackground).
		Render(m.viewport.View())

	main := lipgloss.JoinVertical(lipgloss.Left, editor, results)

	if m.sidebarOpen {
		sidebar := m.renderSidebar()
		main = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)
	}

	status := m.renderStatusBar()

	view := lipgloss.JoinVertical(lipgloss.Left, main, status)

	if m.mode == aiMode {
		view = m.renderWithAIOverlay(view)
	}

	if m.mode == exportMode {
		view = m.renderWithExportOverlay(view)
	}

	return view
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "i":
		m.mode = insertMode
		m.textarea.Focus()
		m.setStatus("Insert mode", false)
	case "t":
		if m.width >= minWidthForSidebar {
			m.sidebarOpen = true
			m.mode = sidebarMode
			m.textarea.Blur()
			m.sidebarCursor = 0
			m.setStatus("Sidebar", false)
			m.resize()
		} else {
			m.setStatus("Terminal too narrow for sidebar", true)
		}
	case "e":
		if len(m.lastResult.Columns) == 0 {
			m.setStatus("No query results to export", true)
		} else {
			m.mode = exportMode
			m.exportCursor = 0
			m.setStatus("Export mode", false)
		}
	case "ctrl+k":
		if m.aiEnabled {
			m.mode = aiMode
			m.aiInput.Reset()
			m.aiInput.Focus()
			m.aiError = ""
			m.aiLoading = false
			m.setStatus("AI mode", false)
			return m, textinput.Blink
		}
		m.setStatus("AI not configured", true)
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

func (m model) updateSidebar(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "t":
		m.sidebarOpen = false
		m.mode = normalMode
		m.setStatus("Normal mode", false)
		m.resize()
	case "j", "down":
		if len(m.sidebarTables) > 0 {
			m.sidebarCursor = min(m.sidebarCursor+1, len(m.sidebarTables)-1)
		}
	case "k", "up":
		if m.sidebarCursor > 0 {
			m.sidebarCursor--
		}
	case "enter":
		if len(m.sidebarTables) > 0 {
			name := m.sidebarTables[m.sidebarCursor]
			quoted := `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
			query := fmt.Sprintf("SELECT * FROM %s LIMIT 100;", quoted)
			m.textarea.SetValue(query)
			m.sidebarOpen = false
			m.mode = insertMode
			m.textarea.Focus()
			m.setStatus("Insert mode", false)
			m.resize()
		}
	}
	m.syncViewport()
	return m, nil
}

func (m model) updateAI(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.aiLoading {
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.mode = normalMode
		m.aiError = ""
		m.setStatus("Normal mode", false)
		return m, nil
	case "enter":
		prompt := strings.TrimSpace(m.aiInput.Value())
		if prompt == "" {
			return m, nil
		}
		m.aiLoading = true
		m.aiError = ""
		return m, tea.Batch(m.aiSpinner.Tick, generateSQLCmd(m.aiClient, m.db, prompt))
	}
	var cmd tea.Cmd
	m.aiInput, cmd = m.aiInput.Update(msg)
	return m, cmd
}

func generateSQLCmd(client *ai.Client, adapter db.DBAdapter, prompt string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		schema, err := adapter.Schema(ctx)
		if err != nil {
			return aiResponseMsg{err: fmt.Errorf("fetching schema: %w", err)}
		}

		sql, err := client.GenerateSQL(ctx, schema, prompt)
		return aiResponseMsg{sql: sql, err: err}
	}
}

func (m model) renderWithAIOverlay(background string) string {
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

	var content string
	if m.aiLoading {
		content = titleStyle.Render("AI Generating SQL...") + "\n" + m.aiSpinner.View() + " Thinking..."
	} else {
		content = titleStyle.Render("AI Assistant (Text-to-SQL)") + "\n" + m.aiInput.View()
		if m.aiError != "" {
			errStyle := lipgloss.NewStyle().Foreground(errorColor).MarginTop(1)
			content += "\n" + errStyle.Render(m.aiError)
		}
	}

	modal := boxStyle.Render(content)

	bgH := lipgloss.Height(background)

	return lipgloss.Place(m.width, bgH, lipgloss.Center, lipgloss.Center, modal,
		lipgloss.WithWhitespaceBackground(appBackground))
}

func (m *model) contentWidth() int {
	if m.sidebarOpen {
		return max(m.width-sidebarWidth, 20)
	}
	return m.width
}

func (m *model) resize() {
	contentWidth := m.contentWidth()
	editorHeight := m.editorHeight()
	resultsHeight := m.resultsHeight()

	// Auto-close sidebar if terminal too narrow
	if m.sidebarOpen && m.width < minWidthForSidebar {
		m.sidebarOpen = false
		if m.mode == sidebarMode {
			m.mode = normalMode
		}
		contentWidth = m.width
	}

	m.textarea.SetWidth(max(contentWidth-4, 20))
	m.textarea.SetHeight(max(editorHeight-2, 5))

	m.table.SetHeight(max(resultsHeight-4, 3))
	m.viewport.Width = contentWidth
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
			sentinel := make(table.Row, len(columns))
			sentinel[0] = "(no rows)"
			rows = []table.Row{sentinel}
		}
	}

	m.table.SetRows([]table.Row{})
	m.table.SetColumns(columns)
	m.table.SetRows(rows)
	m.setStatus(result.Message, false)
	m.syncViewport()
}

func (m *model) setStatus(text string, isError bool) {
	m.statusText = text
	m.statusError = isError
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
	if maxVisible > 0 && m.sidebarCursor >= maxVisible {
		scrollOffset = m.sidebarCursor - maxVisible + 1
	}

	for i := scrollOffset; i < len(m.sidebarTables); i++ {
		if lines >= height-1 {
			break
		}
		name := m.sidebarTables[i]
		if i == m.sidebarCursor {
			b.WriteString(selectedStyle.Render(name))
		} else {
			b.WriteString(itemStyle.Render(name))
		}
		b.WriteByte('\n')
		lines++
	}

	if len(m.sidebarTables) == 0 {
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

func (m model) renderStatusBar() string {
	modeStr := m.modeStyle.Render(string(m.mode))

	msgStyle := m.messageStyle
	if m.statusError {
		msgStyle = msgStyle.Foreground(errorColor)
	} else if strings.TrimSpace(m.statusText) != "" {
		msgStyle = msgStyle.Foreground(successColor)
	}

	var hints string
	switch m.mode {
	case normalMode:
		if m.aiEnabled {
			hints = "t:tables i:insert e:export C-k:AI q:quit"
		} else {
			hints = "t:tables i:insert e:export q:quit"
		}
	case insertMode:
		hints = "C-Enter/C-j:exec Esc:normal"
	case sidebarMode:
		hints = "j/k:nav Enter:select Esc:close"
	case aiMode:
		hints = "Enter:generate Esc:cancel"
	case exportMode:
		hints = "j/k:nav Enter:select Esc:cancel"
	}
	hintStyle := lipgloss.NewStyle().Foreground(mutedTextColor).Background(statusBackground).Padding(0, 1)

	left := modeStr
	center := m.pathStyle.Render(m.dbPath)
	middle := msgStyle.Render(m.statusText)
	right := hintStyle.Render(hints)

	leftPart := lipgloss.JoinHorizontal(lipgloss.Left, left, center, middle)
	gap := max(m.width-lipgloss.Width(leftPart)-lipgloss.Width(right), 0)
	bar := leftPart + strings.Repeat(" ", gap) + right

	return lipgloss.NewStyle().
		Width(m.width).
		Background(statusBackground).
		Render(bar)
}

func executeQueryCmd(adapter db.DBAdapter, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
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
