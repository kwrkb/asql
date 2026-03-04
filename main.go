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
	"github.com/kwrkb/asql/internal/profile"
	"github.com/kwrkb/asql/internal/snippet"
	"github.com/kwrkb/asql/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

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
		fmt.Fprintf(os.Stderr, "  [%d] %s  (%s)\n", i+1, p.Name, MaskDSN(p.DSN))
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

var rePasswordInDSN = regexp.MustCompile(`(://[^:]*:)([^@]*)(@)`)

func MaskDSN(dsn string) string {
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

	displayDSN := MaskDSN(dbPath)

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
		ui.NewModel(adapter, displayDSN, dbPath, aiClient, snippets, profiles),
		tea.WithAltScreen(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "asql exited with error: %v\n", err)
		os.Exit(1)
	}
}
