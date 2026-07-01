// Package bring materializes query results from any DBAdapter into a local
// in-memory SQLite database so they can be JOINed together, per the asql
// "Bring Data" philosophy: bring data locally rather than integrating
// heterogeneous databases directly.
package bring

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/kwrkb/asql/internal/db"
	"github.com/kwrkb/asql/internal/db/sqlite"
)

// batchSize bounds how many rows are inserted per statement to stay well
// under SQLite's bound-parameter limit (default ~32766 in modernc.org/sqlite).
const batchSize = 500

// Open creates a fresh in-memory SQLite database for the Bring & Join
// feature. The returned *sql.DB and *sqlite.Adapter share the same
// connection, so rows written via Materialize are immediately visible
// through the adapter's Query (used to run JOINs against brought tables).
func Open() (*sql.DB, *sqlite.Adapter, error) {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, nil, err
	}
	adapter, err := sqlite.NewAdapter(conn)
	if err != nil {
		return nil, nil, err
	}
	return conn, adapter, nil
}

// Materialize creates a new table named tableName in conn and copies result
// into it. Every column is stored as TEXT: QueryResult.Rows is already
// stringified by the source adapter's scan (see dbutil.StringifyValue), so no
// typed value survives to materialize from. Display sentinels are reversed at
// insert time so brought values round-trip close to their original form (see
// reverseSentinel).
func Materialize(ctx context.Context, conn *sql.DB, quote func(string) string, tableName string, result db.QueryResult) error {
	cols := disambiguateColumns(result.Columns)

	if err := createTable(ctx, conn, quote, tableName, cols); err != nil {
		return err
	}
	if len(result.Rows) == 0 || len(cols) == 0 {
		return nil
	}
	return insertRows(ctx, conn, quote, tableName, cols, result.Rows)
}

func createTable(ctx context.Context, conn *sql.DB, quote func(string) string, tableName string, cols []string) error {
	var sb strings.Builder
	sb.WriteString("CREATE TABLE ")
	sb.WriteString(quote(tableName))
	sb.WriteString(" (")
	for i, c := range cols {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(quote(c))
		sb.WriteString(" TEXT")
	}
	sb.WriteString(")")

	if _, err := conn.ExecContext(ctx, sb.String()); err != nil {
		return fmt.Errorf("create table %s: %w", tableName, err)
	}
	return nil
}

func insertRows(ctx context.Context, conn *sql.DB, quote func(string) string, tableName string, cols []string, rows [][]string) error {
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = quote(c)
	}
	rowPlaceholder := "(" + strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",") + ")"
	insertPrefix := fmt.Sprintf("INSERT INTO %s (%s) VALUES ", quote(tableName), strings.Join(quotedCols, ", "))

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for start := 0; start < len(rows); start += batchSize {
		batch := rows[start:min(start+batchSize, len(rows))]

		var query strings.Builder
		query.WriteString(insertPrefix)
		args := make([]any, 0, len(batch)*len(cols))
		for i, row := range batch {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString(rowPlaceholder)
			for c := range cols {
				var cell string
				if c < len(row) {
					cell = row[c]
				}
				args = append(args, reverseSentinel(cell))
			}
		}
		if _, err := tx.ExecContext(ctx, query.String(), args...); err != nil {
			return fmt.Errorf("insert into %s: %w", tableName, err)
		}
	}

	return tx.Commit()
}

// reverseSentinel undoes dbutil.StringifyValue's display sentinels so
// brought values round-trip through SQLite close to their original form: the
// "NULL" sentinel becomes a real SQL NULL, and the `""` (two double-quote
// characters) sentinel becomes an empty string. This is lossy by design — a
// genuine source value that was literally the text NULL or empty after
// stringify is indistinguishable from the sentinel. Known, accepted
// limitation of the all-TEXT bring design.
func reverseSentinel(s string) any {
	switch s {
	case "NULL":
		return nil
	case `""`:
		return ""
	default:
		return s
	}
}

// disambiguateColumns renames blank or duplicate column names so every
// column in cols is unique, e.g. ["id", "id"] -> ["id", "id_2"]. This is
// required because JOIN results and queries like "SELECT t1.id, t2.id" can
// produce duplicate column names, which CREATE TABLE rejects outright.
func disambiguateColumns(cols []string) []string {
	used := make(map[string]bool, len(cols))
	out := make([]string, len(cols))
	for i, c := range cols {
		name := c
		if name == "" {
			name = "col"
		}
		base := name
		for n := 2; used[name]; n++ {
			name = fmt.Sprintf("%s_%d", base, n)
		}
		used[name] = true
		out[i] = name
	}
	return out
}
