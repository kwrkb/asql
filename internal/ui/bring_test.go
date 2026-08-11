package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/bring"
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

func TestBring_ProvenanceUsesLastExecutedQuery(t *testing.T) {
	m := newTestModel()
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
	}
	m.queryHistory = []string{"SELECT 1", "SELECT id FROM users"}
	// The editor has moved on since the result came back; provenance must
	// describe the query that produced the data, not what is being typed now.
	m.textarea.SetValue("SELECT * FROM something_else")

	if got := m.lastExecutedQuery(); got != "SELECT id FROM users" {
		t.Errorf("lastExecutedQuery = %q, want the tail of queryHistory", got)
	}

	result, cmd := m.updateNormal(runeMsg("b"))
	rm := result.(model)
	done, ok := cmd().(bringDoneMsg)
	if !ok {
		t.Fatalf("expected bringDoneMsg, got %T", done)
	}
	if done.err != nil {
		t.Fatalf("bring failed: %v", done.err)
	}

	got, err := rm.bringSt.adapter.Query(t.Context(),
		`SELECT table_name, source, query FROM `+bring.ProvenanceTable)
	if err != nil {
		t.Fatalf("query provenance: %v", err)
	}
	if len(got.Rows) != 1 {
		t.Fatalf("expected 1 provenance row, got %+v", got.Rows)
	}
	if got.Rows[0][0] != "t1" {
		t.Errorf("table_name = %q, want t1", got.Rows[0][0])
	}
	if got.Rows[0][2] != "SELECT id FROM users" {
		t.Errorf("query = %q, want the last executed query", got.Rows[0][2])
	}
}

func TestBring_LastExecutedQueryEmptyHistory(t *testing.T) {
	m := newTestModel()
	if got := m.lastExecutedQuery(); got != "" {
		t.Errorf("lastExecutedQuery = %q, want empty for an empty history", got)
	}
}

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
	m.mode = normalMode
	m.lastResult = db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
	}

	result, cmd := m.updateNormal(runeMsg("b"))
	rm := result.(model)
	done := cmd().(bringDoneMsg)
	if done.source != rm.connMgr.ActiveName() {
		t.Errorf("done.source = %q, want the active connection name %q", done.source, rm.connMgr.ActiveName())
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
