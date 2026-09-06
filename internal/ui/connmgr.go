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

// Prepare opens the connection for dsn — or finds the one already open for it
// — and returns its index, without changing which connection is active.
//
// Opening and committing are separate steps because the UI runs the open in a
// goroutine and stays interactive meanwhile: a second switch can be started
// before the first returns, and if opening also committed, the loser would
// move the active connection after the model had already been resynchronized
// to the winner. Queries would then run against a connection the status bar,
// the DSN, the table cache and the connection generation all disagree with.
// Only the completion that survives the sequence check in Update calls
// Activate, so those never drift apart.
func (cm *connManager) Prepare(name, dsn string) (int, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Already connected: keep the adapter, take the (possibly new) name.
	for i, c := range cm.conns {
		if c.dsn == dsn {
			cm.conns[i].name = name
			return i, nil
		}
	}

	open := opener.Open
	if cm.readonly {
		open = opener.OpenReadonly
	}
	adapter, err := open(dsn)
	if err != nil {
		return 0, err
	}

	cm.conns = append(cm.conns, connection{
		name:    name,
		dsn:     dsn,
		adapter: adapter,
	})
	return len(cm.conns) - 1, nil
}

// Activate makes the connection at idx the active one. Prepare only ever
// appends, so an index it handed out stays valid until CloseAll; anything out
// of range is ignored rather than panicking a running TUI.
func (cm *connManager) Activate(idx int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if idx < 0 || idx >= len(cm.conns) {
		return
	}
	cm.active = idx
}

// Switch opens the connection for dsn and makes it active in one step.
//
// The UI's own switch path must not use this: it opens in a goroutine while
// the UI stays live, and committing inside that goroutine is exactly the race
// Prepare and Activate exist to separate. This is for callers that do both on
// the same thread.
func (cm *connManager) Switch(name, dsn string) error {
	idx, err := cm.Prepare(name, dsn)
	if err != nil {
		return err
	}
	cm.Activate(idx)
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
