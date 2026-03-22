package ui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"

	"github.com/kwrkb/asql/internal/ai"
	"github.com/kwrkb/asql/internal/profile"
	"github.com/kwrkb/asql/internal/snippet"
)

// detailState holds state for the detail overlay (DETAIL mode).
type detailState struct {
	fieldCursor int
	scroll      int
}

// exportState holds state for the export overlay (EXPORT mode).
type exportState struct {
	cursor int
}

// aiState holds state for the AI text-to-SQL overlay (AI mode).
type aiState struct {
	enabled bool
	client  *ai.Client
	input   textinput.Model
	spinner spinner.Model
	loading bool
	err     string
}

// snippetState holds state for the saved-query overlay (SNIPPET mode).
type snippetState struct {
	items    []snippet.Snippet
	cursor   int
	naming   bool
	input    textinput.Model
	prevMode mode // mode before entering snippet naming via Ctrl+S
}

// profileState holds state for the connection-profile overlay (PROFILE mode).
type profileState struct {
	items  []profile.Profile
	cursor int
	naming bool
	input  textinput.Model
}

// histSearchState holds state for the history search overlay (SEARCH mode).
type histSearchState struct {
	input   textinput.Model
	results []int // indices into queryHistory (filtered)
	cursor  int
}

// completionState holds state for tab-completion in INSERT mode.
type completionState struct {
	active        bool
	items         []string
	cursor        int
	prefix        string
	colCache      map[string][]string
	colOrder      []string // LRU order: most recently used at end
	pendingPrefix string   // prefix when async fetch was initiated (empty = no pending)
}

// sidebarState holds state for the table-list sidebar (SIDEBAR mode).
type sidebarState struct {
	open   bool
	tables []string
	cursor int
}
