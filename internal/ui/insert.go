package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const maxHistory = 100

func (m model) updateInsert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle completion-active keys first
	if m.completion.active {
		switch msg.Type {
		case tea.KeyTab, tea.KeyCtrlN, tea.KeyDown:
			if len(m.completion.items) > 0 {
				m.completion.cursor = (m.completion.cursor + 1) % len(m.completion.items)
			}
			return m, nil
		case tea.KeyCtrlP, tea.KeyUp:
			if len(m.completion.items) > 0 {
				m.completion.cursor = (m.completion.cursor - 1 + len(m.completion.items)) % len(m.completion.items)
			}
			return m, nil
		case tea.KeyEnter:
			m.acceptCompletion()
			return m, nil
		case tea.KeyEsc:
			m.closeCompletion()
			return m, nil
		default:
			// Close completion and fall through to normal handling
			m.closeCompletion()
		}
	}

	switch msg.Type {
	case tea.KeyTab:
		m.triggerCompletion()
		return m, nil
	case tea.KeyEsc:
		m.mode = normalMode
		m.textarea.Blur()
		m.setStatus("Normal mode", false)
		m.syncViewport()
		return m, nil
	case tea.KeyCtrlS:
		return m.enterSnippetNamingMode()
	case tea.KeyCtrlJ:
		query := strings.TrimSpace(m.textarea.Value())
		m.setStatus("Executing query...", false)
		return m, m.prepareAndExecuteQuery(query)
	case tea.KeyCtrlR:
		return m.enterHistorySearchMode()
	case tea.KeyCtrlL:
		m.textarea.SetValue("")
		m.historyIdx = -1
		m.historyDraft = ""
		return m, nil
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
