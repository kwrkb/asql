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

	"github.com/kwrkb/asql/internal/ai"
	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/dbutil"
	"github.com/kwrkb/asql/internal/profile"
	"github.com/kwrkb/asql/internal/snippet"
)

type mode string

const (
	normalMode        mode = "NORMAL"
	insertMode        mode = "INSERT"
	sidebarMode       mode = "SIDEBAR"
	aiMode            mode = "AI"
	exportMode        mode = "EXPORT"
	detailMode        mode = "DETAIL"
	snippetMode       mode = "SNIPPET"
	historySearchMode mode = "SEARCH"
	profileMode       mode = "PROFILE"

	queryTimeout       = 5 * time.Second
	sidebarWidth       = 25
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

var typeStyle = lipgloss.NewStyle().Foreground(mutedTextColor)

type queryExecutedMsg struct {
	seq    uint64
	result db.QueryResult
	err    error
}

type tablesLoadedMsg struct {
	tables []string
	err    error
}

type aiResponseMsg struct {
	seq uint64
	sql string
	err error
}

type connSwitchedMsg struct {
	err       error
	reExecute bool
}

type columnsLoadedMsg struct {
	table   string
	columns []string
	err     error
	connGen uint64 // connection generation when fetch was initiated
}

type model struct {
	// Connection
	connMgr  *connManager
	connName string // display name of initial connection
	dbPath   string
	rawDSN   string // unmasked DSN for profile save

	// Core UI
	mode     mode
	textarea textarea.Model
	table    table.Model
	viewport viewport.Model
	width    int
	height   int

	// Status bar
	statusText  string
	statusError bool

	// Connection generation (incremented on each connection switch)
	connGen uint64

	// Query execution
	queryCancel  context.CancelFunc
	querySeq     uint64
	lastResult   db.QueryResult
	queryHistory []string // executed queries (newest at end)
	historyIdx   int      // -1 = new input, 0..n = history position
	historyDraft string   // input saved before navigating history

	// Result table
	sortCol         int
	sortDir         sortOrder
	colCursor       int         // column cursor in NORMAL mode
	colOffset       int         // first visible column index for horizontal windowing
	cachedColWidths []int       // cached column widths (recomputed only when result changes)
	displayRows     []table.Row // sorted rows for windowing source
	lastVisStart    int         // cached visible range start for rebuild optimization
	lastVisEnd      int         // cached visible range end for rebuild optimization
	viewportDirty   bool        // forces column/row rebuild on next syncViewport

	// Compare
	pinned      *pinnedPane // nil = side-by-side OFF
	comparePane int         // 0=left(pinned), 1=right(active)

	// Styles
	modeStyle    lipgloss.Style
	messageStyle lipgloss.Style
	pathStyle    lipgloss.Style

	// Mode-specific state
	detail     detailState
	exportSt   exportState
	aiSt       aiState
	snippetSt  snippetState
	profileSt  profileState
	histSearch histSearchState
	completion completionState
	sidebar    sidebarState
}

// CloseAll closes all database connections managed by this model.
// Call this after tea.Program exits to avoid connection leaks.
func (m model) CloseAll() {
	if m.connMgr != nil {
		m.connMgr.CloseAll()
	}
}

func NewModel(adapter db.DBAdapter, dbPath string, rawDSN string, connName string, aiClient *ai.Client, snippets []snippet.Snippet, profiles []profile.Profile) model {
	input := textarea.New()

	placeholder := db.Placeholder(adapter.Type())
	initialQuery := db.InitialQuery(adapter.Type())

	input.Placeholder = placeholder
	input.Prompt = lipgloss.NewStyle().Foreground(keywordColor).Render("sql> ")
	input.Focus()
	input.ShowLineNumbers = true
	input.SetHeight(8)
	input.CharLimit = 0
	input.SetValue("-- Press Esc for NORMAL mode, Ctrl+Enter (or Ctrl+J) to execute.\n" + initialQuery)
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

	snippetIn := textinput.New()
	snippetIn.Placeholder = "Snippet name..."
	snippetIn.CharLimit = 100
	snippetIn.Width = 30

	profileIn := textinput.New()
	profileIn.Placeholder = "Profile name..."
	profileIn.CharLimit = 100
	profileIn.Width = 30

	histSearchIn := textinput.New()
	histSearchIn.Placeholder = "Search history..."
	histSearchIn.CharLimit = 200
	histSearchIn.Width = 40

	cm := newConnManager(connName, rawDSN, adapter)

	m := model{
		connMgr:    cm,
		connName:   connName,
		dbPath:     dbPath,
		rawDSN:     rawDSN,
		mode:       insertMode,
		textarea:   input,
		table:      tbl,
		viewport:   vp,
		statusText: "Ready",
		historyIdx: -1,
		aiSt: aiState{
			enabled: aiClient != nil,
			client:  aiClient,
			input:   aiIn,
			spinner: sp,
		},
		snippetSt: snippetState{
			items: snippets,
			input: snippetIn,
		},
		profileSt: profileState{
			items: profiles,
			input: profileIn,
		},
		histSearch: histSearchState{
			input: histSearchIn,
		},
	}
	m.modeStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).Background(accentColor).Foreground(panelBackground)
	m.messageStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(textColor).Background(statusBackground)
	m.pathStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(mutedTextColor).Background(statusBackground)
	m.syncViewport()
	return m
}

func (m *model) activeDB() db.DBAdapter {
	return m.connMgr.Active()
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, loadTablesCmd(m.connMgr.Active()))
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
		if msg.Type == tea.KeyCtrlC {
			if m.queryCancel != nil {
				m.queryCancel()
				m.queryCancel = nil
				m.aiSt.loading = false
				m.mode = normalMode
				m.textarea.Blur()
				m.setStatus("Cancelled", false)
				return m, nil
			}
			return m, tea.Quit
		}
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
		case detailMode:
			return m.updateDetail(msg)
		case snippetMode:
			return m.updateSnippet(msg)
		case profileMode:
			return m.updateProfile(msg)
		case historySearchMode:
			return m.updateHistorySearch(msg)
		}
	case aiResponseMsg:
		if msg.seq != m.querySeq {
			return m, nil
		}
		m.queryCancel = nil
		m.aiSt.loading = false
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				return m, nil
			}
			m.aiSt.err = msg.err.Error()
			return m, nil
		}
		m.textarea.SetValue(msg.sql)
		m.mode = insertMode
		m.textarea.Focus()
		m.setStatus("AI generated SQL — review before executing", false)
		return m, nil
	case spinner.TickMsg:
		if m.aiSt.loading {
			var cmd tea.Cmd
			m.aiSt.spinner, cmd = m.aiSt.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	case connSwitchedMsg:
		if msg.err != nil {
			m.setStatus(fmt.Sprintf("Connection failed: %v", msg.err), true)
			m.mode = normalMode
			m.textarea.Blur()
			return m, nil
		}
		// Cancel any in-flight query from the previous connection
		if m.queryCancel != nil {
			m.queryCancel()
			m.queryCancel = nil
		}
		m.querySeq++ // invalidate stale query results
		m.connGen++  // invalidate stale column fetches
		// Update dbPath to reflect new connection
		m.dbPath = db.MaskDSN(m.connMgr.ActiveDSN())
		m.rawDSN = m.connMgr.ActiveDSN()
		m.completion.colCache = nil
		m.completion.colOrder = nil
		m.sidebar.tables = nil
		m.setStatus(fmt.Sprintf("Connected to %s", sanitize(m.connMgr.ActiveName())), false)
		m.mode = normalMode
		m.textarea.Blur()
		if msg.reExecute {
			query := strings.TrimSpace(m.textarea.Value())
			if query != "" {
				return m, tea.Batch(loadTablesCmd(m.connMgr.Active()), m.prepareAndExecuteQuery(query))
			}
		}
		return m, loadTablesCmd(m.connMgr.Active())
	case tablesLoadedMsg:
		if msg.err != nil {
			m.setStatus("Failed to load tables: "+msg.err.Error(), true)
			return m, nil
		}
		m.sidebar.tables = msg.tables
		m.completion.colCache = nil
		m.completion.colOrder = nil // invalidate column cache
		if m.sidebar.cursor >= len(msg.tables) {
			m.sidebar.cursor = max(len(msg.tables)-1, 0)
		}
		return m, nil
	case columnsLoadedMsg:
		if msg.connGen != m.connGen {
			return m, nil // stale fetch from previous connection
		}
		if msg.err == nil && msg.columns != nil {
			if m.completion.colCache == nil {
				m.completion.colCache = make(map[string][]string)
			}
			const maxColCacheSize = 64
			if len(m.completion.colCache) >= maxColCacheSize && len(m.completion.colOrder) > 0 {
				evict := m.completion.colOrder[0]
				m.completion.colOrder = m.completion.colOrder[1:]
				delete(m.completion.colCache, evict)
			}
			m.completion.colCache[msg.table] = msg.columns
			m.completion.colOrder = append(m.completion.colOrder, msg.table)
			// Re-trigger completion only if cursor context still matches
			if m.mode == insertMode && m.completion.pendingPrefix != "" {
				prefix, _ := wordAtCursor(m.textarea.Value(), m.textarea.Line(), m.textarea.LineInfo().CharOffset)
				if prefix == m.completion.pendingPrefix {
					return m, m.triggerCompletion()
				}
				m.completion.pendingPrefix = ""
			}
		}
		return m, nil
	case queryExecutedMsg:
		if msg.seq != m.querySeq {
			return m, nil
		}
		m.queryCancel = nil
		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				return m, nil
			}
			errMsg := msg.err.Error()
			if errors.Is(msg.err, context.DeadlineExceeded) {
				errMsg = fmt.Sprintf("query timed out after %s", queryTimeout)
			}
			m.setStatus(errMsg, true)
			return m, nil
		}
		m.lastResult = msg.result
		m.sortDir = sortNone
		m.sortCol = 0
		m.colCursor = 0
		m.colOffset = 0
		m.applyResult(msg.result)
		if m.pinned != nil {
			m.setStatus(m.compareStatusSummary(), false)
		}
		return m, loadTablesCmd(m.activeDB())
	}

	var cmd tea.Cmd
	switch m.mode {
	case insertMode:
		m.textarea, cmd = m.textarea.Update(msg)
	case normalMode:
		// Route passthrough events to the focused pane in compare mode
		if m.pinned != nil && m.comparePane == 0 {
			m.pinned.table, cmd = m.pinned.table.Update(msg)
		} else {
			m.table, cmd = m.table.Update(msg)
		}
	case historySearchMode:
		m.histSearch.input, cmd = m.histSearch.input.Update(msg)
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

	fullWidth := m.fullContentWidth()

	editorView := m.textarea.View()
	if m.completion.active && len(m.completion.items) > 0 {
		popup := m.renderCompletionPopup()
		editorView = editorView + "\n" + popup
	}

	editor := lipgloss.NewStyle().
		Width(fullWidth).
		Height(m.editorHeight()).
		Background(appBackground).
		Render(editorView)

	var results string
	if m.pinned != nil {
		results = m.renderCompareView()
	} else {
		results = lipgloss.NewStyle().
			Width(fullWidth).
			Height(m.resultsHeight()).
			Background(appBackground).
			Render(m.viewport.View())
	}

	main := lipgloss.JoinVertical(lipgloss.Left, editor, results)

	if m.sidebar.open {
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

	if m.mode == detailMode {
		view = m.renderWithDetailOverlay(view)
	}

	if m.mode == snippetMode {
		view = m.renderWithSnippetOverlay(view)
	}

	if m.mode == historySearchMode {
		view = m.renderWithHistorySearchOverlay(view)
	}

	if m.mode == profileMode {
		view = m.renderWithProfileOverlay(view)
	}

	return view
}

func (m *model) contentWidth() int {
	w := m.width
	if m.sidebar.open {
		w = max(w-sidebarWidth, 20)
	}
	if m.pinned != nil {
		return w / 2
	}
	return w
}

// fullContentWidth returns the total width available for content (without compare split).
func (m *model) fullContentWidth() int {
	if m.sidebar.open {
		return max(m.width-sidebarWidth, 20)
	}
	return m.width
}

func (m *model) resize() {
	editorHeight := m.editorHeight()
	resultsHeight := m.resultsHeight()

	// Auto-close sidebar if terminal too narrow
	if m.sidebar.open && m.width < minWidthForSidebar {
		m.sidebar.open = false
		if m.mode == sidebarMode {
			m.mode = normalMode
		}
	}

	// Auto-close compare if terminal too narrow
	if m.pinned != nil && m.fullContentWidth() < minWidthForCompare {
		m.pinned = nil
		m.comparePane = 0
		m.table.SetStyles(focusedTableStyles())
		m.setStatus("Compare closed (terminal too narrow)", false)
	}

	fullWidth := m.fullContentWidth()
	contentWidth := m.contentWidth() // half if compare active

	m.textarea.SetWidth(max(fullWidth-4, 20))
	m.textarea.SetHeight(max(editorHeight-2, 5))

	if m.pinned != nil {
		compareHeight := resultsHeight - 1 // subtract label row
		m.table.SetHeight(max(compareHeight-4, 3))
		m.pinned.table.SetHeight(max(compareHeight-4, 3))
		m.pinned.viewportDirty = true
	} else {
		m.table.SetHeight(max(resultsHeight-4, 3))
	}
	m.viewport.Width = contentWidth
	m.viewport.Height = resultsHeight

	m.viewportDirty = true
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
	sum := 0
	for i := start; i < len(m.cachedColWidths); i++ {
		w := m.cachedColWidths[i] + 1 // column width + cell gap
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
			columns = append(columns, table.Column{Title: header, Width: m.cachedColWidths[i]})
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

// sanitize strips ANSI escape sequences and control characters from s.
func sanitize(s string) string {
	// Fast path: if no escape characters exist, return as-is
	if !strings.ContainsRune(s, '\x1b') {
		clean := true
		for i := 0; i < len(s); i++ {
			if s[i] < 0x20 && s[i] != '\t' {
				clean = false
				break
			}
		}
		if clean {
			return s
		}
	}

	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// skip CSI sequence: ESC [ ... final byte
			j := i + 2
			for j < len(s) && s[j] >= 0x20 && s[j] <= 0x3F {
				j++
			}
			if j < len(s) {
				j++ // skip final byte
			}
			i = j
			continue
		}
		if s[i] == '\x1b' {
			// skip other escape sequences (OSC, etc.): ESC ... ST/BEL
			j := i + 1
			for j < len(s) && s[j] != '\x1b' && s[j] != '\a' {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		if s[i] < 0x20 && s[i] != '\t' {
			i++
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func (m *model) applyResult(result db.QueryResult) {
	m.lastResult = result
	m.applyResultWithSort(result)
}

func (m *model) setStatus(text string, isError bool) {
	m.statusText = text
	m.statusError = isError
}

// prepareAndExecuteQuery cancels any in-flight query, records the query in
// history, and returns a Cmd that executes it. Callers should use this instead
// of duplicating cancel/history/execute logic.
func (m *model) prepareAndExecuteQuery(query string) tea.Cmd {
	if m.queryCancel != nil {
		m.queryCancel()
	}
	// Add to history (skip duplicates of last entry)
	if query != "" && (len(m.queryHistory) == 0 || m.queryHistory[len(m.queryHistory)-1] != query) {
		m.queryHistory = append(m.queryHistory, query)
		if len(m.queryHistory) > maxHistory {
			m.queryHistory = m.queryHistory[1:]
		}
	}
	m.historyIdx = -1
	ctx, cancel := context.WithCancel(context.Background())
	m.querySeq++
	m.queryCancel = cancel
	return executeQueryCmd(ctx, m.activeDB(), query, m.querySeq)
}

func executeQueryCmd(parent context.Context, adapter db.DBAdapter, query string, seq uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, queryTimeout)
		defer cancel()

		result, err := adapter.Query(ctx, query)
		return queryExecutedMsg{seq: seq, result: result, err: err}
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
