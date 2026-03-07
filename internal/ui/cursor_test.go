package ui

import "testing"

func TestMoveCursor(t *testing.T) {
	tests := []struct {
		name      string
		initial   int
		length    int
		direction int
		want      int
	}{
		{"move down", 0, 5, 1, 1},
		{"move up", 2, 5, -1, 1},
		{"at top boundary", 0, 5, -1, 0},
		{"at bottom boundary", 4, 5, 1, 4},
		{"empty list down", 0, 0, 1, 0},
		{"empty list up", 0, 0, -1, 0},
		{"single item down", 0, 1, 1, 0},
		{"single item up", 0, 1, -1, 0},
		{"middle of list", 2, 5, 1, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursor := tt.initial
			moveCursor(&cursor, tt.length, tt.direction)
			if cursor != tt.want {
				t.Errorf("moveCursor(%d, %d, %d) = %d, want %d",
					tt.initial, tt.length, tt.direction, cursor, tt.want)
			}
		})
	}
}
