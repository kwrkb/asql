package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/bring"
	"github.com/kwrkb/asql/internal/db/sqlite"
)

func TestBring_NoResultIsNoOp(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode

	result, cmd := m.updateNormal(runeMsg("b"))
	rm := result.(model)

	if cmd != nil {
		t.Error("expected no cmd when there is nothing to bring")
	}
	if !rm.statusError {
		t.Error("expected error status when bringing with no results")
	}
	if rm.bringSt.adapter != nil {
		t.Error("expected bring DB to stay uninitialized")
	}
}

func TestBring_TwiceProducesSequentialTables(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "alice"}},
	}

	result, cmd := m.updateNormal(runeMsg("b"))
	rm := result.(model)
	if rm.bringSt.adapter == nil {
		t.Fatal("expected bring adapter to be initialized after first bring")
	}
	if cmd == nil {
		t.Fatal("expected a cmd to materialize the result asynchronously")
	}
	// connManager started with 1 (nil) connection; Register should add a 2nd.
	if got := len(rm.connMgr.conns); got != 2 {
		t.Fatalf("expected 2 connections after first bring, got %d", got)
	}

	done, ok := cmd().(bringDoneMsg)
	if !ok {
		t.Fatalf("expected bringDoneMsg from cmd, got %T", done)
	}
	if done.err != nil || done.name != "t1" {
		t.Fatalf("expected successful bring as t1, got %+v", done)
	}
	updated, _ := rm.Update(done)
	um := updated.(model)
	if um.statusError {
		t.Errorf("expected success status, got error: %q", um.statusText)
	}

	result2, cmd2 := um.updateNormal(runeMsg("b"))
	rm2 := result2.(model)
	done2, ok := cmd2().(bringDoneMsg)
	if !ok {
		t.Fatalf("expected bringDoneMsg from second cmd, got %T", done2)
	}
	if done2.err != nil || done2.name != "t2" {
		t.Fatalf("expected successful bring as t2, got %+v", done2)
	}
	// The bring adapter is reused, not re-registered.
	if got := len(rm2.connMgr.conns); got != 2 {
		t.Fatalf("expected connection count to stay at 2 on second bring, got %d", got)
	}
}

// TestBring_FailureDoesNotReuseTableName guards against a race introduced by
// making bringCurrentResult asynchronous: rolling tableSeq back on failure
// would let a later, already-succeeded bring's name collide with the next
// attempt (e.g. t1 fails after t2 already succeeded -> the next bring must
// not retry "t2"). Table names are only ever skipped on failure, never
// reused, so tableSeq must stay monotonic regardless of success/failure.
func TestBring_FailureDoesNotReuseTableName(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
	}

	result, cmd := m.updateNormal(runeMsg("b"))
	rm := result.(model)
	done := cmd().(bringDoneMsg)
	done.err = fmt.Errorf("simulated failure")

	updated, _ := rm.Update(done)
	um := updated.(model)
	if !um.statusError {
		t.Error("expected error status after a failed bring")
	}
	if um.bringSt.tableSeq != 1 {
		t.Fatalf("expected tableSeq to stay at 1 (name t1 skipped, not reused) after failure, got %d", um.bringSt.tableSeq)
	}

	// Retrying must generate a fresh name, never reusing the failed one.
	result2, cmd2 := um.updateNormal(runeMsg("b"))
	rm2 := result2.(model)
	done2 := cmd2().(bringDoneMsg)
	if done2.name != "t2" {
		t.Fatalf("expected retry to use a fresh name t2, got %q", done2.name)
	}
	_ = rm2
}

func TestBring_SwitchWithNothingBroughtErrors(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode

	result, cmd := m.updateNormal(runeMsg("J"))
	rm := result.(model)

	if cmd != nil {
		t.Error("expected no cmd when nothing has been brought yet")
	}
	if !rm.statusError {
		t.Error("expected error status when switching with nothing brought")
	}
}

func TestBring_SwitchActivatesLocalConnection(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
	}

	bringResult, _ := m.updateNormal(runeMsg("b"))
	rm := bringResult.(model)

	switchResult, cmd := rm.updateNormal(runeMsg("J"))
	sm := switchResult.(model)
	if sm.statusError {
		t.Fatalf("unexpected error status before switch completes: %q", sm.statusText)
	}
	if cmd == nil {
		t.Fatal("expected a cmd to perform the connection switch")
	}

	msg := cmd()
	switched, ok := msg.(connSwitchedMsg)
	if !ok {
		t.Fatalf("expected connSwitchedMsg, got %T", msg)
	}
	if switched.err != nil {
		t.Fatalf("unexpected switch error: %v", switched.err)
	}

	updated, _ := sm.Update(switched)
	um := updated.(model)
	if !um.connMgr.IsActive(bringDSN) {
		t.Error("expected the local bring connection to become active")
	}
	if um.connMgr.Active() != rm.bringSt.adapter {
		t.Error("expected active adapter to be the bring adapter")
	}
}

// TestBring_OutOfOrderFailureDoesNotCollideWithLaterSuccess simulates two
// bring operations in flight at once (possible now that bringCurrentResult
// runs asynchronously via tea.Cmd) where the first (t1) fails only after the
// second (t2) has already completed successfully. The next bring must not
// retry the already-used name t2.
func TestBring_OutOfOrderFailureDoesNotCollideWithLaterSuccess(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
	}

	// Press 'b' twice before either completes: reserves t1 then t2.
	result1, cmd1 := m.updateNormal(runeMsg("b"))
	rm1 := result1.(model)
	result2, cmd2 := rm1.updateNormal(runeMsg("b"))
	rm2 := result2.(model)

	// t2's materialize completes first (success).
	doneT2 := cmd2().(bringDoneMsg)
	updatedAfterT2, _ := rm2.Update(doneT2)
	afterT2 := updatedAfterT2.(model)

	// t1's materialize completes afterward (failure).
	doneT1 := cmd1().(bringDoneMsg)
	doneT1.err = fmt.Errorf("simulated failure")
	updatedAfterT1, _ := afterT2.Update(doneT1)
	afterT1 := updatedAfterT1.(model)

	if afterT1.bringSt.tableSeq != 2 {
		t.Fatalf("expected tableSeq to remain at 2 after t1's late failure, got %d", afterT1.bringSt.tableSeq)
	}

	result3, cmd3 := afterT1.updateNormal(runeMsg("b"))
	rm3 := result3.(model)
	done3 := cmd3().(bringDoneMsg)
	if done3.name != "t3" {
		t.Fatalf("expected the next bring to use a fresh name t3, got %q", done3.name)
	}
	_ = rm3
}

func TestBring_ProvenanceUsesTheQueryThatProducedTheResult(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode

	// Accept a result the way the query pipeline does.
	accepted, _ := m.Update(queryExecutedMsg{
		seq:    m.querySeq,
		query:  "SELECT id FROM users",
		result: db.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}, Kinds: [][]db.Kind{{db.KindInt}}},
	})
	am := accepted.(model)

	// The editor has moved on since the result came back; provenance must
	// describe the query that produced the data, not what is being typed now.
	am.textarea.SetValue("SELECT * FROM something_else")

	if got := am.lastExecutedQuery(); got != "SELECT id FROM users" {
		t.Errorf("lastExecutedQuery = %q, want the query of the accepted result", got)
	}

	am.mode = normalMode
	result, cmd := am.updateNormal(runeMsg("b"))
	rm := result.(model)
	done, ok := cmd().(bringDoneMsg)
	if !ok {
		t.Fatalf("expected bringDoneMsg, got %T", done)
	}
	if done.err != nil {
		t.Fatalf("bring failed: %v", done.err)
	}

	got, err := rm.bringSt.adapter.Query(t.Context(),
		`SELECT table_name, query FROM `+bring.ProvenanceTable)
	if err != nil {
		t.Fatalf("query provenance: %v", err)
	}
	if len(got.Rows) != 1 {
		t.Fatalf("expected 1 provenance row, got %+v", got.Rows)
	}
	if got.Rows[0][0] != "t1" {
		t.Errorf("table_name = %q, want t1", got.Rows[0][0])
	}
	if got.Rows[0][1] != "SELECT id FROM users" {
		t.Errorf("query = %q, want the query of the accepted result", got.Rows[0][1])
	}
}

func TestBring_ProvenanceIgnoresAFailedFollowUpQuery(t *testing.T) {
	// Query A succeeds. Query B is then attempted and fails (or is cancelled,
	// or is still in flight). lastResult still holds A's rows, so provenance
	// must still credit A — the tail of queryHistory would say B.
	m := newTestModel()
	m.mode = normalMode

	accepted, _ := m.Update(queryExecutedMsg{
		seq:    m.querySeq,
		query:  "SELECT id FROM users",
		result: db.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}, Kinds: [][]db.Kind{{db.KindInt}}},
	})
	am := accepted.(model)

	// B is attempted: history records it before execution.
	am.prepareAndExecuteQuery("SELECT * FROM typo_table")
	if tail := am.queryHistory[len(am.queryHistory)-1]; tail != "SELECT * FROM typo_table" {
		t.Fatalf("test premise broken: history tail = %q", tail)
	}
	// B comes back as an error, so lastResult and lastQuery stay on A.
	failed, _ := am.Update(queryExecutedMsg{seq: am.querySeq, query: "SELECT * FROM typo_table", err: errTestQuery})
	fm := failed.(model)

	if got := fm.lastExecutedQuery(); got != "SELECT id FROM users" {
		t.Fatalf("lastExecutedQuery = %q, want the query of the last accepted result", got)
	}

	fm.mode = normalMode
	result, cmd := fm.updateNormal(runeMsg("b"))
	rm := result.(model)
	if done := cmd().(bringDoneMsg); done.err != nil {
		t.Fatalf("bring failed: %v", done.err)
	}

	got, err := rm.bringSt.adapter.Query(t.Context(),
		`SELECT query FROM `+bring.ProvenanceTable)
	if err != nil {
		t.Fatalf("query provenance: %v", err)
	}
	if got.Rows[0][0] != "SELECT id FROM users" {
		t.Errorf("recorded query = %q, want the query that actually produced the rows", got.Rows[0][0])
	}
}

func TestBring_LastExecutedQueryBeforeAnyResult(t *testing.T) {
	m := newTestModel()
	if got := m.lastExecutedQuery(); got != "" {
		t.Errorf("lastExecutedQuery = %q, want empty before any result is accepted", got)
	}
}

var errTestQuery = errors.New("no such table")

func TestBring_LabelCountsSuccessfulBrings(t *testing.T) {
	m := newTestModel()
	if got := m.bringLabel(); got != "(local bring: 0 tables)" {
		t.Errorf("bringLabel = %q", got)
	}
	m.bringSt.brought = 1
	if got := m.bringLabel(); got != "(local bring: 1 table)" {
		t.Errorf("bringLabel = %q, want the singular form", got)
	}
	m.bringSt.brought = 3
	if got := m.bringLabel(); got != "(local bring: 3 tables)" {
		t.Errorf("bringLabel = %q", got)
	}
}

func TestBring_DoneMsgNamesTheSourceConnection(t *testing.T) {
	m := newTestModel()
	accepted, _ := m.Update(queryExecutedMsg{
		seq:    m.querySeq,
		query:  "SELECT id FROM users",
		conn:   "prod",
		result: db.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}, Kinds: [][]db.Kind{{db.KindInt}}},
	})
	am := accepted.(model)
	am.mode = normalMode

	result, cmd := am.updateNormal(runeMsg("b"))
	rm := result.(model)
	done := cmd().(bringDoneMsg)
	if done.source != "prod" {
		t.Errorf("done.source = %q, want the connection the result came from", done.source)
	}

	updated, _ := rm.Update(done)
	um := updated.(model)
	if um.bringSt.brought != 1 {
		t.Errorf("brought = %d, want 1 after a successful bring", um.bringSt.brought)
	}
	if !strings.Contains(um.statusText, done.name) {
		t.Errorf("status %q should name the new table", um.statusText)
	}

	// A failed bring must not inflate the count shown in the status bar.
	failed, _ := um.Update(bringDoneMsg{name: "t2", err: fmt.Errorf("boom")})
	fm := failed.(model)
	if fm.bringSt.brought != 1 {
		t.Errorf("brought = %d after a failed bring, want it unchanged", fm.bringSt.brought)
	}
}

func TestBring_ProvenanceKeepsTheSourceConnectionAfterASwitch(t *testing.T) {
	// Switching connections leaves lastResult on screen, so the rows still
	// belong to the connection they were fetched from. Pressing b afterwards
	// must record that connection, not whichever one is now active.
	adapter, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	t.Cleanup(func() { adapter.Close() })

	m := NewModel(adapter, "prod.db", "prod.db", "prod", nil, nil, nil, false)
	accepted, _ := m.Update(queryExecutedMsg{
		seq:    m.querySeq,
		query:  "SELECT id FROM users",
		conn:   "prod",
		result: db.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}, Kinds: [][]db.Kind{{db.KindInt}}},
	})
	am := accepted.(model)

	// Switch to another connection without running a query there.
	other, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	t.Cleanup(func() { other.Close() })
	am.connMgr.Register("staging", "staging.db", other)
	if err := am.connMgr.Switch("staging", "staging.db"); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	switched, _ := am.Update(connSwitchedMsg{})
	sm := switched.(model)
	if sm.connMgr.ActiveName() != "staging" {
		t.Fatalf("active connection = %q, want staging", sm.connMgr.ActiveName())
	}
	if len(sm.lastResult.Rows) != 1 {
		t.Fatalf("test premise broken: the switch cleared lastResult")
	}

	sm.mode = normalMode
	result, cmd := sm.updateNormal(runeMsg("b"))
	rm := result.(model)
	done := cmd().(bringDoneMsg)
	if done.err != nil {
		t.Fatalf("bring failed: %v", done.err)
	}

	got, err := rm.bringSt.adapter.Query(t.Context(),
		`SELECT source FROM `+bring.ProvenanceTable)
	if err != nil {
		t.Fatalf("query provenance: %v", err)
	}
	if got.Rows[0][0] != "prod" {
		t.Errorf("recorded source = %q, want prod — the rows never came from staging", got.Rows[0][0])
	}
}

func TestBring_ProvenanceForSidebarInsertedQuery(t *testing.T) {
	// The common path: pick a table in the sidebar, execute what it inserted,
	// then bring the result. prepareAndExecuteQuery records it in history, so
	// provenance must pick it up without any special casing.
	adapter, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	t.Cleanup(func() { adapter.Close() })

	m := NewModel(adapter, "test.db", "test.db", "src", nil, nil, nil, false)
	m.mode = sidebarMode
	m.sidebar.open = true
	m.sidebar.tables = []string{"users"}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sm := next.(model)
	inserted := strings.TrimSpace(sm.textarea.Value())
	if !strings.Contains(inserted, "users") {
		t.Fatalf("sidebar inserted %q, expected a SELECT on users", inserted)
	}

	// Execute it the way INSERT mode does, then accept a result for it.
	sm.prepareAndExecuteQuery(inserted)
	next2, _ := sm.Update(queryExecutedMsg{
		seq:    sm.querySeq,
		query:  inserted,
		conn:   sm.connMgr.ActiveName(),
		result: db.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}, Kinds: [][]db.Kind{{db.KindInt}}},
	})
	sm = next2.(model)

	if got := sm.lastExecutedQuery(); got != inserted {
		t.Fatalf("lastExecutedQuery = %q, want %q", got, inserted)
	}

	sm.mode = normalMode
	result, cmd := sm.updateNormal(runeMsg("b"))
	rm := result.(model)
	done := cmd().(bringDoneMsg)
	if done.err != nil {
		t.Fatalf("bring failed: %v", done.err)
	}
	if done.source != "src" {
		t.Errorf("done.source = %q, want the source connection name", done.source)
	}

	got, err := rm.bringSt.adapter.Query(t.Context(),
		`SELECT query FROM `+bring.ProvenanceTable)
	if err != nil {
		t.Fatalf("query provenance: %v", err)
	}
	if got.Rows[0][0] != inserted {
		t.Errorf("recorded query = %q, want %q", got.Rows[0][0], inserted)
	}
}

func TestBring_ReBringingProvenanceTableIsHarmless(t *testing.T) {
	// Nothing stops a user on the bring DB from selecting _asql_bring and
	// pressing b. The new record must not clobber the existing ones.
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
		Kinds:   [][]db.Kind{{db.KindInt}},
	}

	first, cmd := m.updateNormal(runeMsg("b"))
	fm := first.(model)
	if done := cmd().(bringDoneMsg); done.err != nil {
		t.Fatalf("first bring failed: %v", done.err)
	}

	fm.lastQuery = "SELECT * FROM _asql_bring"
	second, cmd2 := fm.updateNormal(runeMsg("b"))
	sm := second.(model)
	if done := cmd2().(bringDoneMsg); done.err != nil {
		t.Fatalf("second bring failed: %v", done.err)
	}

	got, err := sm.bringSt.adapter.Query(t.Context(),
		`SELECT n, table_name FROM `+bring.ProvenanceTable+` ORDER BY n`)
	if err != nil {
		t.Fatalf("query provenance: %v", err)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("provenance rows = %+v, want 2 distinct records", got.Rows)
	}
	if got.Rows[0][1] != "t1" || got.Rows[1][1] != "t2" {
		t.Errorf("provenance = %+v, want t1 and t2", got.Rows)
	}
}

func TestBring_LabelRefreshesWhileTheBringDBIsActive(t *testing.T) {
	// J refuses to switch when the bring DB is already active, and dbPath is
	// otherwise only recomputed on a switch. Bringing a derived result must
	// still update the count the status bar shows.
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
		Kinds:   [][]db.Kind{{db.KindInt}},
	}

	first, cmd := m.updateNormal(runeMsg("b"))
	fm := first.(model)
	done := cmd().(bringDoneMsg)
	if done.err != nil {
		t.Fatalf("first bring failed: %v", done.err)
	}
	next, _ := fm.Update(done)
	fm = next.(model)

	// Switch to the bring DB, the way J does.
	if err := fm.connMgr.Switch(bringConnName, bringDSN); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	switched, _ := fm.Update(connSwitchedMsg{})
	sm := switched.(model)
	if sm.dbPath != "(local bring: 1 table)" {
		t.Fatalf("dbPath after switch = %q, want the one-table label", sm.dbPath)
	}

	// Bring a second result without leaving the connection.
	sm.mode = normalMode
	second, cmd2 := sm.updateNormal(runeMsg("b"))
	rm := second.(model)
	done2 := cmd2().(bringDoneMsg)
	if done2.err != nil {
		t.Fatalf("second bring failed: %v", done2.err)
	}
	updated, _ := rm.Update(done2)
	um := updated.(model)

	if um.dbPath != "(local bring: 2 tables)" {
		t.Errorf("dbPath = %q, want it refreshed to two tables", um.dbPath)
	}
}
