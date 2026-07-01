package ui

import (
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

	result, _ := m.updateNormal(runeMsg("b"))
	rm := result.(model)
	if rm.bringSt.adapter == nil {
		t.Fatal("expected bring adapter to be initialized after first bring")
	}
	if got := rm.bringSt.tables; len(got) != 1 || got[0] != "t1" {
		t.Fatalf("expected [t1], got %v", got)
	}
	if rm.statusError {
		t.Errorf("expected success status, got error: %q", rm.statusText)
	}
	// connManager started with 1 (nil) connection; Register should add a 2nd.
	if got := len(rm.connMgr.conns); got != 2 {
		t.Fatalf("expected 2 connections after first bring, got %d", got)
	}

	result2, _ := rm.updateNormal(runeMsg("b"))
	rm2 := result2.(model)
	if got := rm2.bringSt.tables; len(got) != 2 || got[1] != "t2" {
		t.Fatalf("expected [t1 t2], got %v", got)
	}
	// The bring adapter is reused, not re-registered.
	if got := len(rm2.connMgr.conns); got != 2 {
		t.Fatalf("expected connection count to stay at 2 on second bring, got %d", got)
	}
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
