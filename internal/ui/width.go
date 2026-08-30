package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Widths in this package are terminal cell counts, never byte or rune counts.
// lipgloss lays out and pads by cells, so anything measured in another unit
// disagrees with the renderer: a wide character costs two cells, and a string
// cut to N runes can occupy up to 2N of them. The overflow does not merely
// look wrong — lipgloss wraps the too-long line, which grows the modal by a
// row and desynchronises every height and scroll calculation around it.
//
// fmt's "%-Ns" cannot stand in for these helpers: it pads by rune count.

// fit renders s in exactly w cells: truncated with "…" when it is too wide,
// padded with spaces when it is too narrow.
func fit(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(w-ansi.StringWidth(s), 0))
}

// truncateCells shortens s to at most w cells, marking a cut with "…". Unlike
// fit it does not pad, so it suits previews that are followed by nothing.
// A non-positive w yields "": there is no room even for the ellipsis.
func truncateCells(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}
