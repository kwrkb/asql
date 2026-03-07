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
				m.sidebar.open = true
				m.mode = sidebarMode
				m.textarea.Blur()
				m.sidebar.cursor = 0
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
				m.exportSt.cursor = 0
				m.setStatus("Export mode", false)
			}
		case "c":
			if m.pinned != nil {
				// Toggle off
				m.pinned = nil
				m.comparePane = 0
				m.table.SetStyles(focusedTableStyles())
				m.setStatus("Compare closed", false)
				m.viewportDirty = true
				m.resize()
			} else {
				if len(m.lastResult.Columns) == 0 {
					m.setStatus("No query results to compare", true)
					break
				}
				if m.fullContentWidth() < minWidthForCompare {
					m.setStatus("Terminal too narrow for compare", true)
					break
				}
				m.pinned = m.pinCurrentResult()
				m.comparePane = 1 // focus on right (active) pane
				m.setStatus(m.compareStatusSummary()+" — switch connection and re-execute", false)
				m.resize()
			}
		case "j":
			if m.pinned != nil && m.comparePane == 0 {
				m.pinned.table.MoveDown(1)
			} else {
				m.table.MoveDown(1)
			}
		case "k":
			if m.pinned != nil && m.comparePane == 0 {
				m.pinned.table.MoveUp(1)
			} else {
				m.table.MoveUp(1)
			}
		case "h":
			if m.pinned != nil && m.comparePane == 0 {
				if m.pinned.colCursor > 0 {
					m.pinned.colCursor--
					m.pinned.adjustColOffset(m.comparePaneWidth())
				}
			} else {
				if m.colCursor > 0 {
					m.colCursor--
					m.adjustColOffset()
				}
			}
		case "l":
			if m.pinned != nil && m.comparePane == 0 {
				if len(m.pinned.result.Columns) > 0 && m.pinned.colCursor < len(m.pinned.result.Columns)-1 {
					m.pinned.colCursor++
					m.pinned.adjustColOffset(m.comparePaneWidth())
				}
			} else {
				if len(m.lastResult.Columns) > 0 && m.colCursor < len(m.lastResult.Columns)-1 {
					m.colCursor++
					m.adjustColOffset()
				}
			}
		case "s":
			if m.pinned != nil && m.comparePane == 0 {
				if len(m.pinned.result.Columns) > 0 {
					m.togglePinnedSort()
				}
			} else if len(m.lastResult.Columns) > 0 {
				m.toggleSort()
			}
		case "P":
			m.mode = profileMode
			m.profileSt.cursor = 0
			m.profileSt.naming = false
			m.textarea.Blur()
			m.setStatus("Profile mode", false)
		case "R":
			query := strings.TrimSpace(m.textarea.Value())
			if query == "" {
				m.setStatus("No query to re-execute", true)
				break
			}
			m.setStatus("Re-executing query...", false)
			return m, m.prepareAndExecuteQuery(query)
		case "S":
			m.mode = snippetMode
			m.snippetSt.cursor = 0
			m.snippetSt.naming = false
			m.textarea.Blur()
			m.setStatus("Snippet mode", false)
		}
	case tea.KeyTab:
		if m.pinned != nil {
			if m.comparePane == 0 {
				m.comparePane = 1
				m.pinned.table.SetStyles(unfocusedTableStyles())
				m.table.SetStyles(focusedTableStyles())
			} else {
				m.comparePane = 0
				m.pinned.table.SetStyles(focusedTableStyles())
				m.table.SetStyles(unfocusedTableStyles())
			}
			m.pinned.viewportDirty = true
			m.viewportDirty = true
		}
	case tea.KeyCtrlS:
		return m.enterSnippetNamingMode()
	case tea.KeyCtrlK:
		if m.aiSt.enabled {
			m.mode = aiMode
			m.aiSt.input.Reset()
			m.aiSt.input.Focus()
			m.aiSt.err = ""
			m.aiSt.loading = false
			m.setStatus("AI mode", false)
			return m, textinput.Blink
		}
		m.setStatus("AI not configured", true)
	case tea.KeyEnter:
		if len(m.lastResult.Columns) > 0 && len(m.lastResult.Rows) > 0 {
			m.mode = detailMode
			m.detail.fieldCursor = 0
			m.detail.scroll = 0
			m.setStatus("Detail mode", false)
		}
	case tea.KeyPgUp, tea.KeyPgDown:
		if m.pinned != nil && m.comparePane == 0 {
			m.pinned.table, _ = m.pinned.table.Update(msg)
		} else {
			m.table, _ = m.table.Update(msg)
		}
	case tea.KeyLeft:
		if m.pinned != nil && m.comparePane == 0 {
			if m.pinned.colCursor > 0 {
				m.pinned.colCursor--
				m.pinned.adjustColOffset(m.comparePaneWidth())
			}
		} else if m.colCursor > 0 {
			m.colCursor--
			m.adjustColOffset()
		}
	case tea.KeyRight:
		if m.pinned != nil && m.comparePane == 0 {
			if len(m.pinned.result.Columns) > 0 && m.pinned.colCursor < len(m.pinned.result.Columns)-1 {
				m.pinned.colCursor++
				m.pinned.adjustColOffset(m.comparePaneWidth())
			}
		} else if len(m.lastResult.Columns) > 0 && m.colCursor < len(m.lastResult.Columns)-1 {
			m.colCursor++
			m.adjustColOffset()
		}
	}
	m.syncViewport()
	return m, nil
}
