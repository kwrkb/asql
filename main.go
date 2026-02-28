package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/ai"
	"github.com/kwrkb/asql/internal/config"
	dbpkg "github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/mysql"
	"github.com/kwrkb/asql/internal/db/postgres"
	"github.com/kwrkb/asql/internal/db/sqlite"
	"github.com/kwrkb/asql/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <database-path-or-dsn>\n", os.Args[0])
		os.Exit(1)
	}

	dbPath := os.Args[1]

	var err error
	var adapter dbpkg.DBAdapter

	switch {
	case strings.HasPrefix(dbPath, "mysql://"):
		adapter, err = mysql.Open(dbPath)
	case strings.HasPrefix(dbPath, "postgres://"), strings.HasPrefix(dbPath, "postgresql://"):
		adapter, err = postgres.Open(dbPath)
	default:
		adapter, err = sqlite.Open(dbPath)
	}
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
		fmt.Fprintf(os.Stderr, "asql exited with error: %v\n", err)
		os.Exit(1)
	}
}
