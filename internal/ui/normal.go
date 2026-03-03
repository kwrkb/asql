package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		if msg.Alt {
			break
		}
		switch string(msg.Runes) {
		case "q":
			return m, tea.Quit
		case "i":
			m.mode = insertMode
			m.textarea.Focus()
			m.setStatus("Insert mode", false)
		case "t":
			if m.width >= minWidthForSidebar {
				m.sidebarOpen = true
				m.mode = sidebarMode
				m.textarea.Blur()
				m.sidebarCursor = 0
				m.setStatus("Sidebar", false)
				m.resize()
			} else {
				m.setStatus("Terminal too narrow for sidebar", true)
			}
		case "e":
			if len(m.lastResult.Columns) == 0 {
				m.setStatus("No query results to export", true)
			} else {
				m.mode = exportMode
				m.exportCursor = 0
				m.setStatus("Export mode", false)
			}
		case "j":
			m.table.MoveDown(1)
		case "k":
			m.table.MoveUp(1)
		case "h":
			if m.colCursor > 0 {
				m.colCursor--
			}
		case "l":
			if len(m.lastResult.Columns) > 0 && m.colCursor < len(m.lastResult.Columns)-1 {
				m.colCursor++
			}
		case "s":
			if len(m.lastResult.Columns) > 0 {
				m.toggleSort()
			}
		case "S":
			m.mode = snippetMode
			m.snippetCursor = 0
			m.snippetNaming = false
			m.textarea.Blur()
			m.setStatus("Snippet mode", false)
		}
	case tea.KeyCtrlS:
		query := strings.TrimSpace(m.textarea.Value())
		if query == "" {
			m.setStatus("No query to save", true)
			return m, nil
		}
		m.snippetPrevMode = normalMode
		m.mode = snippetMode
		m.snippetNaming = true
		m.snippetInput.Reset()
		m.snippetInput.Focus()
		m.textarea.Blur()
		m.setStatus("Save snippet", false)
		return m, textinput.Blink
	case tea.KeyCtrlK:
		if m.aiEnabled {
			m.mode = aiMode
			m.aiInput.Reset()
			m.aiInput.Focus()
			m.aiError = ""
			m.aiLoading = false
			m.setStatus("AI mode", false)
			return m, textinput.Blink
		}
		m.setStatus("AI not configured", true)
	case tea.KeyEnter:
		if len(m.lastResult.Columns) > 0 && len(m.lastResult.Rows) > 0 {
			m.mode = detailMode
			m.detailFieldCursor = 0
			m.detailScroll = 0
			m.setStatus("Detail mode", false)
		}
	case tea.KeyPgUp, tea.KeyPgDown:
		m.table, _ = m.table.Update(msg)
	case tea.KeyLeft:
		if m.colCursor > 0 {
			m.colCursor--
		}
	case tea.KeyRight:
		if len(m.lastResult.Columns) > 0 && m.colCursor < len(m.lastResult.Columns)-1 {
			m.colCursor++
		}
	}
	m.syncViewport()
	return m, nil
}
