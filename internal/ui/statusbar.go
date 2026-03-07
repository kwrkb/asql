package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// statusHints returns the key-binding hint string for the current mode.
func (m model) statusHints() string {
	if m.queryCancel != nil {
		if m.aiSt.loading {
			return "C-c/Esc:cancel"
		}
		return "C-c:cancel"
	}

	switch m.mode {
	case normalMode:
		if m.pinned != nil {
			return "c:close Tab:switch h/l:col s:sort j/k:row i:insert q:quit"
		} else if m.aiSt.enabled {
			return "c:compare h/l:col s:sort R:re-exec t:tables i:insert e:export S:snippets P:profiles C-k:AI q:quit"
		}
		return "c:compare h/l:col s:sort R:re-exec t:tables i:insert e:export S:snippets P:profiles q:quit"
	case insertMode:
		if m.completion.active {
			return "Tab/C-n:next C-p:prev Enter:accept Esc:cancel"
		}
		return "Tab:complete C-Enter/C-j:exec C-r:search C-l:clear C-p/C-n:hist C-s:save Esc:normal"
	case sidebarMode:
		return "j/k:nav Enter:select Esc:close"
	case aiMode:
		return "Enter:generate Esc:cancel"
	case exportMode:
		return "j/k:nav Enter:select Esc:cancel"
	case detailMode:
		return "j/k:field n/N:row q/Esc:close"
	case historySearchMode:
		return "Enter:select C-p/C-n:nav Esc:cancel"
	case snippetMode:
		if m.snippetSt.naming {
			return "Enter:save Esc:cancel"
		}
		return "j/k:nav Enter:load d:del a:add Esc:close"
	case profileMode:
		if m.profileSt.naming {
			return "Enter:save Esc:cancel"
		}
		return "j/k:nav Enter:connect x:switch+exec d:del a:add Esc:close"
	default:
		return ""
	}
}

// statusPositionInfo returns the column/row position string for the status bar.
func (m model) statusPositionInfo() string {
	if m.pinned != nil && m.comparePane == 0 {
		p := m.pinned
		if len(p.result.Columns) > 0 && len(p.result.Rows) > 0 {
			colName := ""
			if p.colCursor < len(p.result.Columns) {
				colName = p.result.Columns[p.colCursor]
			}
			_, visEnd := p.visibleColumnRange(m.comparePaneWidth())
			visCount := visEnd - p.colOffset
			totalCols := len(p.result.Columns)
			if visCount < totalCols {
				return fmt.Sprintf("col:%s [%d/%d] %d/%d", sanitize(colName), p.colCursor+1, totalCols, p.table.Cursor()+1, len(p.result.Rows))
			}
			return fmt.Sprintf("col:%s %d/%d", sanitize(colName), p.table.Cursor()+1, len(p.result.Rows))
		}
		return ""
	}

	if len(m.lastResult.Columns) > 0 && len(m.lastResult.Rows) > 0 {
		colName := ""
		if m.colCursor < len(m.lastResult.Columns) {
			colName = m.lastResult.Columns[m.colCursor]
		}
		_, visEnd := m.visibleColumnRange()
		visCount := visEnd - m.colOffset
		totalCols := len(m.lastResult.Columns)
		if visCount < totalCols {
			return fmt.Sprintf("col:%s [%d/%d] %d/%d", sanitize(colName), m.colCursor+1, totalCols, m.table.Cursor()+1, len(m.lastResult.Rows))
		}
		return fmt.Sprintf("col:%s %d/%d", sanitize(colName), m.table.Cursor()+1, len(m.lastResult.Rows))
	}
	return ""
}

// statusConnectionLabel returns the DB type and connection name label.
func (m model) statusConnectionLabel() string {
	dbLabel := strings.ToUpper(sanitize(m.activeDB().Type()))
	connName := sanitize(m.connMgr.ActiveName())

	if m.pinned != nil {
		pinnedName := sanitize(m.pinned.connName)
		if pinnedName == "" {
			pinnedName = "pinned"
		}
		activeName := connName
		if activeName == "" {
			activeName = "active"
		}
		connName = pinnedName + " | " + activeName
	}

	if connName != "" {
		return connName + ":" + dbLabel
	}
	return dbLabel
}

func (m model) renderStatusBar() string {
	modeStr := m.modeStyle.Render(string(m.mode))

	msgStyle := m.messageStyle
	if m.statusError {
		msgStyle = msgStyle.Foreground(errorColor)
	} else if strings.TrimSpace(m.statusText) != "" {
		msgStyle = msgStyle.Foreground(successColor)
	}

	hintStyle := lipgloss.NewStyle().Foreground(mutedTextColor).Background(statusBackground).Padding(0, 1)
	dbLabelStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(keywordColor).Background(statusBackground)
	posStyle := lipgloss.NewStyle().Foreground(textColor).Background(statusBackground).Padding(0, 1)

	center := dbLabelStyle.Render("["+m.statusConnectionLabel()+"]") + m.pathStyle.Render(m.dbPath)
	middle := msgStyle.Render(sanitize(m.statusText))
	pos := posStyle.Render(m.statusPositionInfo())
	right := hintStyle.Render(m.statusHints())

	leftPart := lipgloss.JoinHorizontal(lipgloss.Left, modeStr, center, middle)
	rightPart := lipgloss.JoinHorizontal(lipgloss.Right, pos, right)
	gap := max(m.width-lipgloss.Width(leftPart)-lipgloss.Width(rightPart), 0)
	bar := leftPart + strings.Repeat(" ", gap) + rightPart

	return lipgloss.NewStyle().
		Width(m.width).
		Background(statusBackground).
		Render(bar)
}
