package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/db/bring"
)

const (
	bringConnName = "local"
	bringDSN      = "asql-bring"
)

// bringCurrentResult materializes the current query result into a new table
// in the session's local SQLite "bring" database, creating that database
// lazily on first use. The active connection and editor are left unchanged
// so the user can keep exploring the source DB or bring more results.
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
	name := fmt.Sprintf("t%d", m.bringSt.tableSeq)

	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()
	if err := bring.Materialize(ctx, m.bringSt.conn, m.bringSt.adapter.QuoteIdentifier, name, m.lastResult); err != nil {
		m.bringSt.tableSeq--
		m.setStatus(fmt.Sprintf("Bring failed: %v", err), true)
		return m, nil
	}

	m.bringSt.tables = append(m.bringSt.tables, name)
	msg := fmt.Sprintf("Brought as %s (%d cols, %d rows)", name, len(m.lastResult.Columns), len(m.lastResult.Rows))
	if m.lastResult.Truncated {
		msg += " [source truncated]"
	}
	m.setStatus(msg, false)
	return m, nil
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
