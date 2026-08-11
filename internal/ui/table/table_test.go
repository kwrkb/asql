package table

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
)

// These tests cover the one behaviour this vendored copy changes: truncation
// must measure display cells, not escape-sequence bytes. Upstream's own suite
// is not vendored — see the package comment.
//
// lipgloss emits no escape sequences when it cannot detect a colour-capable
// terminal, which a `go test` run never has. Without pinning the profile these
// tests would style nothing and pass against the unpatched code, so pin it.
func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

var styled = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))

func TestRenderRow_StyledCellSurvivesANarrowColumn(t *testing.T) {
	// 3 visible cells, but ~25 bytes once the escape sequence is counted.
	value := styled.Render("abc")
	if runewidth.StringWidth(value) <= 12 {
		t.Fatalf("test premise broken: runewidth width is %d, expected it to exceed the column width",
			runewidth.StringWidth(value))
	}

	m := &Model{
		rows:   []Row{{value}},
		cols:   []Column{{Title: "col", Width: 12}},
		styles: Styles{Cell: lipgloss.NewStyle()},
	}

	got := ansi.Strip(m.renderRow(0))
	if !strings.Contains(got, "abc") {
		t.Errorf("rendered row = %q, want it to contain the cell value", got)
	}
	if strings.Contains(got, "…") {
		t.Errorf("rendered row = %q, want no truncation of a value that fits in 12 cells", got)
	}
}

func TestHeadersView_StyledTitleSurvivesANarrowColumn(t *testing.T) {
	// The shape asql builds: a plain column name plus a dim type annotation.
	title := "id " + styled.Render("int")

	m := &Model{
		cols:   []Column{{Title: title, Width: 12}},
		styles: Styles{Header: lipgloss.NewStyle()},
	}

	got := ansi.Strip(m.headersView())
	if !strings.Contains(got, "id int") {
		t.Errorf("header = %q, want it to contain the type annotation", got)
	}
}

func TestRenderRow_StillTruncatesByDisplayWidth(t *testing.T) {
	// The fix must not stop truncation happening — only stop it happening at
	// the wrong place. 20 visible cells in a 12-cell column must still be cut.
	m := &Model{
		rows:   []Row{{styled.Render(strings.Repeat("x", 20))}},
		cols:   []Column{{Title: "col", Width: 12}},
		styles: Styles{Cell: lipgloss.NewStyle()},
	}

	got := ansi.Strip(m.renderRow(0))
	if !strings.Contains(got, "…") {
		t.Errorf("rendered row = %q, want an ellipsis for content wider than the column", got)
	}
	if strings.Contains(got, strings.Repeat("x", 20)) {
		t.Errorf("rendered row = %q, want the content cut to the column width", got)
	}
	if w := lipgloss.Width(got); w != 12 {
		t.Errorf("rendered width = %d, want 12", w)
	}
}

func TestRenderRow_PlainCellUnchanged(t *testing.T) {
	// Unstyled cells must behave exactly as they did before the patch.
	m := &Model{
		rows:   []Row{{"plain"}},
		cols:   []Column{{Title: "col", Width: 12}},
		styles: Styles{Cell: lipgloss.NewStyle()},
	}

	got := ansi.Strip(m.renderRow(0))
	if !strings.Contains(got, "plain") {
		t.Errorf("rendered row = %q, want the plain value intact", got)
	}
}
