package ui

import (
	"fmt"
	"testing"

	"github.com/kwrkb/asql/internal/db"
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

func TestBring_FailureRollsBackTableSeqForRetry(t *testing.T) {
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
	if um.bringSt.tableSeq != 0 {
		t.Fatalf("expected tableSeq to roll back to 0 after failure, got %d", um.bringSt.tableSeq)
	}

	// Retrying must generate the same table name, not skip to t2.
	result2, cmd2 := um.updateNormal(runeMsg("b"))
	rm2 := result2.(model)
	done2 := cmd2().(bringDoneMsg)
	if done2.name != "t1" {
		t.Fatalf("expected retry to reuse name t1, got %q", done2.name)
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
