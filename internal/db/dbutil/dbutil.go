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
		case strings.HasPrefix(trimmed, "--"):
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
