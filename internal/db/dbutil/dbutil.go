package dbutil

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kwrkb/asql/internal/db"
)

// StringifyValue converts a database value to its string representation.
func StringifyValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case []byte:
		if utf8.Valid(v) {
			return string(v)
		}
		return fmt.Sprintf("%x", v)
	case time.Time:
		return v.Format(time.RFC3339)
	default:
		return fmt.Sprint(v)
	}
}

// DefaultRowLimit is the maximum number of rows ScanRows will read.
// Use ScanRowsLimit to override this default.
const DefaultRowLimit = 10_000

// ScanRows reads rows from *sql.Rows up to DefaultRowLimit and returns a QueryResult.
// The caller is responsible for closing rows.
func ScanRows(rows *sql.Rows) (db.QueryResult, error) {
	return ScanRowsLimit(rows, DefaultRowLimit)
}

// ScanRowsLimit reads rows from *sql.Rows up to the given limit.
// A limit of 0 means no limit.
func ScanRowsLimit(rows *sql.Rows, limit int) (db.QueryResult, error) {
	columns, err := rows.Columns()
	if err != nil {
		return db.QueryResult{}, err
	}

	// Retrieve column type names (best-effort; driver-dependent).
	var colTypes []string
	if cts, err := rows.ColumnTypes(); err == nil {
		colTypes = make([]string, len(cts))
		for i, ct := range cts {
			colTypes[i] = ct.DatabaseTypeName()
		}
	}

	values := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	for i := range values {
		ptrs[i] = &values[i]
	}

	resultRows := make([][]string, 0)
	truncated := false
	for rows.Next() {
		if limit > 0 && len(resultRows) >= limit {
			truncated = true
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return db.QueryResult{}, err
		}
		record := make([]string, len(columns))
		for i, value := range values {
			s := StringifyValue(value)
			if s == "" {
				s = `""`
			}
			record[i] = s
		}
		resultRows = append(resultRows, record)
	}
	if err := rows.Err(); err != nil {
		return db.QueryResult{}, err
	}

	msg := fmt.Sprintf("%d row(s) returned", len(resultRows))
	if truncated {
		msg = fmt.Sprintf("%d row(s) returned (truncated at %d)", len(resultRows), limit)
	}

	return db.QueryResult{
		Columns:     columns,
		ColumnTypes: colTypes,
		Rows:        resultRows,
		Message:     msg,
		Truncated:   truncated,
	}, nil
}

// LeadingKeyword returns the first SQL keyword from query, skipping comments
// and leading semicolons. The result is always lowercase.
func LeadingKeyword(query string) string {
	trimmed := strings.TrimSpace(query)

	for trimmed != "" {
		switch {
		case strings.HasPrefix(trimmed, "--"), strings.HasPrefix(trimmed, "#"):
			if idx := strings.Index(trimmed, "\n"); idx >= 0 {
				trimmed = strings.TrimSpace(trimmed[idx+1:])
				continue
			}
			return ""
		case strings.HasPrefix(trimmed, "/*"):
			if idx := strings.Index(trimmed, "*/"); idx >= 0 {
				trimmed = strings.TrimSpace(trimmed[idx+2:])
				continue
			}
			return ""
		case strings.HasPrefix(trimmed, ";"):
			trimmed = strings.TrimSpace(trimmed[1:])
			continue
		}
		break
	}

	fields := strings.Fields(strings.ToLower(trimmed))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// CteBodyKeyword extracts the leading keyword of the body statement in a
// WITH (CTE) query. It skips the WITH keyword, then tracks parenthesis depth
// to skip over CTE definitions, and returns the first keyword of the body
// (e.g. "select", "insert", "delete"). Returns "" if parsing fails.
// It correctly handles string literals, comments, and parentheses.
func CteBodyKeyword(query string) string {
	i := 0
	n := len(query)

	// Skip to past the WITH keyword (LeadingKeyword already confirmed it's "with")
	i = skipWhitespaceAndComments(query, i)
	if i+4 > n {
		return ""
	}
	// Advance past "WITH"
	i += 4
	// Skip optional RECURSIVE
	j := skipWhitespaceAndComments(query, i)
	if j+9 <= n && strings.EqualFold(query[j:j+9], "recursive") {
		after := j + 9
		if after >= n || !isIdentCharByte(query[after]) {
			i = after
		}
	}

	// Now skip CTE definitions by tracking parenthesis depth.
	// We need to find the body keyword after all CTE definitions end.
	depth := 0
	for i < n {
		i = skipWhitespaceAndComments(query, i)
		if i >= n {
			break
		}
		c := query[i]
		switch {
		case c == '(':
			depth++
			i++
		case c == ')':
			depth--
			i++
		case c == '\'':
			i = skipSingleQuoted(query, i)
		case c == '"':
			i = skipDoubleQuoted(query, i)
		case c == '`':
			i = skipBacktickQuoted(query, i)
		case c == '$' && i+1 < n:
			i = skipDollarQuoted(query, i)
		default:
			if depth == 0 && c != ',' {
				// We're outside all CTE parentheses and it's not a comma separator.
				// Check if this is a SQL keyword (the body statement).
				if isIdentCharByte(c) {
					end := i
					for end < n && isIdentCharByte(query[end]) {
						end++
					}
					word := strings.ToLower(query[i:end])
					// CTE name or AS keyword — keep scanning
					if word == "as" {
						i = end
						continue
					}
					// Could be a CTE name followed by column list or AS
					// Check if this is a known body keyword
					switch word {
					case "select", "insert", "update", "delete", "merge",
						"values", "table", "show", "describe", "desc",
						"explain", "pragma":
						return word
					default:
						// CTE alias — skip past it
						i = end
						continue
					}
				}
				i++
			} else {
				i++
			}
		}
	}
	return ""
}

func isIdentCharByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

func skipWhitespaceAndComments(query string, i int) int {
	n := len(query)
	for i < n {
		// Skip whitespace
		if query[i] == ' ' || query[i] == '\t' || query[i] == '\n' || query[i] == '\r' {
			i++
			continue
		}
		// Line comment --
		if i+1 < n && query[i] == '-' && query[i+1] == '-' {
			for i < n && query[i] != '\n' {
				i++
			}
			continue
		}
		// MySQL # comment
		if query[i] == '#' {
			for i < n && query[i] != '\n' {
				i++
			}
			continue
		}
		// Block comment
		if i+1 < n && query[i] == '/' && query[i+1] == '*' {
			i += 2
			for i < n {
				if i+1 < n && query[i] == '*' && query[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}
		break
	}
	return i
}

func skipSingleQuoted(query string, i int) int {
	n := len(query)
	i++ // skip opening '
	for i < n {
		if query[i] == '\'' {
			i++
			if i < n && query[i] == '\'' {
				i++
				continue
			}
			return i
		}
		i++
	}
	return i
}

func skipDoubleQuoted(query string, i int) int {
	n := len(query)
	i++ // skip opening "
	for i < n {
		if query[i] == '"' {
			i++
			if i < n && query[i] == '"' {
				i++
				continue
			}
			return i
		}
		i++
	}
	return i
}

func skipBacktickQuoted(query string, i int) int {
	n := len(query)
	i++ // skip opening `
	for i < n && query[i] != '`' {
		i++
	}
	if i < n {
		i++ // skip closing `
	}
	return i
}

func skipDollarQuoted(query string, i int) int {
	n := len(query)
	// Try to parse a dollar-quote tag
	tag := parseDollarTag(query, i)
	if tag != "" {
		i += len(tag)
		for i+len(tag) <= n {
			if query[i:i+len(tag)] == tag {
				i += len(tag)
				return i
			}
			i++
		}
		return i
	}
	return i + 1
}

// typeShortNames maps verbose type names to shorter display forms.
var typeShortNames = map[string]string{
	"integer":            "int",
	"unsigned int":       "uint",
	"unsigned bigint":    "ubigint",
	"unsigned mediumint": "umedint",
	"unsigned smallint":  "usmallint",
	"unsigned tinyint":   "utinyint",
	"timestamptz":        "tstz",
}

// ShortenTypeName returns a shortened display name for a database column type.
// Unknown types are returned lower-cased as-is.
func ShortenTypeName(typeName string) string {
	lower := strings.ToLower(typeName)
	if short, ok := typeShortNames[lower]; ok {
		return short
	}
	return lower
}

// Dialect controls which quoting styles the SQL scanner recognizes.
type Dialect struct {
	BracketQuote bool // SQLite/MSSQL [identifier] style
	DollarQuote  bool // PostgreSQL $$string$$ style
	BacktickQuote bool // SQLite/MySQL `identifier` style
}

// ContainsReturning scans query for the RETURNING keyword, correctly skipping
// string literals, quoted identifiers, comments, and dialect-specific quoting.
func ContainsReturning(query string, dialect Dialect) bool {
	const kw = "returning"
	i := 0
	n := len(query)
	for i < n {
		switch {
		case query[i] == '-' && i+1 < n && query[i+1] == '-':
			for i < n && query[i] != '\n' {
				i++
			}
		case query[i] == '/' && i+1 < n && query[i+1] == '*':
			i += 2
			for i < n {
				if query[i] == '*' && i+1 < n && query[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
		case query[i] == '\'':
			i = skipSingleQuoted(query, i)
		case query[i] == '"':
			i = skipDoubleQuoted(query, i)
		case dialect.BacktickQuote && query[i] == '`':
			i = skipBacktickQuoted(query, i)
		case dialect.BracketQuote && query[i] == '[':
			i++
			for i < n && query[i] != ']' {
				i++
			}
			if i < n {
				i++
			}
		case dialect.DollarQuote && query[i] == '$' && i+1 < n:
			i = skipDollarQuoted(query, i)
		default:
			if i+len(kw) <= n && strings.EqualFold(query[i:i+len(kw)], kw) {
				before := i == 0 || !isIdentCharByte(query[i-1])
				after := i+len(kw) >= n || !isIdentCharByte(query[i+len(kw)])
				if before && after {
					return true
				}
			}
			i++
		}
	}
	return false
}

func parseDollarTag(query string, i int) string {
	n := len(query)
	if i >= n || query[i] != '$' {
		return ""
	}
	j := i + 1
	for j < n && query[j] != '$' {
		c := query[j]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return ""
		}
		j++
	}
	if j >= n {
		return ""
	}
	return query[i : j+1]
}
