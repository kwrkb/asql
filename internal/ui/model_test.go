package ui

import "testing"

func TestColumnWidth(t *testing.T) {
	tests := []struct {
		name  string
		title string
		rows  [][]string
		idx   int
		want  int
	}{
		{
			name:  "minimum width when title and values are short",
			title: "id",
			rows:  [][]string{{"1"}, {"2"}},
			idx:   0,
			want:  12,
		},
		{
			name:  "title determines width",
			title: "user_name_column",
			rows:  [][]string{{"alice"}},
			idx:   0,
			want:  18, // len("user_name_column")=16, 16+2=18
		},
		{
			name:  "row value determines width",
			title: "val",
			rows:  [][]string{{"a_medium_length_str"}},
			idx:   0,
			want:  21, // len("a_medium_length_str")=19, 19+2=21
		},
		{
			name:  "capped at 32 when width+2 would exceed",
			title: "abcdefghijklmnopqrstuvwxyzabcde", // 31 chars
			rows:  nil,
			idx:   0,
			want:  32, // 31+2=33 → capped at 32
		},
		{
			name:  "exactly 32 when width is 30",
			title: "abcdefghijklmnopqrstuvwxyzabcd", // 30 chars
			rows:  nil,
			idx:   0,
			want:  32, // 30+2=32
		},
		{
			name:  "out of bounds idx skipped safely",
			title: "col",
			rows:  [][]string{{"only one col"}},
			idx:   5,
			want:  12, // title "col" is short, minimum applied
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := columnWidth(tt.title, tt.rows, tt.idx)
			if got != tt.want {
				t.Errorf("columnWidth(%q, rows, %d) = %d, want %d", tt.title, tt.idx, got, tt.want)
			}
		})
	}
}
