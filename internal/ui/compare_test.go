package ui

import (
	"errors"
	"strings"
	"testing"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/sqlite"
	"github.com/kwrkb/asql/internal/ui/table"
)

func TestCellDiffAt(t *testing.T) {
	t.Run("same value is not diff", func(t *testing.T) {
		self := []table.Row{{"1", "alice"}}
		other := []table.Row{{"1", "alice"}}
		if cellDiffAt(0, 0, self, 1, other, 1) {
			t.Fatal("expected no diff for identical cells")
		}
	})

	t.Run("different value is diff", func(t *testing.T) {
		self := []table.Row{{"1"}}
		other := []table.Row{{"2"}}
		if !cellDiffAt(0, 0, self, 1, other, 1) {
			t.Fatal("expected diff for different values")
		}
	})

	t.Run("extra row highlights existing side", func(t *testing.T) {
		self := []table.Row{{"1"}}
		var other []table.Row
		if !cellDiffAt(0, 0, self, 1, other, 0) {
			t.Fatal("expected diff for row existing only on self side")
		}
	})

	t.Run("extra column highlights existing side", func(t *testing.T) {
		self := []table.Row{{"1", "x"}}
		other := []table.Row{{"1"}}
		if !cellDiffAt(0, 1, self, 1, other, 1) {
			t.Fatal("expected diff for column existing only on self side")
		}
		if cellDiffAt(0, 1, other, 1, self, 1) {
			t.Fatal("expected no diff for missing cell on self side")
		}
	})

	t.Run("sentinel row is not highlighted", func(t *testing.T) {
		self := []table.Row{{"(no rows)", ""}}
		other := []table.Row{{"1", "alice"}}
		if cellDiffAt(0, 0, self, 0, other, 1) {
			t.Fatal("expected no diff for sentinel row without backing data")
		}
	})
}

func TestCompareMode_DetectsDiffCells(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 24

	base := db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
		Message: "1 row(s) returned",
	}
	m.applyResult(base)
	m.pinned = m.pinCurrentResult()
	m.comparePane = 1

	changed := db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"2"}},
		Message: "1 row(s) returned",
	}
	m.applyResult(changed)

	if !m.activeCellDiff(0, 0) {
		t.Fatal("expected active diff cell")
	}
	if !m.pinnedCellDiff(0, 0) {
		t.Fatal("expected pinned diff cell")
	}
}

func TestCompareMode_StatusAndLabelsShowRowDiff(t *testing.T) {
	m := newTestModel()
	m.width = 120
	m.height = 24
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
		Message: "1 row(s) returned",
	}
	m.applyResult(m.lastResult)

	result, _ := m.updateNormal(runeMsg("c"))
	rm := result.(model)
	if !strings.Contains(rm.statusText, "left:1 right:1 diff:+0") {
		t.Fatalf("expected compare summary in status, got %q", rm.statusText)
	}

	result, _ = rm.Update(queryExecutedMsg{
		seq: rm.querySeq,
		result: db.QueryResult{
			Columns: []string{"id"},
			Rows:    [][]string{{"1"}, {"2"}},
			Message: "2 row(s) returned",
		},
	})
	rm = result.(model)
	if !strings.Contains(rm.statusText, "left:1 right:2 diff:+1") {
		t.Fatalf("expected updated compare summary in status, got %q", rm.statusText)
	}

	view := ansiRe.ReplaceAllString(rm.renderCompareView(), "")
	if !strings.Contains(view, "rows:1") || !strings.Contains(view, "rows:2") {
		t.Fatalf("expected row counts in compare labels, got:\n%s", view)
	}
}

// Compare rendering at tiny terminal heights: resultsHeight() returns 0 when
// the terminal is too low, so the derived paneHeight must be clamped instead
// of going negative (same class as the contentWidth underflow in #72).
func TestCompareTinyTerminalHeight(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.applyResult(db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
		Message: "1 row(s) returned",
	})
	m.pinned = m.pinCurrentResult()
	m.comparePane = 1

	for h := 0; h <= 8; h++ {
		m.height = h
		m.syncCompareTables()
		_ = m.renderCompareView()
	}
}

// A message-only result — an UPDATE or DELETE run while compare mode is open —
// must still invalidate the pinned pane. Its diff highlighting was computed
// against the previous active rows, and the active side now has none; with the
// rebuild-skip cache in place, a pane left clean keeps that stale highlighting
// indefinitely rather than only until the next redraw.
func TestMessageOnlyResultInvalidatesPinnedPane(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.applyResult(db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
		Message: "1 row(s) returned",
	})
	m.pinned = m.pinCurrentResult()
	m.pinned.viewportDirty = false

	m.applyResult(db.QueryResult{Message: "Query OK, 1 row affected"})

	if !m.pinned.viewportDirty {
		t.Error("a message-only result left the pinned pane's diff highlighting stale")
	}
}

// compareLabels returns the two "[name rows:n]" labels of the compare view.
func compareLabels(t *testing.T, m model) (left, right string) {
	t.Helper()
	view := ansiRe.ReplaceAllString(m.renderCompareView(), "")
	var labels []string
	for _, f := range strings.Fields(view) {
		if strings.HasPrefix(f, "[") {
			labels = append(labels, f)
		}
	}
	if len(labels) < 2 {
		t.Fatalf("expected two pane labels, got %v in:\n%s", labels, view)
	}
	return labels[0], labels[1]
}

// The pane labels name the connection each result came from. A connection
// switch keeps lastResult on screen but moves ActiveName, and the labels used
// to read ActiveName — so after A → B with no query run on B, A's rows sat
// under a B label on both sides.
func TestCompareLabels_NameTheResultOrigin(t *testing.T) {
	m := newTestModel()
	m.width, m.height = 120, 24
	m.mode = normalMode
	m.table.Focus()

	run := func(m model, conn string, res db.QueryResult, err error) model {
		next, _ := m.Update(queryExecutedMsg{seq: m.querySeq, query: "SELECT 1", conn: conn, result: res, err: err})
		return next.(model)
	}
	switchTo := func(m model, name string) model {
		t.Helper()
		other, err := sqlite.Open(":memory:")
		if err != nil {
			t.Fatalf("sqlite.Open: %v", err)
		}
		t.Cleanup(func() { other.Close() })
		m.connMgr.Register(name, name+".db", other)
		idx, err := m.connMgr.Prepare(name, name+".db")
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		next, _ := m.Update(connSwitchedMsg{seq: m.switchSeq, conn: idx})
		sm := next.(model)
		if sm.connMgr.ActiveName() != name {
			t.Fatalf("active connection = %q, want %q", sm.connMgr.ActiveName(), name)
		}
		return sm
	}
	one := db.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}, Message: "1 row(s) returned"}
	two := db.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}, {"2"}}, Message: "2 row(s) returned"}

	rm := run(*m, "test", one, nil)

	// Switch only, then pin: both panes still show the result from "test".
	rm = switchTo(rm, "staging")
	next, _ := rm.Update(runeMsg("c"))
	rm = next.(model)
	if rm.pinned == nil {
		t.Fatal("test premise broken: compare did not open")
	}
	if rm.pinned.connName != "test" {
		t.Fatalf("pinned.connName = %q, want test (the result origin, not the active connection)", rm.pinned.connName)
	}
	if l, r := compareLabels(t, rm); l != "[test" || r != "[test" {
		t.Fatalf("labels after switch without a query = %s / %s, want [test / [test", l, r)
	}

	// A failed query on staging leaves the old result — and its label — in place.
	rm = run(rm, "staging", db.QueryResult{}, errors.New("no such table"))
	if l, r := compareLabels(t, rm); l != "[test" || r != "[test" {
		t.Fatalf("labels after a failed query on staging = %s / %s, want [test / [test", l, r)
	}

	// Only a result that actually came from staging relabels the right pane.
	rm = run(rm, "staging", two, nil)
	if l, r := compareLabels(t, rm); l != "[test" || r != "[staging" {
		t.Fatalf("labels after a staging result = %s / %s, want [test / [staging", l, r)
	}

	// Switching while compare is open moves neither label: no result moved.
	rm = switchTo(rm, "prod")
	if l, r := compareLabels(t, rm); l != "[test" || r != "[staging" {
		t.Fatalf("labels after switching under an open compare = %s / %s, want [test / [staging", l, r)
	}
}
