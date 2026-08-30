package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/kwrkb/asql/internal/db"
)

func computeColumnStats(result db.QueryResult) []columnStat {
	stats := make([]columnStat, len(result.Columns))
	rowCount := len(result.Rows)
	// When the result carries kinds, NULL is known exactly. Otherwise fall back
	// to matching the display string, which also counts a value that is
	// literally the text "NULL".
	hasKinds := result.HasKinds()

	for i, col := range result.Columns {
		s := columnStat{Name: col}
		if i < len(result.ColumnTypes) {
			s.Type = result.ColumnTypes[i]
		}

		distinct := make(map[string]struct{})
		firstNonNull := true

		for rowIdx, row := range result.Rows {
			if i >= len(row) {
				continue
			}
			val := row[i]
			isNull := val == db.NullSentinel
			if hasKinds {
				isNull = result.KindAt(rowIdx, i) == db.KindNull
			}
			if isNull {
				s.NullCnt++
				continue
			}
			distinct[val] = struct{}{}
			if firstNonNull {
				s.Min = val
				s.Max = val
				firstNonNull = false
			} else {
				// compareValues, not smartCompare: NULLs are already skipped
				// above, so anything reaching here that reads "NULL" is text
				// and must be ordered as text.
				if compareValues(val, s.Min) < 0 {
					s.Min = val
				}
				if compareValues(val, s.Max) > 0 {
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
		} else if detectNumericColumn(s.Type) || looksLikeNumeric(result.Rows, i) {
			s.Histogram = computeHistogram(result.Rows, i)
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
// accounting for the extra line when the cursor row has a sparkline or histogram.
func (m model) statsMaxVisible() int {
	// Fixed overhead: border(2), padding(2), title(1), separator(1), header(1) = 7 lines.
	available := max(m.height-7, 1)
	v := available
	if m.statsSt.cursor < len(m.statsSt.stats) {
		s := m.statsSt.stats[m.statsSt.cursor]
		hasExtra := s.Sparkline.Bars != "" || s.Sparkline.Skipped ||
			s.Histogram.Bars != "" || s.Histogram.Skipped
		if hasExtra {
			v = max(v-1, 1)
		}
	}
	return v
}

func (m model) renderWithStatsOverlay(background string) string {
	if m.statsSt.loading {
		msg := lipgloss.NewStyle().
			Foreground(lipgloss.Color(mutedTextColor)).
			Render("Computing stats...")
		modal := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(accentColor)).
			Padding(1, 2).
			Width(calcModalWidth(m.width, 40)).
			Background(panelBackground).
			Render(msg)
		return overlayModal(m.width, background, modal)
	}

	stats := m.statsSt.stats
	if len(stats) == 0 {
		return background
	}

	modalWidth := calcModalWidth(m.width, 76)
	// max: calcModalWidth floors at the real screen width, so a terminal
	// narrower than this overhead would otherwise give a negative width.
	contentWidth := max(modalWidth-6, 1) // padding
	rowCount := len(m.lastResult.Rows)

	var b strings.Builder
	title := fmt.Sprintf("Column Statistics (%d rows)", rowCount)
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accentColor)).Render(title))
	b.WriteByte('\n')
	b.WriteString(strings.Repeat("─", contentWidth))
	b.WriteByte('\n')

	nameW, typeW := 6, 4

	for _, s := range stats {
		if w := ansi.StringWidth(sanitize(s.Name)); w > nameW {
			nameW = w
		}
		if w := ansi.StringWidth(sanitize(s.Type)); w > typeW {
			typeW = w
		}
	}
	if nameW > 20 {
		nameW = 20
	}
	if typeW > 12 {
		typeW = 12
	}

	header := fmt.Sprintf("  %s  %s  %6s  %8s  %s",
		fit("Column", nameW), fit("Type", typeW), "NULL%", "Distinct", "Min → Max")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(mutedTextColor)).Render(header))
	b.WriteByte('\n')

	maxVisible := m.statsMaxVisible()
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
			mn := ansi.Truncate(sanitize(s.Min), 12, "…")
			mx := ansi.Truncate(sanitize(s.Max), 12, "…")
			if mn == mx {
				minMax = mn
			} else {
				minMax = mn + " → " + mx
			}
		}

		line := fmt.Sprintf("%s %s  %s  %6s  %8d  %s",
			cursor, fit(sanitize(s.Name), nameW), fit(sanitize(s.Type), typeW),
			nullPct, s.Distinct, minMax)
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

		if i == m.statsSt.cursor && (s.Histogram.Bars != "" || s.Histogram.Skipped) {
			b.WriteByte('\n')
			// same indent as sparkline: aligns under the NULL% column.
			indent := strings.Repeat(" ", nameW+typeW+7)
			if s.Histogram.Bars != "" {
				hist := lipgloss.NewStyle().Foreground(lipgloss.Color(accentColor)).Render(s.Histogram.Bars)
				lbl := lipgloss.NewStyle().Foreground(lipgloss.Color(mutedTextColor)).Render("  " + s.Histogram.Label)
				b.WriteString(indent + hist + lbl)
			} else {
				msg := lipgloss.NewStyle().Foreground(lipgloss.Color(mutedTextColor)).Render("(histogram skipped: >10k rows)")
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

// fit renders s in exactly w terminal cells: truncated with "…" when it is too
// wide, padded with spaces when it is too narrow. Width is counted in cells
// rather than bytes or runes, so a wide character is never cut in half and a
// column of them still lines up. fmt's "%-Ns" cannot stand in here: it pads by
// rune count, which is two cells short per wide character.
func fit(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(w-ansi.StringWidth(s), 0))
}
