package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type completionContext int

const (
	contextUnknown completionContext = iota
	contextTable
	contextColumn
)

// wordAtCursor extracts the prefix being typed at the cursor position.
// Returns the prefix and its start position within the full text.
func wordAtCursor(text string, cursorRow int, charOffset int) (prefix string, startPos int) {
	lines := strings.Split(text, "\n")
	if cursorRow < 0 || cursorRow >= len(lines) {
		return "", 0
	}
	line := lines[cursorRow]
	// charOffset may exceed line length if cursor is at end
	col := charOffset
	if col > len(line) {
		col = len(line)
	}

	// Calculate absolute position in text
	absPos := 0
	for i := 0; i < cursorRow; i++ {
		absPos += len(lines[i]) + 1 // +1 for newline
	}
	absPos += col

	// Scan backward for identifier characters or dot-prefix
	start := col
	for start > 0 {
		r := rune(line[start-1])
		if isIdentRune(r) || r == '.' {
			start--
		} else {
			break
		}
	}

	prefix = line[start:col]
	startPos = absPos - len(prefix)
	return prefix, startPos
}

func isIdentRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// detectContext determines whether cursor is in a table-name or column-name context
// by scanning the text before startPos for SQL keywords.
func detectContext(text string, startPos int) completionContext {
	before := text[:startPos]
	// Find the last significant keyword token
	kw := lastKeyword(before)
	switch kw {
	case "from", "join", "into", "update", "table":
		return contextTable
	case "select", "where", "and", "or", "set", "by", "on", "having":
		return contextColumn
	default:
		return contextUnknown
	}
}

// lastKeyword returns the last SQL keyword found before the cursor position,
// skipping whitespace, commas, and identifiers.
func lastKeyword(text string) string {
	sqlKeywords := map[string]bool{
		"from": true, "join": true, "into": true, "update": true, "table": true,
		"select": true, "where": true, "and": true, "or": true, "set": true,
		"by": true, "on": true, "having": true, "inner": true, "left": true,
		"right": true, "outer": true, "cross": true, "order": true, "group": true,
	}

	i := len(text) - 1
	// Skip trailing whitespace
	for i >= 0 && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r' || text[i] == ',') {
		i--
	}

	// Walk backward through tokens
	for i >= 0 {
		// Extract word ending at i
		end := i + 1
		for i >= 0 && isIdentRune(rune(text[i])) {
			i--
		}
		start := i + 1
		if start < end {
			word := strings.ToLower(text[start:end])
			if sqlKeywords[word] {
				// "ORDER BY" / "GROUP BY" → return "by"
				// "LEFT JOIN" → return "join"
				return word
			}
		}
		// Skip whitespace/commas/dots between tokens
		for i >= 0 && (text[i] == ' ' || text[i] == '\t' || text[i] == '\n' || text[i] == '\r' || text[i] == ',' || text[i] == '.') {
			i--
		}
		// If we hit a non-identifier, non-space character, stop
		if i >= 0 && !isIdentRune(rune(text[i])) {
			break
		}
	}
	return ""
}

// detectTableFromContext tries to find the relevant table name for column completion.
// Handles "tablename." prefix and "FROM tablename" patterns.
func detectTableFromContext(text string, prefix string, tables []string) string {
	// Check for "tablename." dot-prefix pattern
	if dotIdx := strings.LastIndex(prefix, "."); dotIdx >= 0 {
		tablePart := prefix[:dotIdx]
		for _, t := range tables {
			if strings.EqualFold(t, tablePart) {
				return t
			}
		}
		return ""
	}

	// Look for FROM/JOIN clause table name
	lower := strings.ToLower(text)
	tableSet := make(map[string]string, len(tables)) // lowercase -> original
	for _, t := range tables {
		tableSet[strings.ToLower(t)] = t
	}

	// Find all FROM/JOIN occurrences and extract table names; return the last match
	var lastMatch string
	for _, kw := range []string{"from ", "join "} {
		idx := 0
		for {
			pos := strings.Index(lower[idx:], kw)
			if pos < 0 {
				break
			}
			pos += idx + len(kw)
			// Skip whitespace
			for pos < len(lower) && (lower[pos] == ' ' || lower[pos] == '\t' || lower[pos] == '\n') {
				pos++
			}
			// Extract table name
			end := pos
			for end < len(lower) && isIdentRune(rune(lower[end])) {
				end++
			}
			if end > pos {
				candidate := lower[pos:end]
				if orig, ok := tableSet[candidate]; ok {
					lastMatch = orig
				}
			}
			idx = end
		}
	}
	return lastMatch
}

// filterByPrefix returns items that start with the given prefix (case-insensitive).
func filterByPrefix(items []string, prefix string) []string {
	if prefix == "" {
		return items
	}
	lower := strings.ToLower(prefix)
	var result []string
	for _, item := range items {
		if strings.HasPrefix(strings.ToLower(item), lower) {
			result = append(result, item)
		}
	}
	return result
}

// triggerCompletion collects completion candidates based on cursor context.
func (m *model) triggerCompletion() {
	text := m.textarea.Value()
	row := m.textarea.Line()
	charOffset := m.textarea.LineInfo().CharOffset

	prefix, startPos := wordAtCursor(text, row, charOffset)

	// Strip table prefix for column filtering (e.g., "users.na" → filter "na")
	filterPrefix := prefix
	if dotIdx := strings.LastIndex(prefix, "."); dotIdx >= 0 {
		filterPrefix = prefix[dotIdx+1:]
	}

	ctx := detectContext(text, startPos)

	var candidates []string
	switch ctx {
	case contextTable:
		candidates = filterByPrefix(m.sidebar.tables, filterPrefix)
	case contextColumn:
		tableName := detectTableFromContext(text, prefix, m.sidebar.tables)
		if tableName != "" {
			cols := m.getOrFetchColumns(tableName)
			candidates = filterByPrefix(cols, filterPrefix)
		} else {
			// Gather columns from all known tables
			candidates = m.allColumns(filterPrefix)
		}
	default:
		// Unknown context: offer both tables and columns
		candidates = filterByPrefix(m.sidebar.tables, filterPrefix)
		colCandidates := m.allColumns(filterPrefix)
		candidates = append(candidates, colCandidates...)
		candidates = dedup(candidates)
	}

	if len(candidates) == 0 {
		return
	}

	if len(candidates) == 1 {
		// Single candidate: insert immediately
		m.insertCompletion(candidates[0], prefix)
		return
	}

	m.completion.active = true
	m.completion.items = candidates
	m.completion.cursor = 0
	m.completion.prefix = prefix
}

// acceptCompletion inserts the currently selected completion candidate.
func (m *model) acceptCompletion() {
	if !m.completion.active || len(m.completion.items) == 0 {
		return
	}
	selected := m.completion.items[m.completion.cursor]
	m.insertCompletion(selected, m.completion.prefix)
	m.closeCompletion()
}

// insertCompletion replaces the prefix with the selected item.
func (m *model) insertCompletion(selected string, prefix string) {
	// For "tablename.col" prefix, only replace after the dot
	suffix := selected
	if dotIdx := strings.LastIndex(prefix, "."); dotIdx >= 0 {
		afterDot := prefix[dotIdx+1:]
		if len(afterDot) <= len(suffix) {
			suffix = suffix[len(afterDot):]
		}
	} else if len(prefix) <= len(suffix) {
		suffix = suffix[len(prefix):]
	}
	m.textarea.InsertString(suffix)
}

// closeCompletion hides the completion popup.
func (m *model) closeCompletion() {
	m.completion.active = false
	m.completion.items = nil
	m.completion.cursor = 0
	m.completion.prefix = ""
}

// getOrFetchColumns returns cached columns or fetches them synchronously.
func (m *model) getOrFetchColumns(tableName string) []string {
	if m.completion.colCache == nil {
		m.completion.colCache = make(map[string][]string)
	}
	if cols, ok := m.completion.colCache[tableName]; ok {
		return cols
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	cols, err := m.activeDB().Columns(ctx, tableName)
	if err != nil {
		return nil
	}
	// Cap cache size to prevent unbounded growth
	const maxColCacheSize = 64
	if len(m.completion.colCache) >= maxColCacheSize {
		// Evict one random entry to make space
		for k := range m.completion.colCache {
			delete(m.completion.colCache, k)
			break
		}
	}
	m.completion.colCache[tableName] = cols
	return cols
}

// allColumns gathers columns from all known tables, filtered by prefix.
func (m *model) allColumns(prefix string) []string {
	var all []string
	for _, t := range m.sidebar.tables {
		cols := m.getOrFetchColumns(t)
		all = append(all, filterByPrefix(cols, prefix)...)
	}
	return dedup(all)
}

func dedup(items []string) []string {
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		lower := strings.ToLower(item)
		if !seen[lower] {
			seen[lower] = true
			result = append(result, item)
		}
	}
	return result
}

const maxCompletionVisible = 8

// renderCompletionPopup draws the completion popup below the editor area.
func (m model) renderCompletionPopup() string {
	if !m.completion.active || len(m.completion.items) == 0 {
		return ""
	}

	popupWidth := 30
	for _, item := range m.completion.items {
		w := lipgloss.Width(item) + 4
		if w > popupWidth {
			popupWidth = w
		}
	}
	if popupWidth > 50 {
		popupWidth = 50
	}

	itemStyle := lipgloss.NewStyle().
		Foreground(textColor).
		Background(panelBackground).
		Width(popupWidth - 4).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(panelBackground).
		Background(accentColor).
		Bold(true).
		Width(popupWidth - 4).
		Padding(0, 1)

	// Calculate scroll offset
	scrollOffset := 0
	if m.completion.cursor >= maxCompletionVisible {
		scrollOffset = m.completion.cursor - maxCompletionVisible + 1
	}
	end := min(scrollOffset+maxCompletionVisible, len(m.completion.items))

	var lines []string
	for i := scrollOffset; i < end; i++ {
		label := sanitize(m.completion.items[i])
		runes := []rune(label)
		maxLen := popupWidth - 6
		if maxLen > 0 && len(runes) > maxLen {
			label = string(runes[:maxLen]) + "…"
		}
		if i == m.completion.cursor {
			lines = append(lines, selectedStyle.Render(label))
		} else {
			lines = append(lines, itemStyle.Render(label))
		}
	}

	if len(m.completion.items) > maxCompletionVisible {
		info := lipgloss.NewStyle().
			Foreground(mutedTextColor).
			Background(panelBackground).
			Width(popupWidth - 4).
			Padding(0, 1).
			Render(fmt.Sprintf("%d/%d", m.completion.cursor+1, len(m.completion.items)))
		lines = append(lines, info)
	}

	content := strings.Join(lines, "\n")
	popup := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Background(panelBackground).
		Padding(0).
		Render(content)

	return popup
}
