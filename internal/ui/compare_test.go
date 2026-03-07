package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"

	"github.com/kwrkb/asql/internal/db"
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
