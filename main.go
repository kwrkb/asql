package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	dbpkg "github.com/kwrkb/sqly/internal/db"
	"github.com/kwrkb/sqly/internal/db/sqlite"
	"github.com/kwrkb/sqly/internal/ui"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <sqlite-file>\n", os.Args[0])
		os.Exit(1)
	}

	dbPath := os.Args[1]

	var err error
	var adapter dbpkg.DBAdapter
	adapter, err = sqlite.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database %q: %v\n", dbPath, err)
		os.Exit(1)
	}
	defer adapter.Close()

	program := tea.NewProgram(
		ui.NewModel(adapter, dbPath),
		tea.WithAltScreen(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sqly exited with error: %v\n", err)
		os.Exit(1)
	}
}
