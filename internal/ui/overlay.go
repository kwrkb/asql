package ui

import "github.com/charmbracelet/lipgloss"

// calcModalWidth computes the modal width from the screen width and a maximum.
// The result is clamped to at least 20.
func calcModalWidth(screenWidth, maxWidth int) int {
	w := min(screenWidth-4, maxWidth)
	if w < 20 {
		w = 20
	}
	return w
}

// overlayModal centres a rendered modal on top of a background string.
func overlayModal(screenWidth int, background string, modal string) string {
	bgH := lipgloss.Height(background)
	return lipgloss.Place(screenWidth, bgH, lipgloss.Center, lipgloss.Center, modal,
		lipgloss.WithWhitespaceBackground(appBackground))
}
