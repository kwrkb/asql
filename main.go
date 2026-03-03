package main

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/ai"
	"github.com/kwrkb/asql/internal/config"
	dbpkg "github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/mysql"
	"github.com/kwrkb/asql/internal/db/postgres"
	"github.com/kwrkb/asql/internal/db/sqlite"
	"github.com/kwrkb/asql/internal/snippet"
	"github.com/kwrkb/asql/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func resolveDSN(args []string, getenv func(string) string) (string, error) {
	if len(args) > 2 {
		return "", fmt.Errorf("usage: %s [<database-path-or-dsn>]", args[0])
	}
	if len(args) == 2 {
		return args[1], nil
	}
	if dsn := getenv("ASQL_DSN"); dsn != "" {
		return dsn, nil
	}
	if dsn := getenv("DATABASE_URL"); dsn != "" {
		return dsn, nil
	}
	return "", fmt.Errorf("usage: %s <database-path-or-dsn>\n  or set ASQL_DSN / DATABASE_URL environment variable", args[0])
}

var rePasswordInDSN = regexp.MustCompile(`(://[^:]*:)([^@]*)(@)`)

func maskDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		// Best-effort: mask password in malformed URLs
		return rePasswordInDSN.ReplaceAllString(dsn, "${1}***${3}")
	}
	masked := false
	// Mask userinfo password (user:password@host)
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "***")
			masked = true
		}
	}
	// Mask query parameter password (?password=secret)
	q := u.Query()
	if q.Get("password") != "" {
		q.Set("password", "***")
		u.RawQuery = q.Encode()
		masked = true
	}
	if !masked {
		return dsn
	}
	return u.String()
}

func main() {
	dbPath, err := resolveDSN(os.Args, os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	displayDSN := maskDSN(dbPath)

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
		fmt.Fprintf(os.Stderr, "failed to open database %q: %v\n", displayDSN, err)
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

	snippets, snippetErr := snippet.Load()
	if snippetErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load snippets: %v\n", snippetErr)
	}

	program := tea.NewProgram(
		ui.NewModel(adapter, displayDSN, aiClient, snippets),
		tea.WithAltScreen(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "asql exited with error: %v\n", err)
		os.Exit(1)
	}
}
