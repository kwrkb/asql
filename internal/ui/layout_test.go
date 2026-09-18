package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/kwrkb/asql/internal/db"
)

// Every screen must fill the terminal exactly. lipgloss sizes Width and Height
// without the border, so a bordered block handed the full size renders larger
// than asked and the renderer clips it: the result panel lost its right and
// bottom edges, compare mode ran its second pane off the screen, and the
// sidebar pushed the main area one cell past the right edge. A status bar that
// wraps grows the view the same way and loses its own tail to the crop.
func TestViewFitsTerminal(t *testing.T) {
	sizes := []struct{ w, h int }{{120, 30}, {100, 24}, {80, 24}, {50, 20}}
	setups := []struct {
		name  string
		setup func(m *model)
	}{
		{"insert", func(m *model) { m.mode = insertMode }},
		// AI enabled gives normal mode its longest hint string.
		{"normal", func(m *model) { m.mode = normalMode; m.aiSt.enabled = true }},
		{"sidebar", func(m *model) {
			m.mode = sidebarMode
			m.sidebar.open = true
			m.sidebar.tables = []string{"orders", "users"}
		}},
		{"compare", func(m *model) {
			m.mode = normalMode
			m.pinned = m.pinCurrentResult()
			m.comparePane = 1
		}},
	}

	for _, sz := range sizes {
		for _, s := range setups {
			t.Run(fmt.Sprintf("%s/%dx%d", s.name, sz.w, sz.h), func(t *testing.T) {
				m := newTestModel()
				m.connMgr = newConnManager("test", "", &stubAdapter{}, false)
				// A path long enough to overflow the status bar on its own.
				m.dbPath = "/var/lib/data/" + strings.Repeat("nested/", 12) + "warehouse.db"
				m.applyResult(db.QueryResult{
					Columns: []string{"id", "name", "city"},
					Rows:    [][]string{{"1", "alice", "Tokyo"}, {"2", "日本語太郎", "札幌"}},
					Message: "2 row(s) returned",
				})
				s.setup(m)
				m.width, m.height = sz.w, sz.h
				m.resize()
				m.setStatus("2 row(s) returned", false)

				lines := strings.Split(ansiRe.ReplaceAllString(m.View(), ""), "\n")
				if len(lines) != m.height {
					t.Errorf("view is %d lines, want exactly %d", len(lines), m.height)
				}
				for i, l := range lines {
					if w := ansi.StringWidth(l); w != m.width {
						t.Errorf("line %d is %d cells wide, want %d: %q", i, w, m.width, l)
					}
				}
				if len(lines) < 2 {
					return
				}
				// The row above the status bar is the bottom edge of the
				// results; its right corner is what the viewport used to clip.
				if above := strings.TrimRight(lines[len(lines)-2], " "); !strings.HasSuffix(above, "╯") {
					t.Errorf("results bottom-right corner is missing: %q", above)
				}
				if !strings.Contains(lines[len(lines)-1], strings.TrimSpace(string(m.mode))) {
					t.Errorf("last line is not the status bar: %q", lines[len(lines)-1])
				}
			})
		}
	}
}

// The status bar must stay one row whatever it has to show; hints give way
// first, and a message too long for even that is cut rather than wrapped.
func TestStatusBarStaysOneRow(t *testing.T) {
	m := newTestModel()
	m.connMgr = newConnManager("test", "", &stubAdapter{}, false)
	m.mode = normalMode
	m.dbPath = "db.sqlite"
	m.setStatus(strings.Repeat("a very long message ", 10), false)

	for w := 1; w <= 200; w++ {
		m.width = w
		bar := m.renderStatusBar()
		if h := strings.Count(bar, "\n") + 1; h != 1 {
			t.Fatalf("width %d: status bar is %d rows", w, h)
		}
		if got := ansi.StringWidth(bar); got != w {
			t.Fatalf("width %d: status bar is %d cells wide", w, got)
		}
	}

	m.setStatus("ok", false)
	m.width = 60
	bar := ansiRe.ReplaceAllString(m.renderStatusBar(), "")
	if !strings.Contains(bar, "…") {
		t.Errorf("hints that do not fit should be cut with an ellipsis: %q", bar)
	}
	if !strings.Contains(bar, "NORMAL") || !strings.Contains(bar, "ok") {
		t.Errorf("mode and message must survive the cut: %q", bar)
	}
}
