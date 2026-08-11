package ui

import (
	"sync"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/opener"
)

type connection struct {
	name    string
	dsn     string
	adapter db.DBAdapter
}

type connManager struct {
	mu     sync.RWMutex
	conns  []connection
	active int // index of active connection
	// readonly makes every connection opened through Switch refuse writes.
	// It does not apply to Register, which takes adapters asql opened for
	// itself — see the note there.
	readonly bool
}

func newConnManager(name, dsn string, adapter db.DBAdapter, readonly bool) *connManager {
	return &connManager{
		conns: []connection{{
			name:    name,
			dsn:     dsn,
			adapter: adapter,
		}},
		active:   0,
		readonly: readonly,
	}
}

// Active returns the currently active adapter.
func (cm *connManager) Active() db.DBAdapter {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if len(cm.conns) == 0 || cm.active >= len(cm.conns) {
		return nil
	}
	return cm.conns[cm.active].adapter
}

// ActiveName returns the name of the current connection.
func (cm *connManager) ActiveName() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if len(cm.conns) == 0 || cm.active >= len(cm.conns) {
		return ""
	}
	return cm.conns[cm.active].name
}

// ActiveDSN returns the DSN of the current connection.
func (cm *connManager) ActiveDSN() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if len(cm.conns) == 0 || cm.active >= len(cm.conns) {
		return ""
	}
	return cm.conns[cm.active].dsn
}

// Switch switches to the connection with the given name.
// If already connected, just makes it active.
// If not connected, opens a new connection.
func (cm *connManager) Switch(name, dsn string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if already connected
	for i, c := range cm.conns {
		if c.dsn == dsn {
			cm.active = i
			// Update name if different
			cm.conns[i].name = name
			return nil
		}
	}

	// Open new connection
	open := opener.Open
	if cm.readonly {
		open = opener.OpenReadonly
	}
	adapter, err := open(dsn)
	if err != nil {
		return err
	}

	cm.conns = append(cm.conns, connection{
		name:    name,
		dsn:     dsn,
		adapter: adapter,
	})
	cm.active = len(cm.conns) - 1
	return nil
}

// Register adds an already-open adapter as a new connection without making
// it active and without going through opener.Open. Used for adapters created
// internally (e.g. the Bring & Join local SQLite database), which have no
// real DSN the opener could re-derive a connection from.
//
// Registered adapters stay writable even in a readonly session, and that is
// deliberate: the local bring database is asql's own scratch space, and a
// readonly session that could not materialize a result would lose Bring & Join
// entirely. What readonly protects is the databases the user connected to.
func (cm *connManager) Register(name, dsn string, adapter db.DBAdapter) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.conns = append(cm.conns, connection{
		name:    name,
		dsn:     dsn,
		adapter: adapter,
	})
}

// IsConnected checks if a DSN is already connected.
func (cm *connManager) IsConnected(dsn string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, c := range cm.conns {
		if c.dsn == dsn {
			return true
		}
	}
	return false
}

// IsActive checks if a DSN is the currently active connection.
func (cm *connManager) IsActive(dsn string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if len(cm.conns) == 0 || cm.active >= len(cm.conns) {
		return false
	}
	return cm.conns[cm.active].dsn == dsn
}

// CloseAll closes all connections.
func (cm *connManager) CloseAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	for _, c := range cm.conns {
		c.adapter.Close()
	}
	cm.conns = nil
	cm.active = 0
}
