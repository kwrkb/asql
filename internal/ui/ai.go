package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/asql/internal/ai"
	"github.com/kwrkb/asql/internal/db"
)

const aiRequestTimeout = 30 * time.Second

func (m model) updateAI(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.aiSt.loading {
		if msg.Type == tea.KeyEsc {
			m.cancelAI()
			m.aiSt.input.Blur()
			m.mode = normalMode
			m.setStatus("Cancelled", false)
			return m, nil
		}
		return m, nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		m.aiSt.input.Blur()
		m.mode = normalMode
		m.aiSt.err = ""
		m.setStatus("Normal mode", false)
		return m, nil
	case tea.KeyEnter:
		prompt := strings.TrimSpace(m.aiSt.input.Value())
		if prompt == "" {
			return m, nil
		}
		m.cancelAI()
		ctx, cancel := context.WithCancel(context.Background())
		m.aiSt.cancel = cancel
		m.aiSt.loading = true
		m.aiSt.err = ""
		return m, tea.Batch(m.aiSt.spinner.Tick, generateSQLCmd(ctx, m.aiSt.client, m.activeDB(), prompt, m.aiSt.seq))
	}
	var cmd tea.Cmd
	m.aiSt.input, cmd = m.aiSt.input.Update(msg)
	return m, cmd
}

// cancelAI stops an in-flight AI request and reports whether there was one.
// The seq bump runs either way: it also discards a response that completed
// just before the cancel, whose aiResponseMsg would otherwise overwrite the
// editor and force INSERT mode after "Cancelled".
func (m *model) cancelAI() bool {
	m.aiSt.seq++
	m.aiSt.loading = false
	if m.aiSt.cancel == nil {
		return false
	}
	m.aiSt.cancel()
	m.aiSt.cancel = nil
	return true
}

func generateSQLCmd(parent context.Context, client *ai.Client, adapter db.DBAdapter, prompt string, seq uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, aiRequestTimeout)
		defer cancel()

		schema, err := adapter.Schema(ctx)
		if err != nil {
			return aiResponseMsg{seq: seq, err: fmt.Errorf("fetching schema: %w", err)}
		}

		sql, err := client.GenerateSQL(ctx, adapter.Type(), schema, prompt)
		return aiResponseMsg{seq: seq, sql: sql, err: err}
	}
}

func (m model) renderWithAIOverlay(background string) string {
	modalWidth := calcModalWidth(m.width, 60)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor).
		MarginBottom(1)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 2).
		Width(modalWidth).
		Background(panelBackground)

	var content string
	if m.aiSt.loading {
		content = titleStyle.Render("AI Generating SQL...") + "\n" + m.aiSt.spinner.View() + " Thinking..."
	} else {
		content = titleStyle.Render("AI Assistant (Text-to-SQL)") + "\n" + m.aiSt.input.View()
		if m.aiSt.err != "" {
			errStyle := lipgloss.NewStyle().Foreground(errorColor).MarginTop(1)
			content += "\n" + errStyle.Render(m.aiSt.err)
		}
	}

	modal := boxStyle.Render(content)

	return overlayModal(m.width, background, modal)
}
