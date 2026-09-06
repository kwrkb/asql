package db

import "testing"

func TestDetectType(t *testing.T) {
	tests := []struct {
		dsn  string
		want string
	}{
		{"mysql://user:pass@host/db", "mysql"},
		{"postgres://user:pass@host/db", "postgres"},
		{"postgresql://user:pass@host/db", "postgres"},
		{"test.db", "sqlite"},
		{"/path/to/file.db", "sqlite"},
	}
	for _, tt := range tests {
		if got := DetectType(tt.dsn); got != tt.want {
			t.Errorf("DetectType(%q) = %q, want %q", tt.dsn, got, tt.want)
		}
	}
}

func TestDisplayName(t *testing.T) {
	tests := []struct {
		dsn  string
		want string
	}{
		{"test.db", "test.db"},
		{"/path/to/data.db", "data.db"},
		{"mysql://user:pass@myhost:3306/mydb", "myhost"},
		{"postgres://user:pass@pghost:5432/pgdb", "pghost"},
		{"postgres://user@localhost/db", "localhost"},
	}
	for _, tt := range tests {
		if got := DisplayName(tt.dsn); got != tt.want {
			t.Errorf("DisplayName(%q) = %q, want %q", tt.dsn, got, tt.want)
		}
	}
}

func TestMaskDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{"sqlite path unchanged", "test.db", "test.db"},
		{"userinfo password masked", "mysql://user:secret@host/db", "mysql://user:%2A%2A%2A@host/db"},
		{"postgres password masked", "postgres://admin:p4ss@host:5432/db", "postgres://admin:%2A%2A%2A@host:5432/db"},
		{"sqlite file unchanged", "sqlite.db", "sqlite.db"},
		{"query param password masked", "postgres://user@host/db?password=secret", "postgres://user@host/db?password=%2A%2A%2A"},
		{"both userinfo and query param", "mysql://user:pass@host/db?password=secret", "mysql://user:%2A%2A%2A@host/db?password=%2A%2A%2A"},
		{"no password unchanged", "postgres://user@host/db", "postgres://user@host/db"},
		{"malformed URL best-effort", "postgres://user:secret@host:5432/db%zz", "postgres://user:***@host:5432/db%zz"},
		// net/url reads userinfo up to the *last* '@', so the password here is
		// "sec@ret". Masking only to the first '@' would print "ret@host" back.
		{"malformed URL with '@' in the password", "mysql://user:sec@ret@host:bad/db", "mysql://user:***@host:bad/db"},
		// The bound at '/' keeps the match inside the authority: an '@' in the
		// path must not extend the masked span.
		{"malformed URL with '@' in the path", "mysql://user:secret@host:bad/db@x", "mysql://user:***@host:bad/db@x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskDSN(tt.dsn); got != tt.want {
				t.Errorf("MaskDSN(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}
