package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/db/bring"
)

const (
	bringConnName = "local"
	// bringDSN is a sentinel DSN for the local Bring & Join database, not a
	// real connection string. It is prefixed with a NUL byte, which no shell
	// can pass through argv and no real DSN can contain, so a user-supplied
	// DSN (CLI arg or saved profile) can never collide with it — see
	// IsActive/Switch usage below and the display-label handling in
	// model.go's connSwitchedMsg case.
	bringDSN = "\x00asql-bring"
)

// bringDoneMsg reports the outcome of materializing a result into the local
// bring database (see bringCurrentResult).
type bringDoneMsg struct {
	name      string
	source    string // connection the result came from, for the status line
	cols      int
	rows      int
	truncated bool
	err       error
}

// bringCurrentResult materializes the current query result into a new table
// in the session's local SQLite "bring" database, creating that database
// lazily on first use. The active connection and editor are left unchanged
// so the user can keep exploring the source DB or bring more results.
//
// The actual CREATE TABLE + INSERT work runs in a tea.Cmd (like query
// execution in query.go), not inline in Update, since inserting a large
// result can take long enough to freeze the TUI if run synchronously.
func (m model) bringCurrentResult() (tea.Model, tea.Cmd) {
	if len(m.lastResult.Columns) == 0 {
		m.setStatus("No query results to bring", true)
		return m, nil
	}

	if m.bringSt.adapter == nil {
		conn, adapter, err := bring.Open()
		if err != nil {
			m.setStatus(fmt.Sprintf("Bring DB init failed: %v", err), true)
			return m, nil
		}
		m.bringSt.conn = conn
		m.bringSt.adapter = adapter
		m.connMgr.Register(bringConnName, bringDSN, adapter)
	}

	m.bringSt.tableSeq++
	src := bring.Source{
		Seq:   m.bringSt.tableSeq,
		Table: fmt.Sprintf("t%d", m.bringSt.tableSeq),
		Conn:  m.connMgr.ActiveName(),
		Query: m.lastExecutedQuery(),
	}
	m.setStatus(fmt.Sprintf("Bringing as %s...", src.Table), false)

	conn := m.bringSt.conn
	quote := m.bringSt.adapter.QuoteIdentifier
	result := m.lastResult
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
		defer cancel()
		err := bring.Materialize(ctx, conn, quote, src, result)
		return bringDoneMsg{
			name:      src.Table,
			source:    src.Conn,
			cols:      len(result.Columns),
			rows:      len(result.Rows),
			truncated: result.Truncated,
			err:       err,
		}
	}
}

// lastExecutedQuery returns the query that produced lastResult.
//
// It reads m.lastQuery, which is set only when a queryExecutedMsg is accepted.
// Neither the editor nor the tail of queryHistory would do: the editor may have
// been edited since the result came back, and queryHistory records every query
// *attempted*, so a query that failed, was cancelled, or is still in flight
// would be credited with the previous query's rows.
func (m model) lastExecutedQuery() string {
	return m.lastQuery
}

// bringLabel is what the status bar shows in place of a DSN while the local
// bring database is active. bringDSN is a NUL-prefixed sentinel, not a
// displayable connection string, and "how many tables have I brought" is the
// one piece of state that is otherwise invisible from the status bar.
func (m model) bringLabel() string {
	if m.bringSt.brought == 1 {
		return "(local bring: 1 table)"
	}
	return fmt.Sprintf("(local bring: %d tables)", m.bringSt.brought)
}

// switchToBring activates the local Bring & Join SQLite database as the
// current connection, reusing the same Switch/connSwitchedMsg path as
// profile switching (internal/ui/profile.go:switchProfile) so the sidebar,
// tab-completion, and status bar all update through existing plumbing.
func (m model) switchToBring() (tea.Model, tea.Cmd) {
	if m.bringSt.adapter == nil {
		m.setStatus("Nothing brought yet — press b on a result first", true)
		return m, nil
	}
	if m.connMgr.IsActive(bringDSN) {
		m.setStatus("Already on local (bring) connection", false)
		return m, nil
	}
	m.setStatus("Switching to local (bring) connection...", false)
	cm := m.connMgr
	return m, func() tea.Msg {
		err := cm.Switch(bringConnName, bringDSN)
		return connSwitchedMsg{err: err}
	}
}
