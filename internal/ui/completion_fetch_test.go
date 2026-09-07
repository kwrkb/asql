package ui

import (
	"context"
	"fmt"
	"testing"
	"time"

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

// deadlineAdapter records the deadline each catalog request was given, so a
// test can tell a per-request budget from one budget shared by the batch.
type deadlineAdapter struct {
	db.DBAdapter
	deadlines []time.Time
}

func (d *deadlineAdapter) Columns(ctx context.Context, table string) ([]string, error) {
	dl, ok := ctx.Deadline()
	if !ok {
		return nil, fmt.Errorf("Columns(%q) got a context with no deadline", table)
	}
	d.deadlines = append(d.deadlines, dl)
	time.Sleep(time.Millisecond)
	return d.DBAdapter.Columns(ctx, table)
}

// One deadline over the whole batch scales with the schema: every request can
// be healthy and the gather still expire, throwing the completion away. Each
// request gets its own budget instead, so the deadline a table is given moves
// forward as the batch proceeds rather than draining towards a fixed instant.
func TestAllColumnsCompletion_DeadlineIsPerRequest(t *testing.T) {
	m, counting := newCompletionModel(t, 4)
	deadlines := &deadlineAdapter{DBAdapter: counting}
	m.connMgr = newConnManager("test", ":memory:", deadlines, false)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	drain(t, next, cmd, 4)

	if len(deadlines.deadlines) != 4 {
		t.Fatalf("recorded %d deadlines, want one per table", len(deadlines.deadlines))
	}
	for i := 1; i < len(deadlines.deadlines); i++ {
		if !deadlines.deadlines[i].After(deadlines.deadlines[i-1]) {
			t.Errorf("request %d's deadline (%v) is not later than request %d's (%v): the batch shares one budget",
				i, deadlines.deadlines[i], i-1, deadlines.deadlines[i-1])
		}
	}
}

// A batch fetch must stop once the completion that asked for it is gone. The
// prefix check only runs when the batch returns, so without cancellation a Tab
// the user typed straight past still cost one catalog request per table.
//
// The abandoned batch then returns early with an error, and that message is
// still in flight when the next Tab starts its own batch: the late arrival must
// not clear the pendingPrefix the new batch is waiting on.
func TestAllColumnsCompletion_CancelledWhenAbandoned(t *testing.T) {
	const n = 32
	m, counting := newCompletionModel(t, n)

	next, abandoned := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if abandoned == nil {
		t.Fatal("Tab produced no fetch Cmd, so there is no batch to abandon")
	}

	// The user types on before the batch runs: the prefix it is gathering for
	// no longer exists.
	next, _ = next.(model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	if next.(model).completion.fetchCancel != nil {
		t.Error("fetchCancel still set after the completion was abandoned")
	}

	// The user asks again at the new prefix, and only then does the abandoned
	// batch return.
	next, wanted := next.(model).Update(tea.KeyMsg{Type: tea.KeyTab})
	if wanted == nil {
		t.Fatal("the second Tab produced no fetch Cmd")
	}

	msg, ok := abandoned().(allColumnsLoadedMsg)
	if !ok {
		t.Fatalf("abandoned batch returned %T, want allColumnsLoadedMsg", msg)
	}
	if msg.err == nil {
		t.Error("abandoned batch reported no error, so it ran to completion")
	}
	if counting.columnsCalls >= n {
		t.Errorf("Columns called %d times after the completion was abandoned, want it to stop early", counting.columnsCalls)
	}
	next, _ = next.(model).Update(msg)

	rm := drain(t, next, wanted, n)
	if got := rm.textarea.Value(); got != "SELECT zvalue" {
		t.Errorf("editor = %q, want %q: the abandoned batch's late message killed the completion the user asked for", got, "SELECT zvalue")
	}
}
