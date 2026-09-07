package db

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// rePasswordInDSN masks the password in a DSN url.Parse could not read: from
// the first ':' after "://" — the delimiter that starts the password — to the
// *last* '@' in the string.
//
// Running to the last '@' rather than stopping at the authority is deliberate.
// net/url reads userinfo greedily, so in "mysql://user:sec@ret@host/db" the
// password really is "sec@ret" and a shorter span prints "ret@host" back. There
// is no bound that is right for every rejected DSN, because a rejected DSN has
// no grammar left to appeal to: a '/' in it is irreducibly ambiguous between
// path-start and password byte, so any bound at '/' both misses a password
// containing one and, worse, can match a span too *short* to cover the secret.
//
// So it over-masks instead: an '@' in the path swallows the host, and
// "mysql://user:secret@host:bad/db@x" masks down to "mysql://user:***@x". That
// is the direction to fail in — a hidden host is a worse error message, a
// printed password is a leaked credential.
//
// The (?s) flag is load-bearing: url.Parse rejects a DSN whose password holds a
// newline, so such a DSN reaches here by construction, and without dot-all the
// span between ':' and '@' would not match at all — MaskDSN would then hand the
// whole credential back unmasked. Only '\n' needs the flag; '.' already covers
// every other control byte.
var rePasswordInDSN = regexp.MustCompile(`(?s)(://[^:]*:)(.*)(@)`)

// isPasswordParam reports whether a query-parameter key names a password. Both
// halves of MaskDSN consult it, so a key one half masks cannot be a key the
// other half prints. It is case-insensitive because the value is a secret
// whether or not the driver ends up honouring the spelling.
func isPasswordParam(key string) bool {
	return strings.EqualFold(key, "password")
}

// maskPasswordParams masks password query parameters in a DSN url.Parse
// rejected. The parsed path does this through url.Values; here there is no
// RawQuery to read, so the query is split textually.
func maskPasswordParams(dsn string) string {
	i := strings.IndexByte(dsn, '?')
	if i < 0 {
		return dsn
	}
	params := strings.Split(dsn[i+1:], "&")
	for j, p := range params {
		eq := strings.IndexByte(p, '=')
		if eq < 0 || !isPasswordParam(p[:eq]) {
			continue
		}
		params[j] = p[:eq+1] + "***"
	}
	return dsn[:i+1] + strings.Join(params, "&")
}

// maskMalformedDSN is the best effort for a DSN url.Parse could not read. It is
// not a fallback for display alone: every DSN reported in a url.Parse *error*
// arrives here by construction, and a stored profile reaches the status bar and
// the profile overlay through here too, so anything left unmatched is a raw
// credential on screen.
func maskMalformedDSN(dsn string) string {
	return maskPasswordParams(rePasswordInDSN.ReplaceAllString(dsn, "${1}***${3}"))
}

// MaskDSN returns a display-safe version of the DSN with passwords masked.
func MaskDSN(dsn string) string {
	if !strings.Contains(dsn, "://") {
		return dsn
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return maskMalformedDSN(dsn)
	}
	masked := false
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "***")
			masked = true
		}
	}
	q := u.Query()
	maskedParam := false
	for key := range q {
		if isPasswordParam(key) && q.Get(key) != "" {
			q.Set(key, "***")
			maskedParam = true
		}
	}
	if maskedParam {
		u.RawQuery = q.Encode()
		masked = true
	}
	if !masked {
		return dsn
	}
	return u.String()
}

// URLParseError reports a url.Parse failure for a DSN without leaking it.
//
// Neither half of a url.Parse failure is safe to print. url.Error embeds the
// raw URL, and its cause embeds a fragment of it — url.EscapeError holds the
// offending "%xx" sequence, which is the whole password in a DSN like
// postgres://alice:%ss@host/db. These errors reach stderr and the TUI, so this
// reports the DSN masked, reduces the cause to the kind of failure, and drops
// the original error rather than wrapping it, so it cannot travel to a caller
// that prints it.
//
// It exists so that rule lives in one signature rather than in a comment beside
// every adapter's url.Parse call: a caller cannot reach for %w without noticing
// it is stepping around this.
//
// label names the DSN's flavour for the message, e.g. "MySQL" or "PostgreSQL".
func URLParseError(label, dsn string, err error) error {
	return fmt.Errorf("parsing %s URL %s: %s", label, MaskDSN(dsn), urlParseCause(err))
}

// urlParseCause names the kind of url.Parse failure without quoting any of the
// DSN back. The causes that embed input do so unpredictably — url.Parse reads
// userinfo greedily, so which bytes land in an escape or port error is not
// something the DSN's shape predicts — which is why this describes the known
// kinds and says nothing more for the rest, rather than allow-listing causes
// believed to be safe to print verbatim. The invalid-port failure in particular
// arrives as an unnamed errors.errorString that no type switch can match, so
// the default is what covers it.
func urlParseCause(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	switch err.(type) {
	case url.EscapeError:
		return "invalid percent-escape"
	case url.InvalidHostError:
		return "invalid character in host"
	}
	return "invalid URL"
}

// DetectType returns the database type string for a given DSN.
func DetectType(dsn string) string {
	switch {
	case strings.HasPrefix(dsn, "mysql://"):
		return "mysql"
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return "postgres"
	default:
		return "sqlite"
	}
}

// InitialQuery returns an appropriate initial query for the database type.
func InitialQuery(dbType string) string {
	switch dbType {
	case "mysql":
		return "SELECT VERSION();"
	case "postgres":
		return "SELECT version();"
	default:
		return "SELECT sqlite_version();"
	}
}

// Placeholder returns an appropriate placeholder query for the database type.
func Placeholder(dbType string) string {
	switch dbType {
	case "mysql":
		return "SHOW TABLES;"
	case "postgres":
		return "SELECT tablename FROM pg_tables WHERE schemaname = 'public';"
	default:
		return "SELECT name FROM sqlite_master WHERE type = 'table';"
	}
}

// DisplayName returns a short display name for a DSN.
func DisplayName(dsn string) string {
	switch {
	case strings.HasPrefix(dsn, "mysql://"),
		strings.HasPrefix(dsn, "postgres://"),
		strings.HasPrefix(dsn, "postgresql://"):
		return extractHost(dsn)
	default:
		parts := strings.Split(dsn, "/")
		return parts[len(parts)-1]
	}
}

func extractHost(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return dsn
	}
	host := u.Hostname()
	if host == "" {
		return "localhost"
	}
	return host
}
