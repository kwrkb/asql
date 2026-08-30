package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/kwrkb/asql/internal/profile"
	"github.com/kwrkb/asql/internal/snippet"
)

func TestFit(t *testing.T) {
	tests := []struct {
		input string
		width int
		want  string
	}{
		{"hello", 10, "hello     "},
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"hello", 1, "…"},
		{"hello", 0, ""}, // no cell to spend, not even on the ellipsis
		{"", 5, "     "},
		// A width is a cell count, not a byte or rune count: a wide character
		// costs two cells and is never cut in half.
		{"日本語", 6, "日本語"},
		{"日本語", 10, "日本語    "},
		{"日本語", 4, "日… "}, // the cell freed by dropping a wide char is padded back
		{"日本語のとても長い値です", 12, "日本語のと… "},
	}
	for _, tt := range tests {
		got := fit(tt.input, tt.width)
		if got != tt.want {
			t.Errorf("fit(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
		}
		if w := ansi.StringWidth(got); w != tt.width {
			t.Errorf("fit(%q, %d) = %q: occupies %d cells, want %d", tt.input, tt.width, got, w, tt.width)
		}
	}
}

// The old implementation sliced at a byte offset, so a multi-byte value was cut
// mid-sequence and written to the terminal as invalid UTF-8.
func TestFit_NeverBreaksUTF8(t *testing.T) {
	inputs := []string{
		"日本語のとても長い値です",
		"aあbいcうdえeお",
		"emoji 👨‍👩‍👧 family",
	}
	for _, in := range inputs {
		for w := 1; w <= 24; w++ {
			got := fit(in, w)
			if !utf8.ValidString(got) {
				t.Errorf("fit(%q, %d) = %q: not valid UTF-8", in, w, got)
			}
			if gw := ansi.StringWidth(got); gw != w {
				t.Errorf("fit(%q, %d) = %q: occupies %d cells, want %d", in, w, got, gw, w)
			}
		}
	}
}

func TestTruncateCells(t *testing.T) {
	tests := []struct {
		input string
		width int
		want  string
	}{
		{"hello", 10, "hello"}, // unlike fit, no padding
		{"hello", 5, "hello"},
		{"hello", 4, "hel…"},
		{"hello", 0, ""},
		{"hello", -3, ""},
		// A rune-counting truncation would keep all three characters here and
		// occupy six cells instead of four.
		{"日本語", 4, "日…"},
		{"日本語", 6, "日本語"},
	}
	for _, tt := range tests {
		got := truncateCells(tt.input, tt.width)
		if got != tt.want {
			t.Errorf("truncateCells(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
		}
		if w := ansi.StringWidth(got); w > max(tt.width, 0) {
			t.Errorf("truncateCells(%q, %d) = %q: occupies %d cells, over budget", tt.input, tt.width, got, w)
		}
	}
}

func TestTruncateCells_NeverBreaksUTF8(t *testing.T) {
	inputs := []string{
		"日本語のとても長い値です",
		"aあbいcうdえeお",
		"emoji 👨‍👩‍👧 family",
	}
	for _, in := range inputs {
		for w := 1; w <= 24; w++ {
			got := truncateCells(in, w)
			if !utf8.ValidString(got) {
				t.Errorf("truncateCells(%q, %d) = %q: not valid UTF-8", in, w, got)
			}
			if gw := ansi.StringWidth(got); gw > w {
				t.Errorf("truncateCells(%q, %d) = %q: occupies %d cells, want at most %d", in, w, got, gw, w)
			}
		}
	}
}

// modalBoxHeight counts the rendered rows of the bordered modal, from its top
// border to its bottom one. A preview that overruns the modal width makes
// lipgloss wrap it, which shows up here as extra rows.
func modalBoxHeight(t *testing.T, rendered string) int {
	t.Helper()
	top, bottom := -1, -1
	for i, ln := range strings.Split(rendered, "\n") {
		plain := ansi.Strip(ln)
		if strings.Contains(plain, "╭") {
			top = i
		}
		if strings.Contains(plain, "╰") {
			bottom = i
		}
	}
	if top < 0 || bottom < 0 {
		t.Fatalf("no modal border found in:\n%s", rendered)
	}
	return bottom - top + 1
}

// Every preview below is truncated against a cell budget. Counting that budget
// in runes let a preview occupy up to twice its allowance, and lipgloss wrapped
// the overrun onto a second line: the modal grew a row and every height and
// scroll calculation around it drifted. Each case renders a short preview that
// certainly fits, then long ASCII and long wide-character ones, and requires
// all three boxes to come out the same height.
func TestOverlayPreviews_LongValuesDoNotGrowTheModal(t *testing.T) {
	const short = "select 1"
	const longASCII = "select id, name from users where created_at > '2024-01-01' order by id desc limit 100"
	// Same number of runes as longASCII, twice the cells.
	longWide := strings.Repeat("あ", utf8.RuneCountInString(longASCII))

	cases := []struct {
		name   string
		render func(text string) string
	}{
		{"snippet", func(text string) string {
			m := newTestModel()
			m.width, m.height = 100, 30
			m.mode = snippetMode
			m.snippetSt.items = []snippet.Snippet{{Name: "q1", Query: text}}
			return m.renderWithSnippetOverlay("background")
		}},
		{"history search", func(text string) string {
			m := newTestModel()
			m.width, m.height = 100, 30
			m.mode = historySearchMode
			m.queryHistory = []string{text}
			m.histSearch.results = []int{0}
			return m.renderWithHistorySearchOverlay("background")
		}},
		{"profile", func(text string) string {
			m := newTestModel()
			m.width, m.height = 100, 30
			m.mode = profileMode
			m.profileSt.items = []profile.Profile{{Name: "p1", DSN: text}}
			return m.renderWithProfileOverlay("background")
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := modalBoxHeight(t, tc.render(short))
			for _, tt := range []struct {
				label string
				text  string
			}{{"long ASCII", longASCII}, {"long wide-character", longWide}} {
				if got := modalBoxHeight(t, tc.render(tt.text)); got != want {
					t.Errorf("%s preview renders a %d-row modal, want %d — the preview wrapped",
						tt.label, got, want)
				}
			}
		})
	}
}

// The completion popup sizes itself from the widest item, so the check there
// is that its label still fits the width the popup chose. A wrapped label keeps
// the popup's width, so height is what gives it away.
func TestCompletionPopup_LongLabelStaysOnOneRow(t *testing.T) {
	render := func(item string) string {
		m := newTestModel()
		m.width, m.height = 100, 30
		m.completion.active = true
		m.completion.items = []string{item}
		return m.renderCompletionPopup()
	}

	want := modalBoxHeight(t, render("id"))
	for _, tt := range []struct {
		label string
		item  string
	}{
		{"long ASCII", strings.Repeat("a", 80)},
		{"long wide-character", strings.Repeat("\u3042", 40)},
	} {
		if got := modalBoxHeight(t, render(tt.item)); got != want {
			t.Errorf("%s label renders a %d-row popup, want %d — the label wrapped",
				tt.label, got, want)
		}
	}
}
