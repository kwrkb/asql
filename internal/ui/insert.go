package ui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const maxHistory = 100

func (m model) updateInsert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = normalMode
		m.textarea.Blur()
		m.setStatus("Normal mode", false)
		m.syncViewport()
		return m, nil
	case tea.KeyCtrlS:
		query := strings.TrimSpace(m.textarea.Value())
		if query == "" {
			m.setStatus("No query to save", true)
			return m, nil
		}
		m.snippetPrevMode = insertMode
		m.mode = snippetMode
		m.snippetNaming = true
		m.snippetInput.Reset()
		m.snippetInput.Focus()
		m.textarea.Blur()
		m.setStatus("Save snippet", false)
		return m, textinput.Blink
	case tea.KeyCtrlJ:
		query := strings.TrimSpace(m.textarea.Value())
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
		m.setStatus("Executing query...", false)
		return m, executeQueryCmd(ctx, m.db, query, m.querySeq)
	case tea.KeyCtrlP:
		if len(m.queryHistory) == 0 {
			return m, nil
		}
		if m.historyIdx == -1 {
			m.historyDraft = m.textarea.Value()
			m.historyIdx = len(m.queryHistory) - 1
		} else if m.historyIdx > 0 {
			m.historyIdx--
		}
		m.textarea.SetValue(m.queryHistory[m.historyIdx])
		return m, nil
	case tea.KeyCtrlN:
		if m.historyIdx == -1 {
			return m, nil
		}
		if m.historyIdx < len(m.queryHistory)-1 {
			m.historyIdx++
			m.textarea.SetValue(m.queryHistory[m.historyIdx])
		} else {
			m.historyIdx = -1
			m.textarea.SetValue(m.historyDraft)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.syncViewport()
	return m, cmd
}
