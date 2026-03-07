package ui

// moveCursor shifts *cursor by direction (+1 or -1) within [0, length).
// If the new position is out of bounds, the cursor stays unchanged.
func moveCursor(cursor *int, length int, direction int) {
	n := *cursor + direction
	if n >= 0 && n < length {
		*cursor = n
	}
}
