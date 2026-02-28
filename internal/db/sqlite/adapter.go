package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/dbutil"
)

type Adapter struct {
	conn *sql.DB
}

func Open(path string) (*Adapter, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}

	return &Adapter{conn: conn}, nil
}

func (a *Adapter) Type() string { return "sqlite" }

func (a *Adapter) Close() error {
	return a.conn.Close()
}

func (a *Adapter) Tables(ctx context.Context) ([]string, error) {
	rows, err := a.conn.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (a *Adapter) Schema(ctx context.Context) (string, error) {
	rows, err := a.conn.QueryContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND sql IS NOT NULL ORDER BY name")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var stmts []string
	for rows.Next() {
		var sql string
		if err := rows.Scan(&sql); err != nil {
			return "", err
		}
		stmts = append(stmts, sql+";")
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return strings.Join(stmts, "\n\n"), nil
}

func (a *Adapter) Query(ctx context.Context, query string) (db.QueryResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return db.QueryResult{}, fmt.Errorf("query is empty")
	}

	if returnsRows(query) {
		return a.queryRows(ctx, query)
	}

	res, err := a.conn.ExecContext(ctx, query)
	if err != nil {
		return db.QueryResult{}, err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return db.QueryResult{Message: "statement executed (rows affected unknown)"}, nil
	}

	return db.QueryResult{
		Message: fmt.Sprintf("%d row(s) affected", rowsAffected),
	}, nil
}

func (a *Adapter) queryRows(ctx context.Context, query string) (db.QueryResult, error) {
	rows, err := a.conn.QueryContext(ctx, query)
	if err != nil {
		return db.QueryResult{}, err
	}
	defer rows.Close()

	return dbutil.ScanRows(rows)
}

// returnsRows determines whether a SQL statement will produce a result set.
// Two strategies:
//  1. Leading keyword: SELECT, PRAGMA, WITH, EXPLAIN, VALUES always return rows.
//  2. RETURNING clause: any DML with a RETURNING clause returns rows.
func returnsRows(query string) bool {
	keyword := dbutil.LeadingKeyword(query)
	if keyword == "" {
		return false
	}
	switch keyword {
	case "select", "pragma", "with", "explain", "values":
		return true
	default:
		return containsReturning(query)
	}
}

// isIdentChar reports whether c is a SQL identifier character.
func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// containsReturning scans query for the RETURNING keyword, correctly skipping
// string literals, quoted identifiers, and comments.
func containsReturning(query string) bool {
	const kw = "returning"
	i := 0
	n := len(query)
	for i < n {
		switch {
		case query[i] == '-' && i+1 < n && query[i+1] == '-':
			// line comment: skip to end of line
			for i < n && query[i] != '\n' {
				i++
			}
		case query[i] == '/' && i+1 < n && query[i+1] == '*':
			// block comment
			i += 2
			for i < n {
				if query[i] == '*' && i+1 < n && query[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
		case query[i] == '\'':
			// single-quoted string literal ('' is an escaped quote)
			i++
			for i < n {
				if query[i] == '\'' {
					i++
					if i < n && query[i] == '\'' {
						i++ // escaped quote, continue
						continue
					}
					break
				}
				i++
			}
		case query[i] == '"':
			// double-quoted identifier ("" is an escaped quote)
			i++
			for i < n {
				if query[i] == '"' {
					i++
					if i < n && query[i] == '"' {
						i++ // escaped quote, continue
						continue
					}
					break
				}
				i++
			}
		case query[i] == '`':
			// backtick-quoted identifier (SQLite also accepts MySQL-style backticks)
			i++
			for i < n && query[i] != '`' {
				i++
			}
			if i < n {
				i++ // skip closing `
			}
		case query[i] == '[':
			// bracket-quoted identifier ([id], SQLite/MSSQL style)
			i++
			for i < n && query[i] != ']' {
				i++
			}
			if i < n {
				i++ // skip closing ]
			}
		default:
			if i+len(kw) <= n && strings.EqualFold(query[i:i+len(kw)], kw) {
				before := i == 0 || !isIdentChar(query[i-1])
				after := i+len(kw) >= n || !isIdentChar(query[i+len(kw)])
				if before && after {
					return true
				}
			}
			i++
		}
	}
	return false
}

