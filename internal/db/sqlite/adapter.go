package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/kwrkb/sqly/internal/db"
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

func (a *Adapter) Close() error {
	return a.conn.Close()
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
		return db.QueryResult{Message: "statement executed"}, nil
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

	columns, err := rows.Columns()
	if err != nil {
		return db.QueryResult{}, err
	}

	values := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	resultRows := make([][]string, 0)

	for i := range values {
		ptrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return db.QueryResult{}, err
		}

		record := make([]string, len(columns))
		for i, value := range values {
			record[i] = stringifyValue(value)
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

func returnsRows(query string) bool {
	keyword := leadingKeyword(query)
	if keyword == "" {
		return false
	}

	switch keyword {
	case "select", "pragma", "with", "explain", "values":
		return true
	default:
		return false
	}
}

func leadingKeyword(query string) string {
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

func stringifyValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339)
	default:
		return fmt.Sprint(v)
	}
}
