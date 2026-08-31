package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// calcModalWidth computes the modal width from the screen width and a maximum.
// The result is clamped to at least 20 when the screen is wide enough.
func calcModalWidth(screenWidth, maxWidth int) int {
	w := min(screenWidth-4, maxWidth)
	minWidth := min(20, max(screenWidth, 1))
	if w < minWidth {
		w = minWidth
	}
	return w
}

// overlayModal centres a rendered modal on top of a background string.
func overlayModal(screenWidth int, background string, modal string) string {
	bgH := lipgloss.Height(background)
	return lipgloss.Place(screenWidth, bgH, lipgloss.Center, lipgloss.Center, modal,
		lipgloss.WithWhitespaceBackground(appBackground))
}

// overlayTopLines draws overlay over the first lines of background, padding
// each overlaid line to width so it fully covers the background row. Lines are
// replaced whole — splicing an overlay into the middle of a styled line would
// need ANSI-aware surgery — and overlay lines beyond the background are
// dropped, so the composite never grows taller than the background.
func overlayTopLines(background, overlay string, width int) string {
	bgLines := strings.Split(background, "\n")
	ovLines := strings.Split(overlay, "\n")
	pad := lipgloss.NewStyle().Width(max(width, 0)).Background(appBackground)
	for i := 0; i < len(ovLines) && i < len(bgLines); i++ {
		bgLines[i] = pad.Render(ovLines[i])
	}
	return strings.Join(bgLines, "\n")
}
