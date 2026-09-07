package ui

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/sqlite"
)

// countingAdapter counts catalog requests so a test can tell "one fetch per
// table" from "fetching forever".
type countingAdapter struct {
	db.DBAdapter
	columnsCalls int
}

func (c *countingAdapter) Columns(ctx context.Context, table string) ([]string, error) {
	c.columnsCalls++
	return c.DBAdapter.Columns(ctx, table)
}

// newCompletionModel opens an in-memory SQLite with n tables t00..tNN, each
// holding a zvalue column, and returns a model in INSERT mode with "SELECT z"
// typed — a column context with no FROM clause, so completion has to gather
// the columns of every table.
func newCompletionModel(t *testing.T, n int) (*model, *countingAdapter) {
	t.Helper()
	raw, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	t.Cleanup(func() { raw.Close() })
	tables := make([]string, n)
	for i := range tables {
		tables[i] = fmt.Sprintf("t%02d", i)
		if _, err := raw.Query(t.Context(), "CREATE TABLE "+tables[i]+" (zvalue TEXT)"); err != nil {
			t.Fatalf("create %s: %v", tables[i], err)
		}
	}
	counting := &countingAdapter{DBAdapter: raw}
	m := newTestModel()
	m.connMgr = newConnManager("test", ":memory:", counting, false)
	m.sidebar.tables = tables
	m.mode = insertMode
	m.textarea.Focus()
	m.textarea.SetValue("SELECT z")
	return m, counting
}

// drain runs the Cmd/Update loop the Bubble Tea runtime would, with a bound
// so a fetch loop fails the test instead of hanging it.
func drain(t *testing.T, m tea.Model, cmd tea.Cmd, limit int) model {
	t.Helper()
	for rounds := 0; cmd != nil; rounds++ {
		if rounds >= limit {
			t.Fatalf("completion still fetching after %d rounds", rounds)
		}
		m, cmd = m.Update(cmd())
	}
	return m.(model)
}

// allColumns used to demand every table in the cache at once, so on a schema
// larger than 64 the fetch of table 65 evicted table 1 and the
// re-scan started over: 200 rounds of columnsLoadedMsg, the cache pinned at
// 64 entries, the editor still reading "SELECT z".
func TestAllColumnsCompletion_AcrossTheCacheBoundary(t *testing.T) {
	for _, n := range []int{64, 64 + 1, 2*64 + 3} {
		t.Run(fmt.Sprintf("%d tables", n), func(t *testing.T) {
			m, counting := newCompletionModel(t, n)

			next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
			rm := drain(t, next, cmd, n)

			// zvalue is the only candidate, so completion inserts it directly.
			if got := rm.textarea.Value(); got != "SELECT zvalue" {
				t.Fatalf("editor = %q, want %q", got, "SELECT zvalue")
			}
			if counting.columnsCalls != n {
				t.Errorf("Columns called %d times for %d tables, want exactly one per table", counting.columnsCalls, n)
			}
			if len(rm.completion.colCache) > 64 {
				t.Errorf("cache holds %d entries after completion, want at most %d", len(rm.completion.colCache), 64)
			}
			if rm.completion.pendingPrefix != "" {
				t.Errorf("pendingPrefix = %q after completion, want empty", rm.completion.pendingPrefix)
			}
		})
	}
}

