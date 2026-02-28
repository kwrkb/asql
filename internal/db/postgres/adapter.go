package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/dbutil"
)

type Adapter struct {
	conn *sql.DB
}

// Open connects to a PostgreSQL database using the given DSN.
// Accepts postgres:// or postgresql:// URL format.
func Open(dsn string) (*Adapter, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}

	return &Adapter{conn: conn}, nil
}

func (a *Adapter) Type() string { return "postgres" }

func (a *Adapter) Close() error {
	return a.conn.Close()
}

func (a *Adapter) Tables(ctx context.Context) ([]string, error) {
	rows, err := a.conn.QueryContext(ctx,
		"SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename")
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
	// Build CREATE TABLE statements from information_schema.columns
	tables, err := a.Tables(ctx)
	if err != nil {
		return "", err
	}

	var stmts []string
	for _, t := range tables {
		ddl, err := a.buildCreateTable(ctx, t)
		if err != nil {
			return "", err
		}
		stmts = append(stmts, ddl+";")
	}
	return strings.Join(stmts, "\n\n"), nil
}

func (a *Adapter) buildCreateTable(ctx context.Context, tableName string) (string, error) {
	rows, err := a.conn.QueryContext(ctx, `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position`, tableName)
	if err != nil {
		return "", fmt.Errorf("querying columns for %s: %w", tableName, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var name, dataType, nullable string
		var defaultVal *string
		if err := rows.Scan(&name, &dataType, &nullable, &defaultVal); err != nil {
			return "", err
		}
		col := fmt.Sprintf("  %s %s", name, dataType)
		if nullable == "NO" {
			col += " NOT NULL"
		}
		if defaultVal != nil {
			col += " DEFAULT " + *defaultVal
		}
		cols = append(cols, col)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	quoted := `"` + strings.ReplaceAll(tableName, `"`, `""`) + `"`
	if len(cols) == 0 {
		return fmt.Sprintf("CREATE TABLE %s ()", quoted), nil
	}

	return fmt.Sprintf("CREATE TABLE %s (\n%s\n)", quoted, strings.Join(cols, ",\n")), nil
}

func (a *Adapter) Query(ctx context.Context, query string) (db.QueryResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return db.QueryResult{}, fmt.Errorf("query is empty")
	}

	if returnsRows(query) {
		rows, err := a.conn.QueryContext(ctx, query)
		if err != nil {
			return db.QueryResult{}, err
		}
		defer rows.Close()
		return dbutil.ScanRows(rows)
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

// returnsRows determines whether a SQL statement returns a result set.
// PostgreSQL supports RETURNING clause.
func returnsRows(query string) bool {
	keyword := dbutil.LeadingKeyword(query)
	switch keyword {
	case "select", "show", "explain", "values", "table":
		return true
	case "with":
		body := dbutil.CteBodyKeyword(query)
		switch body {
		case "select", "values", "table", "show", "explain":
			return true
		default:
			return containsReturning(query)
		}
	default:
		return containsReturning(query)
	}
}

// isIdentChar reports whether c is a SQL identifier character.
func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// containsReturning scans query for the RETURNING keyword, correctly skipping
// string literals, quoted identifiers, comments, and dollar-quoted strings.
func containsReturning(query string) bool {
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
			i++
			for i < n {
				if query[i] == '\'' {
					i++
					if i < n && query[i] == '\'' {
						i++
						continue
					}
					break
				}
				i++
			}
		case query[i] == '"':
			i++
			for i < n {
				if query[i] == '"' {
					i++
					if i < n && query[i] == '"' {
						i++
						continue
					}
					break
				}
				i++
			}
		case query[i] == '$' && i+1 < n:
			// Dollar-quoted string: $tag$...$tag$
			tag := parseDollarTag(query, i)
			if tag != "" {
				i += len(tag)
				for i+len(tag) <= n {
					if query[i:i+len(tag)] == tag {
						i += len(tag)
						break
					}
					i++
				}
			} else {
				i++
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

// parseDollarTag extracts a dollar-quote tag like $$ or $tag$ starting at position i.
// Returns the full tag string (e.g. "$$" or "$tag$"), or "" if not a valid dollar-quote.
func parseDollarTag(query string, i int) string {
	if i >= len(query) || query[i] != '$' {
		return ""
	}
	j := i + 1
	for j < len(query) && query[j] != '$' {
		c := query[j]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return ""
		}
		j++
	}
	if j >= len(query) {
		return ""
	}
	return query[i : j+1] // includes closing $
}
