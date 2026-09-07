package db

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// rePasswordInDSN masks the password in a DSN url.Parse could not read. The
// password group stops at '/' rather than at '@' so it can backtrack to the
// *last* '@' before the path — net/url reads userinfo greedily, so in
// "mysql://user:sec@ret@host/db" the password is "sec@ret", and stopping at
// the first '@' would print "ret@host" back. Bounding it at '/' keeps the
// match from running past the authority into an '@' in the path.
var rePasswordInDSN = regexp.MustCompile(`(://[^:]*:)([^/]*)(@)`)

// MaskDSN returns a display-safe version of the DSN with passwords masked.
func MaskDSN(dsn string) string {
	if !strings.Contains(dsn, "://") {
		return dsn
	}
	u, err := url.Parse(dsn)
	if err != nil {
		// Best-effort: mask password in malformed URLs
		return rePasswordInDSN.ReplaceAllString(dsn, "${1}***${3}")
	}
	masked := false
	if u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "***")
			masked = true
		}
	}
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

// URLParseCause names the kind of url.Parse failure without quoting any of the
// DSN back. Both halves of a url.Parse failure carry the input: url.Error
// embeds the raw URL, and its cause embeds a fragment of it — url.EscapeError
// holds the offending "%xx" sequence, which is the whole password in a DSN like
// postgres://alice:%ss@host/db. The causes that embed input do so unpredictably
// — url.Parse reads userinfo greedily, so which bytes land in an escape or port
// error is not something the DSN's shape predicts — which is why this describes
// the known kinds and says nothing more for the rest, rather than allow-listing
// causes believed to be safe to print verbatim.
//
// Callers report the DSN through MaskDSN and this cause instead of wrapping the
// original error, so the raw error cannot travel to a caller that prints it.
func URLParseCause(err error) string {
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
