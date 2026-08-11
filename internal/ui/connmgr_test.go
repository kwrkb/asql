package ui

import (
	"context"
	"testing"

	"github.com/kwrkb/asql/internal/db/opener"
	"github.com/kwrkb/asql/internal/db/readonly"
	"github.com/kwrkb/asql/internal/db/sqlite"
)

func TestConnManager(t *testing.T) {
	// Create a temp SQLite adapter for testing
	adapter, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer adapter.Close()

	cm := newConnManager("test", ":memory:", adapter, false)

	t.Run("Active returns initial adapter", func(t *testing.T) {
		if cm.Active() != adapter {
			t.Error("expected initial adapter")
		}
	})

	t.Run("ActiveName returns initial name", func(t *testing.T) {
		if cm.ActiveName() != "test" {
			t.Errorf("expected 'test', got %q", cm.ActiveName())
		}
	})

	t.Run("IsConnected for initial DSN", func(t *testing.T) {
		if !cm.IsConnected(":memory:") {
			t.Error("expected initial DSN to be connected")
		}
		if cm.IsConnected("other.db") {
			t.Error("unexpected DSN should not be connected")
		}
	})

	t.Run("IsActive for initial DSN", func(t *testing.T) {
		if !cm.IsActive(":memory:") {
			t.Error("expected initial DSN to be active")
		}
		if cm.IsActive("other.db") {
			t.Error("unexpected DSN should not be active")
		}
	})

	t.Run("Switch to same DSN reuses connection", func(t *testing.T) {
		err := cm.Switch("renamed", ":memory:")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cm.conns) != 1 {
			t.Errorf("expected 1 connection, got %d", len(cm.conns))
		}
		if cm.ActiveName() != "renamed" {
			t.Errorf("expected name 'renamed', got %q", cm.ActiveName())
		}
	})

	t.Run("Switch to new DSN opens new connection", func(t *testing.T) {
		// Use a file-based temp DB to avoid :memory: conflict
		err := cm.Switch("second", "file::memory:?cache=shared&_fk=1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cm.conns) != 2 {
			t.Errorf("expected 2 connections, got %d", len(cm.conns))
		}
		if cm.ActiveName() != "second" {
			t.Errorf("expected name 'second', got %q", cm.ActiveName())
		}
		if cm.IsActive(":memory:") {
			t.Error("first connection should not be active")
		}
	})

	t.Run("Switch back to first connection", func(t *testing.T) {
		err := cm.Switch("test", ":memory:")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cm.Active() != adapter {
			t.Error("expected to switch back to first adapter")
		}
		if len(cm.conns) != 2 {
			t.Errorf("expected 2 connections (cached), got %d", len(cm.conns))
		}
	})

	t.Run("CloseAll closes all connections", func(t *testing.T) {
		cm.CloseAll()
		if len(cm.conns) != 0 {
			t.Errorf("expected 0 connections after CloseAll, got %d", len(cm.conns))
		}
	})
}

func TestConnManagerRegister(t *testing.T) {
	base, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer base.Close()
	cm := newConnManager("test", ":memory:", base, false)

	bring, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open bring sqlite: %v", err)
	}
	defer bring.Close()

	cm.Register("local", "asql-bring", bring)
	if len(cm.conns) != 2 {
		t.Fatalf("expected 2 connections after Register, got %d", len(cm.conns))
	}
	if cm.Active() != base {
		t.Error("Register must not change the active connection")
	}

	t.Run("Switch to registered DSN reuses it without reopening", func(t *testing.T) {
		if err := cm.Switch("local", "asql-bring"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cm.conns) != 2 {
			t.Errorf("expected Switch to reuse the registered connection, got %d conns", len(cm.conns))
		}
		if cm.Active() != bring {
			t.Error("expected the registered bring adapter to become active")
		}
	})
}

// A readonly session must guard the connections it opens later too — the
// second database a user switches to is as much a production database as the
// first.
func TestConnManagerReadonlySession(t *testing.T) {
	dir := t.TempDir()
	firstPath := dir + "/first.db"
	secondPath := dir + "/second.db"
	for _, path := range []string{firstPath, secondPath} {
		conn, err := sqlite.Open(path)
		if err != nil {
			t.Fatalf("failed to create %s: %v", path, err)
		}
		if _, err := conn.Query(context.Background(), "CREATE TABLE t (a TEXT)"); err != nil {
			t.Fatalf("failed to seed %s: %v", path, err)
		}
		conn.Close()
	}

	first, err := opener.OpenReadonly(firstPath)
	if err != nil {
		t.Fatalf("OpenReadonly: %v", err)
	}
	cm := newConnManager("first", firstPath, first, true)
	defer cm.CloseAll()

	if err := cm.Switch("second", secondPath); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if !readonly.IsWrapped(cm.Active()) {
		t.Fatal("a connection opened during a readonly session is not guarded")
	}

	// The local bring database is asql's own scratch space and stays writable,
	// otherwise a readonly session could not bring anything at all.
	bring, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open bring sqlite: %v", err)
	}
	cm.Register("local", "asql-bring", bring)
	if err := cm.Switch("local", "asql-bring"); err != nil {
		t.Fatalf("Switch to bring: %v", err)
	}
	if readonly.IsWrapped(cm.Active()) {
		t.Fatal("the local bring database was guarded; Bring & Join would stop working")
	}
	if _, err := cm.Active().Query(context.Background(), "CREATE TABLE local_t (a TEXT)"); err != nil {
		t.Fatalf("the bring database refused a write: %v", err)
	}
}

// A writable session must not pick up the guard by accident.
func TestConnManagerWritableSessionStaysUnguarded(t *testing.T) {
	base, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	cm := newConnManager("test", ":memory:", base, false)
	defer cm.CloseAll()

	path := t.TempDir() + "/other.db"
	if err := cm.Switch("other", path); err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if readonly.IsWrapped(cm.Active()) {
		t.Fatal("a writable session opened a guarded connection")
	}
}
