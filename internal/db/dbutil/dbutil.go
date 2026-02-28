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

// ScanRows reads all rows from *sql.Rows and returns a QueryResult.
// The caller is responsible for closing rows.
func ScanRows(rows *sql.Rows) (db.QueryResult, error) {
	columns, err := rows.Columns()
	if err != nil {
		return db.QueryResult{}, err
	}

	values := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	for i := range values {
		ptrs[i] = &values[i]
	}

	resultRows := make([][]string, 0)
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return db.QueryResult{}, err
		}
		record := make([]string, len(columns))
		for i, value := range values {
			record[i] = StringifyValue(value)
		}
		resultRows = append(resultRows, record)
	}
	if err := rows.Err(); err != nil {
		return db.QueryResult{}, err
	}

	return db.QueryResult{
		Columns: columns,
		Rows:    resultRows,
		Message: fmt.Sprintf("%d row(s) returned", len(resultRows)),
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
