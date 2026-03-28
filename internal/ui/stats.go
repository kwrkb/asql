package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kwrkb/asql/internal/db"
)

func computeColumnStats(result db.QueryResult) []columnStat {
	stats := make([]columnStat, len(result.Columns))
	rowCount := len(result.Rows)

	for i, col := range result.Columns {
		s := columnStat{Name: col}
		if i < len(result.ColumnTypes) {
			s.Type = result.ColumnTypes[i]
		}

		distinct := make(map[string]struct{})
		firstNonNull := true

		for _, row := range result.Rows {
			val := row[i]
			if val == "NULL" {
				s.NullCnt++
				continue
			}
			distinct[val] = struct{}{}
			if firstNonNull {
				s.Min = val
				s.Max = val
				firstNonNull = false
			} else {
				if smartCompare(val, s.Min) < 0 {
					s.Min = val
				}
				if smartCompare(val, s.Max) > 0 {
					s.Max = val
				}
			}
		}

		s.Distinct = len(distinct)
		if rowCount > 0 {
			s.NullRate = float64(s.NullCnt) / float64(rowCount)
		}

		if detectDateColumn(s.Type) || looksLikeDate(result.Rows, i) {
			s.Sparkline = computeSparkline(result.Rows, i)
		}

		stats[i] = s
	}
	return stats
}

func (m model) updateStats(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = normalMode
		m.setStatus("", false)
	case tea.KeyRunes:
		if msg.Alt {
			break
		}
		switch string(msg.Runes) {
		case "q":
			m.mode = normalMode
			m.setStatus("", false)
		case "j":
			moveCursor(&m.statsSt.cursor, len(m.statsSt.stats), 1)
		case "k":
			moveCursor(&m.statsSt.cursor, len(m.statsSt.stats), -1)
		}
	case tea.KeyDown:
		moveCursor(&m.statsSt.cursor, len(m.statsSt.stats), 1)
	case tea.KeyUp:
		moveCursor(&m.statsSt.cursor, len(m.statsSt.stats), -1)
	}

	// Adjust scroll to keep cursor visible (must happen here, not in render,
	// because View() uses a value receiver and mutations would be discarded).
	maxVisible := m.statsMaxVisible()
	if m.statsSt.cursor < m.statsSt.scroll {
		m.statsSt.scroll = m.statsSt.cursor
	}
	if m.statsSt.cursor >= m.statsSt.scroll+maxVisible {
		m.statsSt.scroll = m.statsSt.cursor - maxVisible + 1
	}

	return m, nil
}

// statsMaxVisible returns the number of stat rows visible in the overlay,
// accounting for the extra sparkline line when the cursor row has one.
func (m model) statsMaxVisible() int {
	v := max(m.height-10, 3)
	if m.statsSt.cursor < len(m.statsSt.stats) && (m.statsSt.stats[m.statsSt.cursor].Sparkline.Bars != "" || m.statsSt.stats[m.statsSt.cursor].Sparkline.Skipped) {
		v = max(v-1, 2)
	}
	return v
}

func (m model) renderWithStatsOverlay(background string) string {
	stats := m.statsSt.stats
	if len(stats) == 0 {
		return background
	}

	modalWidth := calcModalWidth(m.width, 76)
	contentWidth := modalWidth - 6 // padding
	rowCount := len(m.lastResult.Rows)

	var b strings.Builder
	title := fmt.Sprintf("Column Statistics (%d rows)", rowCount)
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accentColor)).Render(title))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", contentWidth))
	b.WriteByte('\n')

	nameW, typeW := 6, 4

	for _, s := range stats {
		if len(s.Name) > nameW {
			nameW = len(s.Name)
		}
		if len(s.Type) > typeW {
			typeW = len(s.Type)
		}
	}
	if nameW > 20 {
		nameW = 20
	}
	if typeW > 12 {
		typeW = 12
	}

	headerFmt := fmt.Sprintf("  %%-%ds  %%-%ds  %%6s  %%8s  %%s", nameW, typeW)
	header := fmt.Sprintf(headerFmt, "Column", "Type", "NULL%", "Distinct", "Min → Max")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(mutedTextColor)).Render(header))
	b.WriteByte('\n')

	maxVisible := m.statsMaxVisible()
	rowFmt := fmt.Sprintf("%%s %%-%ds  %%-%ds  %%6s  %%8d  %%s", nameW, typeW)
	end := min(m.statsSt.scroll+maxVisible, len(stats))

	for i := m.statsSt.scroll; i < end; i++ {
		s := stats[i]
		cursor := "  "
		if i == m.statsSt.cursor {
			cursor = lipgloss.NewStyle().Foreground(lipgloss.Color(accentColor)).Render("▸ ")
		}

		nullPct := fmt.Sprintf("%.1f%%", s.NullRate*100)

		minMax := ""
		if s.Distinct > 0 {
			mn := truncate(sanitize(s.Min), 12)
			mx := truncate(sanitize(s.Max), 12)
			if mn == mx {
				minMax = mn
			} else {
				minMax = mn + " → " + mx
			}
		}

		name := truncate(sanitize(s.Name), nameW)
		typ := truncate(sanitize(s.Type), typeW)

		line := fmt.Sprintf(rowFmt, cursor, name, typ, nullPct, s.Distinct, minMax)
		if i == m.statsSt.cursor {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color(textColor)).Bold(true).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color(textColor)).Render(line)
		}
		b.WriteString(line)

		if i == m.statsSt.cursor && (s.Sparkline.Bars != "" || s.Sparkline.Skipped) {
			b.WriteByte('\n')
			// 7 = cursor(2) + space(1) + post-name spaces(2) + post-type spaces(2)
			// aligns sparkline under the NULL% column.
			indent := strings.Repeat(" ", nameW+typeW+7)
			if s.Sparkline.Bars != "" {
				spark := lipgloss.NewStyle().Foreground(lipgloss.Color(accentColor)).Render(s.Sparkline.Bars)
				lbl := lipgloss.NewStyle().Foreground(lipgloss.Color(mutedTextColor)).Render("  " + s.Sparkline.Label)
				b.WriteString(indent + spark + lbl)
			} else {
				msg := lipgloss.NewStyle().Foreground(lipgloss.Color(mutedTextColor)).Render("(sparkline skipped: >10k rows)")
				b.WriteString(indent + msg)
			}
		}

		if i < end-1 {
			b.WriteByte('\n')
		}
	}

	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(accentColor)).
		Padding(1, 2).
		Width(modalWidth).
		Background(panelBackground).
		Render(b.String())

	return overlayModal(m.width, background, modal)
}

// truncate shortens s to maxLen, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return s[:maxLen-1] + "…"
}
