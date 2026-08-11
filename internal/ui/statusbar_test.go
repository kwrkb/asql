package ui

import (
	"testing"

	"github.com/kwrkb/asql/internal/db/readonly"
	"github.com/kwrkb/asql/internal/db/sqlite"
)

// The read-only marker is not decoration: a user who cannot see that the
// connection refuses writes has lost half of what --readonly is for.
func TestStatusConnectionLabelShowsReadonly(t *testing.T) {
	adapter, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer adapter.Close()

	writable := newTestModel()
	writable.connMgr = newConnManager("prod", "prod.db", adapter, false)
	if got := writable.statusConnectionLabel(); got != "prod:SQLITE" {
		t.Errorf("writable label = %q, want %q", got, "prod:SQLITE")
	}

	guarded := newTestModel()
	guarded.connMgr = newConnManager("prod", "prod.db", readonly.Wrap(adapter), true)
	if got := guarded.statusConnectionLabel(); got != "prod:SQLITE ro" {
		t.Errorf("readonly label = %q, want %q", got, "prod:SQLITE ro")
	}
}
