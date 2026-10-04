package ui

import (
	"context"
	"slices"
	"testing"

	"github.com/kwrkb/asql/internal/db"
)

// stubAdapter answers Tables() with a fixed list. Everything else is unused by
// the table-loading path.
type stubAdapter struct {
	tables []string
}

func (s *stubAdapter) Type() string { return "stub" }
func (s *stubAdapter) Query(context.Context, string) (db.QueryResult, error) {
	return db.QueryResult{}, nil
}
func (s *stubAdapter) Tables(context.Context) ([]string, error) { return s.tables, nil }
func (s *stubAdapter) Columns(context.Context, string) ([]string, error) {
	return nil, nil
}
func (s *stubAdapter) Schema(context.Context) (string, error) { return "", nil }
func (s *stubAdapter) QuoteIdentifier(name string) string     { return name }
func (s *stubAdapter) Close() error                           { return nil }

func TestLoadTablesCmd_CarriesConnGen(t *testing.T) {
	cmd := loadTablesCmd(&stubAdapter{tables: []string{"a"}}, 7)
	msg, ok := cmd().(tablesLoadedMsg)
	if !ok {
		t.Fatalf("got %T, want tablesLoadedMsg", cmd())
	}
	if msg.connGen != 7 {
		t.Errorf("connGen = %d, want 7", msg.connGen)
	}
	if !slices.Equal(msg.tables, []string{"a"}) {
		t.Errorf("tables = %v, want [a]", msg.tables)
	}
}

// TestTablesLoaded_DiscardsStaleLoad reproduces the out-of-order sequence: a
// load started on connection A finishes after the user has switched to B and
// B's own load has already landed. A's result must not reach the sidebar, or
// the user picks a table name that does not exist on the live connection.
func TestTablesLoaded_DiscardsStaleLoad(t *testing.T) {
	m := newTestModel()

	// Connection A's load starts at generation 0.
	genA := m.connGen

	// Switching connections invalidates in-flight loads (see connSwitchedMsg).
	m.connGen++
	m.sidebar.tables = nil
	genB := m.connGen

	// B's load lands first.
	updated, _ := m.Update(tablesLoadedMsg{tables: []string{"b_orders"}, connGen: genB})
	afterB := updated.(model)
	if !slices.Equal(afterB.sidebar.tables, []string{"b_orders"}) {
		t.Fatalf("after B load: tables = %v, want [b_orders]", afterB.sidebar.tables)
	}

	// A's load arrives late and must be dropped.
	updated, _ = afterB.Update(tablesLoadedMsg{tables: []string{"a_users"}, connGen: genA})
	afterA := updated.(model)
	if !slices.Equal(afterA.sidebar.tables, []string{"b_orders"}) {
		t.Errorf("stale load applied: tables = %v, want [b_orders]", afterA.sidebar.tables)
	}
}

// A stale load must not report its error either — the message would name a
// connection the user has already left.
func TestTablesLoaded_DiscardsStaleError(t *testing.T) {
	m := newTestModel()
	genA := m.connGen
	m.connGen++

	updated, _ := m.Update(tablesLoadedMsg{tables: []string{"b_orders"}, connGen: m.connGen})
	afterB := updated.(model)

	updated, _ = afterB.Update(tablesLoadedMsg{err: context.DeadlineExceeded, connGen: genA})
	afterA := updated.(model)

	if afterA.statusError {
		t.Errorf("stale error surfaced: %q", afterA.statusText)
	}
	if !slices.Equal(afterA.sidebar.tables, []string{"b_orders"}) {
		t.Errorf("tables = %v, want [b_orders]", afterA.sidebar.tables)
	}
}

// TestQueryExecuted_ReloadsTablesOnlyWhenCatalogMayChange: a read-only
// statement cannot add or drop a table, so reloading the list after it only
// costs a catalog round-trip on a remote database — and the reload's arrival
// throws away the completion column cache, which Tab then fetches again.
func TestQueryExecuted_ReloadsTablesOnlyWhenCatalogMayChange(t *testing.T) {
	tests := []struct {
		query      string
		wantReload bool
	}{
		{"SELECT * FROM users", false},
		{"WITH x AS (SELECT 1) SELECT * FROM x", false},
		{"EXPLAIN SELECT 1", false},
		{"CREATE TABLE t (id INT)", true},
		{"DROP TABLE t", true},
		{"INSERT INTO t VALUES (1)", true},
		{"SELECT 1; DROP TABLE t", true},
		{"WITH gone AS (DELETE FROM t RETURNING *) SELECT * FROM gone", true},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			m := newTestModel()
			m.querySeq = 1
			_, cmd := m.Update(queryExecutedMsg{seq: 1, query: tt.query, result: db.QueryResult{
				Columns: []string{"a"}, Rows: [][]string{{"1"}},
			}})
			if got := cmd != nil; got != tt.wantReload {
				t.Errorf("reload = %v, want %v", got, tt.wantReload)
			}
		})
	}
}

// TestQueryExecuted_RetriesTableLoadAfterFailure: a read-only query skips the
// reload, so a table-list load that failed — say a timeout at connect — would
// otherwise never be retried until a write or a connection switch, leaving the
// sidebar and table-name completion empty.
func TestQueryExecuted_RetriesTableLoadAfterFailure(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tablesLoadedMsg{err: context.DeadlineExceeded, connGen: m.connGen})
	failed := updated.(model)

	failed.querySeq = 1
	updated, cmd := failed.Update(queryExecutedMsg{seq: 1, query: "SELECT 1", result: db.QueryResult{
		Columns: []string{"a"}, Rows: [][]string{{"1"}},
	}})
	if cmd == nil {
		t.Fatal("SELECT after a failed table load: no reload, want one")
	}

	updated, _ = updated.(model).Update(tablesLoadedMsg{tables: []string{"users"}, connGen: m.connGen})
	loaded := updated.(model)
	loaded.querySeq = 2
	if _, cmd := loaded.Update(queryExecutedMsg{seq: 2, query: "SELECT 1", result: db.QueryResult{
		Columns: []string{"a"}, Rows: [][]string{{"1"}},
	}}); cmd != nil {
		t.Error("SELECT after a successful table load: reloaded, want no reload")
	}
}
