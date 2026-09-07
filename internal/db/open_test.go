package db

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

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
		// The key is compared case-insensitively. A DSN url.Parse accepts is
		// what the status bar and the profile overlay render, so a spelling the
		// parsed branch skipped was a password on screen.
		{"upper-case query param password masked", "postgres://user@host/db?PASSWORD=secret", "postgres://user@host/db?PASSWORD=%2A%2A%2A"},
		{"mixed-case query param password masked", "postgres://user@host/db?Password=secret&sslmode=require", "postgres://user@host/db?Password=%2A%2A%2A&sslmode=require"},
		{"no password unchanged", "postgres://user@host/db", "postgres://user@host/db"},
		// The over-masking below is confined to DSNs url.Parse rejects. A
		// well-formed DSN with an '@' in the database name — which is what the
		// profile overlay and status bar actually render — keeps its host,
		// because it never reaches the malformed branch.
		{"well-formed URL with '@' in the path keeps its host", "mysql://user:secret@db.internal:3306/analytics@staging", "mysql://user:%2A%2A%2A@db.internal:3306/analytics@staging"},
		{"malformed URL best-effort", "postgres://user:secret@host:5432/db%zz", "postgres://user:***@host:5432/db%zz"},
		// net/url reads userinfo up to the *last* '@', so the password here is
		// "sec@ret". Masking only to the first '@' would print "ret@host" back.
		{"malformed URL with '@' in the password", "mysql://user:sec@ret@host:bad/db", "mysql://user:***@host:bad/db"},
		// An '@' in the path extends the masked span to it, hiding the host.
		// That is the accepted cost of masking to the last '@': a rejected DSN
		// has no grammar left that says which '@' ends the userinfo, and
		// over-masking loses an error message, under-masking loses a secret.
		{"malformed URL with '@' in the path", "mysql://user:secret@host:bad/db@x", "mysql://user:***@x"},
		// A password holding both '@' and '/' used to match a span too *short*
		// to cover it — "e:bad/cret" was printed — because the earlier pattern
		// stopped at the first '/' and matched anyway, so no fallback ran.
		{"malformed URL with '@' before '/' in the password", "postgres://alice:s@e:bad/cret@host/db", "postgres://alice:***@host/db"},
		// A literal '/' in the password. url.Parse rejects it, so it only ever
		// reaches this branch, and a pattern bounded at '/' could not match it
		// at all — the DSN came back with the password intact.
		{"malformed URL with '/' in the password", "postgres://alice:se/cret@localhost:bad/db", "postgres://alice:***@localhost:bad/db"},
		{"malformed URL with '/' and '@' in the password", "postgres://alice:se/cr@et@localhost:bad/db", "postgres://alice:***@localhost:bad/db"},
		// url.Parse rejects a control character anywhere in the URL, so a
		// password holding a newline always lands on the malformed path. The
		// pattern is dot-all so the span still matches; without that it matched
		// nothing and the credential came back whole.
		{"malformed URL with a newline in the password", "postgres://alice:se\ncret@localhost/db", "postgres://alice:***@localhost/db"},
		{"malformed URL with a newline in the user", "postgres://al\nice:secret@localhost/db", "postgres://al\nice:***@localhost/db"},
		// The parsed path masks a password= parameter; the malformed path has
		// no RawQuery to read, so it has to do the same textually.
		{"malformed URL with a password parameter", "postgres://alice@localhost:bad/db?password=secret", "postgres://alice@localhost:bad/db?password=***"},
		{"malformed URL with an upper-case password parameter", "postgres://alice@localhost:bad/db?PASSWORD=secret&x=1", "postgres://alice@localhost:bad/db?PASSWORD=***&x=1"},
		// Nothing that looks like a credential: the DSN must survive readable,
		// or a bad-host error stops naming the host.
		{"malformed URL with no password", "postgres://alice@ho|st/db", "postgres://alice@ho|st/db"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskDSN(tt.dsn); got != tt.want {
				t.Errorf("MaskDSN(%q) = %q, want %q", tt.dsn, got, tt.want)
			}
		})
	}
}

// URLParseError must name the kind of failure and show the DSN masked, and
// must never quote the input back. Every cause url.Parse produces embeds part
// of the input — url.EscapeError is the whole password in a DSN like
// postgres://alice:%ss@host/db — so a caller that printed the cause verbatim
// would leak it.
func TestURLParseError(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantMsg string
		secrets []string
	}{
		{
			"invalid percent-escape",
			"postgres://alice:%ss@host/db",
			"parsing PostgreSQL URL postgres://alice:***@host/db: invalid percent-escape",
			[]string{"%ss"},
		},
		{
			// The invalid-port cause is an unnamed errors.errorString, so the
			// default arm is what keeps it from being printed.
			"invalid port",
			"postgres://alice:secret@host:bad/db",
			"parsing PostgreSQL URL postgres://alice:***@host:bad/db: invalid URL",
			[]string{"secret"},
		},
		{
			"invalid character in host",
			"postgres://alice:secret@ho|st/db",
			"parsing PostgreSQL URL postgres://alice:***@ho|st/db: invalid character in host",
			[]string{"secret"},
		},
		{
			// The leak this guards is the whole reason the mask pattern is
			// dot-all: url.Parse refuses the DSN, so the raw password would
			// otherwise travel to stderr and the TUI inside this message.
			"newline in the password",
			"postgres://alice:se\ncret@localhost/db",
			"parsing PostgreSQL URL postgres://alice:***@localhost/db: invalid URL",
			[]string{"se\ncret", "cret"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := url.Parse(tt.dsn)
			if err == nil {
				t.Fatalf("url.Parse(%q) expected an error, got nil", tt.dsn)
			}
			got := URLParseError("PostgreSQL", tt.dsn, err).Error()
			if got != tt.wantMsg {
				t.Errorf("URLParseError() = %q, want %q", got, tt.wantMsg)
			}
			for _, secret := range tt.secrets {
				if strings.Contains(got, secret) {
					t.Errorf("URLParseError() leaks %q: %q", secret, got)
				}
			}
		})
	}
}

// The returned error must not carry the original as a cause: a caller that
// unwrapped it would get url.Error's copy of the raw DSN back.
func TestURLParseErrorDropsTheCause(t *testing.T) {
	dsn := "postgres://alice:review-secret@localhost:bad/db"
	_, cause := url.Parse(dsn)
	err := URLParseError("PostgreSQL", dsn, cause)
	if errors.Unwrap(err) != nil {
		t.Errorf("URLParseError() wraps a cause that can be unwrapped and printed: %v", errors.Unwrap(err))
	}
	if strings.Contains(err.Error(), "review-secret") {
		t.Errorf("URLParseError() leaks the password: %q", err.Error())
	}
}
