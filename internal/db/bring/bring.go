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

// maxBoundParams stays well under SQLite's bound-parameter limit (default
// ~32766 in modernc.org/sqlite) to leave headroom for other in-flight
// statements on the same connection.
const maxBoundParams = 20_000

// maxBatchRows caps how many rows go into one INSERT statement even for
// narrow (few-column) results, to keep individual statements reasonably
// sized.
const maxBatchRows = 500

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
	// sqlite.NewAdapter sets a 5-minute connection lifetime, which is safe
	// for file-based databases (reopening the file preserves data) but
	// fatal here: this is a private ":memory:" database that exists only
	// inside this one connection. Closing it to "refresh" the pool would
	// destroy every brought table, so disable the lifetime for this
	// connection specifically.
	conn.SetConnMaxLifetime(0)
	return conn, adapter, nil
}

// Materialize creates a new table named tableName in conn and copies result
// into it. Every column is stored as TEXT: QueryResult.Rows is already
// stringified by the source adapter's scan (see dbutil.StringifyValue), so no
// typed value survives to materialize from. Display sentinels are reversed at
// insert time so brought values round-trip close to their original form (see
// reverseSentinel).
//
// CREATE TABLE and all INSERTs run inside a single transaction so a failure
// partway through (e.g. a bound-parameter overflow) leaves no orphaned empty
// table behind — the caller can safely retry with the same tableName.
func Materialize(ctx context.Context, conn *sql.DB, quote func(string) string, tableName string, result db.QueryResult) error {
	cols := disambiguateColumns(result.Columns)
	if len(cols) == 0 {
		return fmt.Errorf("cannot materialize a result with no columns")
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := createTable(ctx, tx, quote, tableName, cols); err != nil {
		return err
	}
	if len(result.Rows) > 0 {
		if err := insertRows(ctx, tx, quote, tableName, cols, result.Rows); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func createTable(ctx context.Context, tx *sql.Tx, quote func(string) string, tableName string, cols []string) error {
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

	if _, err := tx.ExecContext(ctx, sb.String()); err != nil {
		return fmt.Errorf("create table %s: %w", tableName, err)
	}
	return nil
}

func insertRows(ctx context.Context, tx *sql.Tx, quote func(string) string, tableName string, cols []string, rows [][]string) error {
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = quote(c)
	}
	rowPlaceholder := "(" + strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",") + ")"
	insertPrefix := fmt.Sprintf("INSERT INTO %s (%s) VALUES ", quote(tableName), strings.Join(quotedCols, ", "))

	// Size each batch so rows*cols stays under maxBoundParams, in addition
	// to the flat maxBatchRows cap — a wide result (many columns) must use
	// a smaller batch than a narrow one to avoid exceeding SQLite's bound
	// parameter limit.
	batchRows := max(min(maxBatchRows, maxBoundParams/len(cols)), 1)

	for start := 0; start < len(rows); start += batchRows {
		batch := rows[start:min(start+batchRows, len(rows))]

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

	return nil
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
//
// Uniqueness is checked case-insensitively (SQLite treats column names
// case-insensitively, so "ID" and "id" collide just as much as "id"/"id").
// Every original column name is reserved up front so a generated suffix
// (e.g. "id_2") never collides with — and silently steals data from — an
// original column that already has that exact name.
func disambiguateColumns(cols []string) []string {
	names := make([]string, len(cols))
	for i, c := range cols {
		if c == "" {
			names[i] = "col"
		} else {
			names[i] = c
		}
	}

	reserved := make(map[string]bool, len(names))
	for _, n := range names {
		reserved[strings.ToLower(n)] = true
	}

	used := make(map[string]bool, len(names))
	out := make([]string, len(names))
	for i, base := range names {
		name := base
		key := strings.ToLower(name)
		if used[key] {
			for n := 2; ; n++ {
				candidate := fmt.Sprintf("%s_%d", base, n)
				candidateKey := strings.ToLower(candidate)
				if !used[candidateKey] && !reserved[candidateKey] {
					name, key = candidate, candidateKey
					break
				}
			}
		}
		used[key] = true
		out[i] = name
	}
	return out
}
