package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kwrkb/asql/internal/ai"
	"github.com/kwrkb/asql/internal/config"
	dbpkg "github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/opener"
	"github.com/kwrkb/asql/internal/profile"
	"github.com/kwrkb/asql/internal/snippet"
	"github.com/kwrkb/asql/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// connectionHint returns a user-friendly hint for common DB connection errors.
func connectionHint(dsn string, err error) string {
	msg := err.Error()
	switch {
	case strings.HasPrefix(dsn, "mysql://"), strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		switch {
		case strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host") || strings.Contains(msg, "dial"):
			return "hint: connection refused — check host/port and confirm the server is running"
		case strings.Contains(msg, "authentication") || strings.Contains(msg, "password") || strings.Contains(msg, "Access denied"):
			return "hint: authentication failed — check username and password"
		case strings.Contains(msg, "unknown database") || strings.Contains(msg, "does not exist"):
			return "hint: database not found — check the database name in the DSN"
		}
		if strings.HasPrefix(dsn, "mysql://") {
			return "hint: DSN format: mysql://user:password@host:3306/dbname"
		}
		return "hint: DSN format: postgres://user:password@host:5432/dbname"
	default:
		// SQLite
		if strings.Contains(msg, "no such file") || strings.Contains(msg, "unable to open") {
			return "hint: file not found — check the path to your SQLite database"
		}
		return "hint: DSN formats: <path>.db  |  mysql://user:pass@host:3306/db  |  postgres://user:pass@host:5432/db"
	}
}

// parseSaveProfile extracts --save-profile <name> from args and returns
// the profile name and the remaining args.
func parseSaveProfile(args []string) (string, []string, error) {
	var remaining []string
	var profileName string
	for i := 0; i < len(args); i++ {
		if args[i] == "--save-profile" {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--save-profile requires a name argument")
			}
			profileName = args[i+1]
			i++ // skip value
		} else {
			remaining = append(remaining, args[i])
		}
	}
	return profileName, remaining, nil
}

func resolveDSN(args []string, getenv func(string) string, profiles []profile.Profile) (string, error) {
	if len(args) > 2 {
		return "", fmt.Errorf("usage: %s [<database-path-or-dsn>]", args[0])
	}
	if len(args) == 2 {
		arg := args[1]
		// @profile-name resolution
		if strings.HasPrefix(arg, "@") {
			name := arg[1:]
			p := profile.Find(profiles, name)
			if p == nil {
				return "", fmt.Errorf("profile %q not found", name)
			}
			return p.DSN, nil
		}
		return arg, nil
	}
	if dsn := getenv("ASQL_DSN"); dsn != "" {
		return dsn, nil
	}
	if dsn := getenv("DATABASE_URL"); dsn != "" {
		return dsn, nil
	}
	// Interactive profile selection if profiles exist
	if len(profiles) > 0 {
		return selectProfile(profiles)
	}
	return "", fmt.Errorf("usage: %s <database-path-or-dsn>\n  or set ASQL_DSN / DATABASE_URL environment variable\n  or use @profile-name to connect via saved profile", args[0])
}

func selectProfile(profiles []profile.Profile) (string, error) {
	fmt.Fprintln(os.Stderr, "Select a profile:")
	for i, p := range profiles {
		fmt.Fprintf(os.Stderr, "  [%d] %s  (%s)\n", i+1, p.Name, dbpkg.MaskDSN(p.DSN))
	}
	fmt.Fprint(os.Stderr, "Enter number: ")
	var choice int
	if _, err := fmt.Fscan(os.Stdin, &choice); err != nil {
		return "", fmt.Errorf("invalid selection: %w", err)
	}
	if choice < 1 || choice > len(profiles) {
		return "", fmt.Errorf("selection out of range: %d", choice)
	}
	return profiles[choice-1].DSN, nil
}

const helpText = `asql — lightweight TUI SQL client for data observation

Usage:
  asql <database-path-or-dsn>
  asql @<profile-name>
  asql --save-profile <name> <dsn>
  asql [--help | --version]

Arguments:
  <database-path-or-dsn>    SQLite file path or MySQL/PostgreSQL DSN
  @<profile-name>           Connect using a saved profile

Options:
  --save-profile <name>     Save the DSN as a named profile and connect
  --help, -h                Show this help message
  --version, -v             Show version information

Environment:
  ASQL_DSN                  Default DSN (used when no argument is given)
  DATABASE_URL              Fallback DSN

If no argument or environment variable is set, asql will prompt you to
select from saved profiles (~/.config/asql/profiles.yaml).

Examples:
  asql chinook.db
  asql "mysql://root:pass@localhost:3306/mydb"
  asql "postgres://user:pass@localhost:5432/mydb"
  asql --save-profile prod "postgres://user:pass@db.example.com:5432/app"
  asql @prod`

func main() {
	// Handle --help/-h and --version/-v before anything else
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--help", "-h":
			fmt.Println(helpText)
			return
		case "--version", "-v":
			fmt.Printf("asql %s (commit: %s, built: %s)\n", version, commit, date)
			return
		}
	}

	saveProfileName, args, parseErr := parseSaveProfile(os.Args)
	if parseErr != nil {
		fmt.Fprintln(os.Stderr, parseErr)
		os.Exit(1)
	}

	profiles, profileErr := profile.Load()
	if profileErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load profiles: %v\n", profileErr)
	}

	dbPath, err := resolveDSN(args, os.Getenv, profiles)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// --save-profile: save current DSN and continue
	if saveProfileName != "" {
		newProfiles := profile.Upsert(profiles, profile.Profile{Name: saveProfileName, DSN: dbPath})
		if err := profile.Save(newProfiles); err != nil {
			fmt.Fprintf(os.Stderr, "failed to save profile: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Profile %q saved.\n", saveProfileName)
		// Reload profiles for TUI
		profiles = newProfiles
	}

	displayDSN := dbpkg.MaskDSN(dbPath)

	adapter, err := opener.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open database %q: %v\n", displayDSN, err)
		fmt.Fprintln(os.Stderr, connectionHint(dbPath, err))
		os.Exit(1)
	}
	// Determine connection name from profile or DSN
	connName := dbpkg.DisplayName(dbPath)
	for _, p := range profiles {
		if p.DSN == dbPath {
			connName = p.Name
			break
		}
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load config: %v\n", err)
	}
	for _, w := range cfg.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}

	var aiClient *ai.Client
	if cfg.AIEnabled() {
		aiClient = ai.NewClient(cfg.AI.Endpoint, cfg.AI.Model, cfg.AI.APIKey)
	}

	snippets, snippetErr := snippet.Load()
	if snippetErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load snippets: %v\n", snippetErr)
	}

	m := ui.NewModel(adapter, displayDSN, dbPath, connName, aiClient, snippets, profiles)
	defer m.CloseAll()

	program := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "asql exited with error: %v\n", err)
		os.Exit(1)
	}
}
