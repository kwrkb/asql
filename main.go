package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/sqly/internal/ai"
	"github.com/kwrkb/sqly/internal/config"
	dbpkg "github.com/kwrkb/sqly/internal/db"
	"github.com/kwrkb/sqly/internal/db/sqlite"
	"github.com/kwrkb/sqly/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
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

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load config: %v\n", err)
	}

	var aiClient *ai.Client
	if cfg.AIEnabled() {
		aiClient = ai.NewClient(cfg.AI.Endpoint, cfg.AI.Model, cfg.AI.APIKey)
	}

	program := tea.NewProgram(
		ui.NewModel(adapter, dbPath, aiClient),
		tea.WithAltScreen(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "sqly exited with error: %v\n", err)
		os.Exit(1)
	}
}
