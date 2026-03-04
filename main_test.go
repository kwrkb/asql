package main

import (
	"testing"

	"github.com/kwrkb/asql/internal/profile"
)

func TestResolveDSN(t *testing.T) {
	noenv := func(string) string { return "" }
	noprofiles := []profile.Profile(nil)

	tests := []struct {
		name     string
		args     []string
		getenv   func(string) string
		profiles []profile.Profile
		want     string
		wantErr  bool
	}{
		{
			name: "CLI argument",
			args: []string{"asql", "test.db"},
			getenv: noenv,
			profiles: noprofiles,
			want:   "test.db",
		},
		{
			name: "CLI argument takes priority over env",
			args: []string{"asql", "cli.db"},
			getenv: func(key string) string {
				if key == "ASQL_DSN" {
					return "env.db"
				}
				return ""
			},
			profiles: noprofiles,
			want: "cli.db",
		},
		{
			name: "ASQL_DSN env var",
			args: []string{"asql"},
			getenv: func(key string) string {
				if key == "ASQL_DSN" {
					return "mysql://user:pass@host/db"
				}
				return ""
			},
			profiles: noprofiles,
			want: "mysql://user:pass@host/db",
		},
		{
			name: "DATABASE_URL env var",
			args: []string{"asql"},
			getenv: func(key string) string {
				if key == "DATABASE_URL" {
					return "postgres://user:pass@host/db"
				}
				return ""
			},
			profiles: noprofiles,
			want: "postgres://user:pass@host/db",
		},
		{
			name: "ASQL_DSN takes priority over DATABASE_URL",
			args: []string{"asql"},
			getenv: func(key string) string {
				switch key {
				case "ASQL_DSN":
					return "asql.db"
				case "DATABASE_URL":
					return "database.db"
				}
				return ""
			},
			profiles: noprofiles,
			want: "asql.db",
		},
		{
			name:     "no argument and no env",
			args:     []string{"asql"},
			getenv:   noenv,
			profiles: noprofiles,
			wantErr:  true,
		},
		{
			name:     "too many arguments",
			args:     []string{"asql", "a", "b"},
			getenv:   noenv,
			profiles: noprofiles,
			wantErr:  true,
		},
		{
			name: "@profile resolves to DSN",
			args: []string{"asql", "@mydb"},
			getenv: noenv,
			profiles: []profile.Profile{
				{Name: "mydb", DSN: "postgres://user:pass@host/db"},
			},
			want: "postgres://user:pass@host/db",
		},
		{
			name:     "@profile not found",
			args:     []string{"asql", "@unknown"},
			getenv:   noenv,
			profiles: noprofiles,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDSN(tt.args, tt.getenv, tt.profiles)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveDSN() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("resolveDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMaskDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "SQLite path unchanged",
			dsn:  "test.db",
			want: "test.db",
		},
		{
			name: "relative path unchanged",
			dsn:  "./data/my.db",
			want: "./data/my.db",
		},
		{
			name: "MySQL password masked",
			dsn:  "mysql://admin:secret@localhost:3306/mydb",
			want: "mysql://admin:%2A%2A%2A@localhost:3306/mydb",
		},
		{
			name: "PostgreSQL password masked",
			dsn:  "postgres://user:p@ssw0rd@host:5432/db",
			want: "postgres://user:%2A%2A%2A@host:5432/db",
		},
		{
			name: "no password in URL",
			dsn:  "postgres://user@host:5432/db",
			want: "postgres://user@host:5432/db",
		},
		{
			name: "empty password masked",
			dsn:  "mysql://user:@localhost/db",
			want: "mysql://user:%2A%2A%2A@localhost/db",
		},
		{
			name: "query parameter password masked",
			dsn:  "postgres://host:5432/db?user=alice&password=secret",
			want: "postgres://host:5432/db?password=%2A%2A%2A&user=alice",
		},
		{
			name: "both userinfo and query password masked",
			dsn:  "postgres://user:pass@host/db?password=secret",
			want: "postgres://user:%2A%2A%2A@host/db?password=%2A%2A%2A",
		},
		{
			name: "malformed URL with password best-effort masked",
			dsn:  "postgres://user:secret@host:5432/db%zz",
			want: "postgres://user:***@host:5432/db%zz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskDSN(tt.dsn)
			if got != tt.want {
				t.Errorf("MaskDSN(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}

func TestParseSaveProfile(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantName string
		wantArgs []string
	}{
		{
			name:     "no flag",
			args:     []string{"asql", "test.db"},
			wantName: "",
			wantArgs: []string{"asql", "test.db"},
		},
		{
			name:     "with flag",
			args:     []string{"asql", "--save-profile", "mydb", "test.db"},
			wantName: "mydb",
			wantArgs: []string{"asql", "test.db"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, args := parseSaveProfile(tt.args)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("args len = %d, want %d", len(args), len(tt.wantArgs))
			}
			for i := range args {
				if args[i] != tt.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q", i, args[i], tt.wantArgs[i])
				}
			}
		})
	}
}
