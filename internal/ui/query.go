package ui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/db"
)

func loadTablesCmd(adapter db.DBAdapter) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
		defer cancel()
		tables, err := adapter.Tables(ctx)
		return tablesLoadedMsg{tables: tables, err: err}
	}
}

// prepareAndExecuteQuery cancels any in-flight query, records the query in
// history, and returns a Cmd that executes it. Callers should use this instead
// of duplicating cancel/history/execute logic.
func (m *model) prepareAndExecuteQuery(query string) tea.Cmd {
	if m.queryCancel != nil {
		m.queryCancel()
	}
	// Add to history (skip duplicates of last entry)
	if query != "" && (len(m.queryHistory) == 0 || m.queryHistory[len(m.queryHistory)-1] != query) {
		m.queryHistory = append(m.queryHistory, query)
		if len(m.queryHistory) > maxHistory {
			m.queryHistory = m.queryHistory[1:]
		}
	}
	m.historyIdx = -1
	ctx, cancel := context.WithCancel(context.Background())
	m.querySeq++
	m.queryCancel = cancel
	// Capture the connection name now rather than when the result arrives: a
	// switch in between bumps querySeq, so this result would be discarded
	// anyway, and the name here is always the one the query actually ran on.
	return executeQueryCmd(ctx, m.activeDB(), m.connMgr.ActiveName(), query, m.querySeq)
}

func executeQueryCmd(parent context.Context, adapter db.DBAdapter, conn, query string, seq uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, queryTimeout)
		defer cancel()

		result, err := adapter.Query(ctx, query)
		return queryExecutedMsg{seq: seq, query: query, conn: conn, result: result, err: err}
	}
}
