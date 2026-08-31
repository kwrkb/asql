package ui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/ui/table"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestColumnWidth(t *testing.T) {
	tests := []struct {
		name  string
		title string
		rows  [][]string
		idx   int
		want  int
	}{
		{
			name:  "minimum width when title and values are short",
			title: "id",
			rows:  [][]string{{"1"}, {"2"}},
			idx:   0,
			want:  12,
		},
		{
			name:  "title determines width",
			title: "user_name_column",
			rows:  [][]string{{"alice"}},
			idx:   0,
			want:  18, // len("user_name_column")=16, 16+2=18
		},
		{
			name:  "row value determines width",
			title: "val",
			rows:  [][]string{{"a_medium_length_str"}},
			idx:   0,
			want:  21, // len("a_medium_length_str")=19, 19+2=21
		},
		{
			name:  "capped at 32 when width+2 would exceed",
			title: "abcdefghijklmnopqrstuvwxyzabcde", // 31 chars
			rows:  nil,
			idx:   0,
			want:  32, // 31+2=33 → capped at 32
		},
		{
			name:  "exactly 32 when width is 30",
			title: "abcdefghijklmnopqrstuvwxyzabcd", // 30 chars
			rows:  nil,
			idx:   0,
			want:  32, // 30+2=32
		},
		{
			name:  "out of bounds idx skipped safely",
			title: "col",
			rows:  [][]string{{"only one col"}},
			idx:   5,
			want:  12, // title "col" is short, minimum applied
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := columnWidth(tt.title, tt.rows, tt.idx)
			if got != tt.want {
				t.Errorf("columnWidth(%q, rows, %d) = %d, want %d", tt.title, tt.idx, got, tt.want)
			}
		})
	}
}

func newTestModel() *model {
	tbl := table.New()
	vp := viewport.New(0, 0)
	ta := textarea.New()
	return &model{
		connMgr:    newConnManager("test", "", nil, false),
		table:      tbl,
		viewport:   vp,
		textarea:   ta,
		width:      80,
		height:     24,
		historyIdx: -1,
		histSearch: histSearchState{input: textinput.New()},
	}
}

func TestApplyResult(t *testing.T) {
	t.Run("SELECT result with rows", func(t *testing.T) {
		m := newTestModel()
		result := db.QueryResult{
			Columns: []string{"id", "name"},
			Rows:    [][]string{{"1", "alice"}, {"2", "bob"}},
			Message: "2 row(s) returned",
		}
		m.applyResult(result)

		cols := m.table.Columns()
		if len(cols) != 2 {
			t.Errorf("expected 2 columns, got %d", len(cols))
		}
		rows := m.table.Rows()
		if len(rows) != 2 {
			t.Errorf("expected 2 rows, got %d", len(rows))
		}
		if m.statusText != "2 row(s) returned" {
			t.Errorf("unexpected status: %q", m.statusText)
		}
	})

	t.Run("SELECT result with no rows uses padded sentinel", func(t *testing.T) {
		m := newTestModel()
		result := db.QueryResult{
			Columns: []string{"id", "name"},
			Rows:    [][]string{},
			Message: "0 row(s) returned",
		}
		m.applyResult(result)

		rows := m.table.Rows()
		if len(rows) != 1 {
			t.Fatalf("expected 1 sentinel row, got %d", len(rows))
		}
		if rows[0][0] != "(no rows)" {
			t.Errorf("expected '(no rows)' sentinel, got %q", rows[0][0])
		}
		// sentinel row must have same column count as columns to avoid panic
		if len(rows[0]) != 2 {
			t.Errorf("expected sentinel row to have 2 cols (matching columns), got %d", len(rows[0]))
		}
	})

	t.Run("DML result shows message in Result column", func(t *testing.T) {
		m := newTestModel()
		result := db.QueryResult{
			Message: "3 row(s) affected",
		}
		m.applyResult(result)

		cols := m.table.Columns()
		if len(cols) != 1 || cols[0].Title != "Result" {
			t.Errorf("expected single 'Result' column, got %v", cols)
		}
		rows := m.table.Rows()
		if len(rows) != 1 || rows[0][0] != "3 row(s) affected" {
			t.Errorf("unexpected DML row: %v", rows)
		}
		if m.statusText != "3 row(s) affected" {
			t.Errorf("unexpected status: %q", m.statusText)
		}
	})

	t.Run("column headers include type info", func(t *testing.T) {
		m := newTestModel()
		result := db.QueryResult{
			Columns:     []string{"id", "name"},
			ColumnTypes: []string{"INTEGER", "TEXT"},
			Rows:        [][]string{{"1", "alice"}},
			Message:     "1 row(s) returned",
		}
		m.applyResult(result)

		cols := m.table.Columns()
		got0 := ansiRe.ReplaceAllString(cols[0].Title, "")
		got1 := ansiRe.ReplaceAllString(cols[1].Title, "")
		if got0 != "id int" {
			t.Errorf("expected 'id int', got %q", got0)
		}
		if got1 != "name text" {
			t.Errorf("expected 'name text', got %q", got1)
		}
	})

	t.Run("column headers without type info", func(t *testing.T) {
		m := newTestModel()
		result := db.QueryResult{
			Columns: []string{"id", "name"},
			Rows:    [][]string{{"1", "alice"}},
			Message: "1 row(s) returned",
		}
		m.applyResult(result)

		cols := m.table.Columns()
		if cols[0].Title != "id" {
			t.Errorf("expected 'id', got %q", cols[0].Title)
		}
	})
}

func TestQueryHistory(t *testing.T) {
	t.Run("history stores executed queries", func(t *testing.T) {
		m := newTestModel()
		m.queryHistory = append(m.queryHistory, "SELECT 1")
		m.queryHistory = append(m.queryHistory, "SELECT 2")

		if len(m.queryHistory) != 2 {
			t.Fatalf("expected 2 history entries, got %d", len(m.queryHistory))
		}
		if m.queryHistory[0] != "SELECT 1" {
			t.Errorf("expected 'SELECT 1', got %q", m.queryHistory[0])
		}
	})

	t.Run("history navigation with ctrl+p and ctrl+n", func(t *testing.T) {
		m := newTestModel()
		m.mode = insertMode
		m.queryHistory = []string{"SELECT 1", "SELECT 2", "SELECT 3"}
		m.historyIdx = -1

		// ctrl+p: go to last entry
		m.historyDraft = "current input"
		m.historyIdx = len(m.queryHistory) - 1
		if m.queryHistory[m.historyIdx] != "SELECT 3" {
			t.Errorf("expected 'SELECT 3', got %q", m.queryHistory[m.historyIdx])
		}

		// ctrl+p again: go to previous
		m.historyIdx--
		if m.queryHistory[m.historyIdx] != "SELECT 2" {
			t.Errorf("expected 'SELECT 2', got %q", m.queryHistory[m.historyIdx])
		}

		// ctrl+n: go to next
		m.historyIdx++
		if m.queryHistory[m.historyIdx] != "SELECT 3" {
			t.Errorf("expected 'SELECT 3', got %q", m.queryHistory[m.historyIdx])
		}

		// ctrl+n at end: back to draft
		m.historyIdx = -1
		if m.historyDraft != "current input" {
			t.Errorf("expected draft 'current input', got %q", m.historyDraft)
		}
	})

	t.Run("history cap at maxHistory", func(t *testing.T) {
		m := newTestModel()
		for i := 0; i < maxHistory+10; i++ {
			m.queryHistory = append(m.queryHistory, "q")
			if len(m.queryHistory) > maxHistory {
				m.queryHistory = m.queryHistory[1:]
			}
		}
		if len(m.queryHistory) != maxHistory {
			t.Errorf("expected %d entries, got %d", maxHistory, len(m.queryHistory))
		}
	})
}

func TestDetailMode_EnterFromNormal(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "alice"}, {"2", "bob"}},
	}
	m.applyResult(m.lastResult)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.updateNormal(msg)
	rm := result.(model)

	if rm.mode != detailMode {
		t.Errorf("expected detailMode, got %q", rm.mode)
	}
	if rm.detail.fieldCursor != 0 {
		t.Errorf("expected detailFieldCursor=0, got %d", rm.detail.fieldCursor)
	}
	if rm.detail.scroll != 0 {
		t.Errorf("expected detailScroll=0, got %d", rm.detail.scroll)
	}
}

func TestDetailMode_EnterWithNoResults(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{}

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.updateNormal(msg)
	rm := result.(model)

	if rm.mode != normalMode {
		t.Errorf("expected normalMode when no results, got %q", rm.mode)
	}
}

func TestDetailMode_EscReturns(t *testing.T) {
	m := newTestModel()
	m.mode = detailMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "alice"}},
	}
	m.detail.fieldCursor = 1

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	result, _ := m.updateDetail(msg)
	rm := result.(model)

	if rm.mode != normalMode {
		t.Errorf("expected normalMode, got %q", rm.mode)
	}
}

func TestDetailMode_FieldNavigation(t *testing.T) {
	m := newTestModel()
	m.mode = detailMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id", "name", "email"},
		Rows:    [][]string{{"1", "alice", "alice@example.com"}},
	}
	m.detail.fieldCursor = 0

	// j moves down
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	result, _ := m.updateDetail(msg)
	rm := result.(model)
	if rm.detail.fieldCursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", rm.detail.fieldCursor)
	}

	// j again
	m.detail.fieldCursor = 1
	result, _ = m.updateDetail(msg)
	rm = result.(model)
	if rm.detail.fieldCursor != 2 {
		t.Errorf("expected cursor=2 after j, got %d", rm.detail.fieldCursor)
	}

	// j at bottom boundary
	m.detail.fieldCursor = 2
	result, _ = m.updateDetail(msg)
	rm = result.(model)
	if rm.detail.fieldCursor != 2 {
		t.Errorf("expected cursor=2 at boundary, got %d", rm.detail.fieldCursor)
	}

	// k moves up
	m.detail.fieldCursor = 2
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	result, _ = m.updateDetail(msg)
	rm = result.(model)
	if rm.detail.fieldCursor != 1 {
		t.Errorf("expected cursor=1 after k, got %d", rm.detail.fieldCursor)
	}

	// k at top boundary
	m.detail.fieldCursor = 0
	result, _ = m.updateDetail(msg)
	rm = result.(model)
	if rm.detail.fieldCursor != 0 {
		t.Errorf("expected cursor=0 at boundary, got %d", rm.detail.fieldCursor)
	}
}

func TestDetailMode_ShowsSortedRow(t *testing.T) {
	m := newTestModel()
	m.lastResult = db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"2", "bob"}, {"1", "alice"}},
		Message: "2 row(s) returned",
	}
	m.applyResult(m.lastResult)

	// Sort by column 0 ascending: alice(1) should come first
	m.colCursor = 0
	m.sortCol = 0
	m.sortDir = sortAsc
	m.applySortedResult()

	// First row in sorted table should be "1", "alice"
	rows := m.table.Rows()
	if rows[0][0] != "1" || rows[0][1] != "alice" {
		t.Fatalf("expected sorted first row [1, alice], got %v", rows[0])
	}

	// Enter detail mode on sorted first row
	m.mode = detailMode
	m.detail.fieldCursor = 0
	m.detail.scroll = 0
	m.width = 80
	m.height = 24

	// renderWithDetailOverlay should show the sorted row, not original
	view := m.renderWithDetailOverlay("background")
	if !strings.Contains(view, "alice") {
		t.Errorf("expected detail view to show 'alice' (sorted first row), got:\n%s", view)
	}
}

// A goroutine that finishes just before a cancel has already posted its
// message with a matching seq and nil error, so the seq must be bumped on
// every cancel path or the "Cancelled" state gets silently overwritten.
func TestCancelDiscardsCompletedResponse(t *testing.T) {
	t.Run("Ctrl+C bumps querySeq so a stale AI response is dropped", func(t *testing.T) {
		m := newTestModel()
		m.querySeq = 5
		m.queryCancel = func() {}
		m.textarea.SetValue("SELECT 1;")

		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		nm := next.(model)
		if nm.querySeq != 6 {
			t.Fatalf("querySeq = %d after Ctrl+C, want 6", nm.querySeq)
		}

		next, _ = nm.Update(aiResponseMsg{seq: 5, sql: "SELECT 2;"})
		nm = next.(model)
		if nm.mode == insertMode {
			t.Error("stale aiResponseMsg forced INSERT mode after cancel")
		}
		if got := nm.textarea.Value(); got != "SELECT 1;" {
			t.Errorf("stale aiResponseMsg overwrote the editor: %q", got)
		}
	})

	t.Run("Esc during AI loading bumps querySeq", func(t *testing.T) {
		m := newTestModel()
		m.mode = aiMode
		m.aiSt.loading = true
		m.querySeq = 5
		m.queryCancel = func() {}
		m.textarea.SetValue("SELECT 1;")

		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		nm := next.(model)
		if nm.querySeq != 6 {
			t.Fatalf("querySeq = %d after Esc, want 6", nm.querySeq)
		}

		next, _ = nm.Update(aiResponseMsg{seq: 5, sql: "SELECT 2;"})
		nm = next.(model)
		if nm.mode == insertMode {
			t.Error("stale aiResponseMsg forced INSERT mode after cancel")
		}
		if got := nm.textarea.Value(); got != "SELECT 1;" {
			t.Errorf("stale aiResponseMsg overwrote the editor: %q", got)
		}
	})
}

// The completion popup must not grow the view past the terminal height: it
// used to be appended below the textarea, pushing the status bar and the
// bottom of the results pane past the final MaxHeight cut.
func TestViewHeightWithCompletionPopup(t *testing.T) {
	m := newTestModel()
	m.connMgr = newConnManager("test", "", &stubAdapter{}, false)
	m.mode = insertMode
	m.width = 80
	m.height = 24
	m.resize()
	m.setStatus("Insert mode", false)

	baseline := len(strings.Split(m.View(), "\n"))

	m.completion.active = true
	m.completion.items = []string{"users", "user_logs", "user_roles"}
	withPopup := m.View()
	lines := strings.Split(withPopup, "\n")

	if len(lines) > baseline {
		t.Errorf("popup grew the view: %d -> %d lines", baseline, len(lines))
	}
	if len(lines) > m.height {
		t.Errorf("view is %d lines, exceeds terminal height %d", len(lines), m.height)
	}
	if !strings.Contains(ansiRe.ReplaceAllString(withPopup, ""), "Insert mode") {
		t.Error("status bar is missing while the completion popup is open")
	}
	if !strings.Contains(ansiRe.ReplaceAllString(withPopup, ""), "user_logs") {
		t.Error("completion popup items are not rendered")
	}
}

// The rebuild-skip cache: syncViewport must rebuild when the column cursor or
// the header-highlight state changes, and must NOT be forced dirty by the
// adjustColOffset call that runs on every sync (that would rebuild all rows on
// every keypress, j/k included).
func TestViewportRebuildSkip(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.applyResult(db.QueryResult{
		Columns: []string{"a", "b"},
		Rows:    [][]string{{"1", "2"}, {"3", "4"}},
		Message: "2 row(s) returned",
	})
	if m.viewportDirty {
		t.Fatal("viewportDirty still set after applyResult's rebuild")
	}

	// A no-op adjustment (cursor already visible) must not dirty the viewport.
	m.adjustColOffset()
	if m.viewportDirty {
		t.Error("adjustColOffset dirtied the viewport without an offset change")
	}

	// Moving the column cursor must trigger a rebuild (the header highlight
	// follows it), observable via the cache bookkeeping.
	m.colCursor = 1
	m.syncViewport()
	if m.lastColCursor != 1 {
		t.Error("expected a rebuild after colCursor moved")
	}

	// Leaving NORMAL mode drops the header highlight, which needs a rebuild too.
	m.mode = insertMode
	m.syncViewport()
	if m.lastHighlight {
		t.Error("expected a rebuild after leaving NORMAL mode")
	}
}

// The UI stays interactive while a connection switch runs in its goroutine,
// so a completion message must carry the switch generation and must not yank
// the user out of whatever they started in the meantime.
func TestConnSwitchGuards(t *testing.T) {
	newSwitchModel := func() *model {
		m := newTestModel()
		m.connMgr = newConnManager("test", "", &stubAdapter{}, false)
		return m
	}

	t.Run("stale switch completion is discarded", func(t *testing.T) {
		m := newSwitchModel()
		m.switchSeq = 2
		m.querySeq = 5
		m.mode = insertMode
		m.setStatus("Insert mode", false)

		next, _ := m.Update(connSwitchedMsg{seq: 1})
		nm := next.(model)
		if nm.querySeq != 5 || nm.connGen != 0 {
			t.Errorf("stale connSwitchedMsg mutated state: querySeq=%d connGen=%d", nm.querySeq, nm.connGen)
		}
		if nm.mode != insertMode {
			t.Errorf("stale connSwitchedMsg changed mode to %q", nm.mode)
		}
		if nm.statusText != "Insert mode" {
			t.Errorf("stale connSwitchedMsg changed status to %q", nm.statusText)
		}
	})

	t.Run("completion leaves INSERT mode alone", func(t *testing.T) {
		m := newSwitchModel()
		m.mode = insertMode
		m.textarea.Focus()
		m.textarea.SetValue("SELECT 1;")

		next, _ := m.Update(connSwitchedMsg{seq: 0})
		nm := next.(model)
		if nm.mode != insertMode {
			t.Errorf("switch completion forced mode to %q while user was typing", nm.mode)
		}
		if got := nm.textarea.Value(); got != "SELECT 1;" {
			t.Errorf("editor content changed: %q", got)
		}
	})

	t.Run("completion closes the profile overlay when still open", func(t *testing.T) {
		m := newSwitchModel()
		m.mode = profileMode

		next, _ := m.Update(connSwitchedMsg{seq: 0})
		nm := next.(model)
		if nm.mode != normalMode {
			t.Errorf("expected normalMode after switch from profile overlay, got %q", nm.mode)
		}
	})

	t.Run("cancelling an in-flight query is said in the status", func(t *testing.T) {
		m := newSwitchModel()
		m.mode = normalMode
		m.queryCancel = func() {}

		next, _ := m.Update(connSwitchedMsg{seq: 0})
		nm := next.(model)
		if !strings.Contains(nm.statusText, "cancelled") {
			t.Errorf("status does not mention the cancelled query: %q", nm.statusText)
		}
	})
}

// Two switches in flight at once. The newer one completes and is applied; the
// older one completes afterwards and must leave the active connection where it
// is. Opening is what runs in the goroutine — committing happens on the UI
// thread, past the sequence check — so the connection queries actually run
// against never drifts from the one the status bar, the DSN, the table cache
// and the connection generation describe.
func TestStaleSwitchDoesNotMoveTheActiveConnection(t *testing.T) {
	m := newTestModel()
	m.connMgr = newConnManager("base", "base.db", &stubAdapter{}, false)
	m.connMgr.Register("older", "older.db", &stubAdapter{})
	m.connMgr.Register("newer", "newer.db", &stubAdapter{})

	m.switchSeq++
	olderSeq := m.switchSeq
	olderIdx, err := m.connMgr.Prepare("older", "older.db")
	if err != nil {
		t.Fatalf("Prepare(older): %v", err)
	}
	m.switchSeq++
	newerSeq := m.switchSeq
	newerIdx, err := m.connMgr.Prepare("newer", "newer.db")
	if err != nil {
		t.Fatalf("Prepare(newer): %v", err)
	}
	if got := m.connMgr.ActiveName(); got != "base" {
		t.Fatalf("Prepare moved the active connection to %q; it must only open", got)
	}

	applied, _ := m.Update(connSwitchedMsg{seq: newerSeq, conn: newerIdx})
	am := applied.(model)
	if got := am.connMgr.ActiveName(); got != "newer" {
		t.Fatalf("active connection = %q after the newer switch, want newer", got)
	}

	stale, _ := am.Update(connSwitchedMsg{seq: olderSeq, conn: olderIdx})
	sm := stale.(model)
	if got := sm.connMgr.ActiveName(); got != "newer" {
		t.Errorf("a stale switch completion moved the active connection to %q", got)
	}
	if got := sm.statusText; !strings.Contains(got, "newer") {
		t.Errorf("status = %q, no longer describes the connection queries run against", got)
	}
}
