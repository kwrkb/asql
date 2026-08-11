// Package bring materializes query results from any DBAdapter into a local
// in-memory SQLite database so they can be JOINed together, per the asql
// "Bring Data" philosophy: bring data locally rather than integrating
// heterogeneous databases directly.
package bring

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strconv"
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

// ProvenanceTable is the name of the bookkeeping table Materialize maintains
// inside the bring database. It records where each brought table came from so
// a user who brought t1, t2 and t3 half an hour ago does not have to remember
// which was which.
//
// It is a plain table, deliberately: it shows up in the sidebar, and it can be
// SELECTed, sorted, exported and JOINed through the existing query pipeline
// with no new UI. The leading underscore sorts it to the top of the table list.
const ProvenanceTable = "_asql_bring"

// Source describes where a brought result came from.
type Source struct {
	Seq   int    // creation order; also the number in Table (t1 -> 1)
	Table string // local table name (t1, t2, ...)
	Conn  string // source connection or profile name
	Query string // the query whose result was brought
}

// Materialize creates a new table named src.Table in conn and copies result
// into it, recording src in ProvenanceTable as part of the same transaction.
//
// Values keep their meaning across the copy when result carries kind
// information (db.QueryResult.Kinds, recorded at scan time): each column is
// declared with the SQLite affinity implied by the kinds observed in it, and
// each cell is bound as an int64/float64/[]byte/string/NULL rather than as its
// display string. Numbers therefore sort and JOIN numerically, and SQL NULL is
// distinguishable from the literal text "NULL".
//
// When result carries no kinds — e.g. a QueryResult assembled by hand — every
// column falls back to TEXT and display sentinels are reversed heuristically
// (see reverseSentinel), which is lossy for values that are literally "NULL"
// or `""`.
//
// CREATE TABLE, all INSERTs and the provenance record run inside a single
// transaction so a failure partway through (e.g. a bound-parameter overflow)
// leaves neither an orphaned empty table nor a provenance row pointing at a
// table that does not exist — the caller can safely retry with the same name.
func Materialize(ctx context.Context, conn *sql.DB, quote func(string) string, src Source, result db.QueryResult) error {
	cols := disambiguateColumns(result.Columns)
	if len(cols) == 0 {
		return fmt.Errorf("cannot materialize a result with no columns")
	}

	affinities := columnAffinities(result, len(cols))

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := createTable(ctx, tx, quote, src.Table, cols, affinities); err != nil {
		return err
	}
	if len(result.Rows) > 0 {
		if err := insertRows(ctx, tx, quote, src.Table, cols, result); err != nil {
			return err
		}
	}
	if err := recordProvenance(ctx, tx, quote, src, len(result.Rows), len(cols), result.Truncated); err != nil {
		return err
	}
	return tx.Commit()
}

// recordProvenance creates ProvenanceTable if needed and adds one row for this
// bring. truncated matters as much as the row count: a table brought from a
// result that hit the scan limit is a partial view, and forgetting that is
// exactly the kind of mistake this table exists to prevent.
func recordProvenance(ctx context.Context, tx *sql.Tx, quote func(string) string, src Source, rowCount, colCount int, truncated bool) error {
	name := quote(ProvenanceTable)
	ddl := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		n INTEGER PRIMARY KEY,
		table_name TEXT,
		source TEXT,
		row_count INTEGER,
		col_count INTEGER,
		truncated INTEGER,
		query TEXT
	)`, name)
	if _, err := tx.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("create %s: %w", ProvenanceTable, err)
	}

	// OR REPLACE keeps a retry idempotent: a bring that failed after this
	// statement (or a caller that reuses a sequence number) overwrites its own
	// row instead of failing on the primary key. It cannot clobber a different
	// table's record, because n comes from a monotonic per-session counter.
	stmt := fmt.Sprintf(
		`INSERT OR REPLACE INTO %s (n, table_name, source, row_count, col_count, truncated, query)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`, name)
	if _, err := tx.ExecContext(ctx, stmt,
		src.Seq, src.Table, src.Conn, rowCount, colCount, boolToInt(truncated), src.Query,
	); err != nil {
		return fmt.Errorf("record provenance for %s: %w", src.Table, err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// columnAffinities picks a declared SQLite type per column from the kinds
// observed in that column's cells. An empty string means "declare no type",
// which gives the column BLOB affinity: SQLite then stores every value in its
// own storage class instead of coercing it. That is the right choice for a
// column whose values are genuinely mixed — declaring TEXT there would
// stringify the numbers back and undo the whole point of carrying kinds.
func columnAffinities(result db.QueryResult, colCount int) []string {
	out := make([]string, colCount)
	if !result.HasKinds() {
		for i := range out {
			out[i] = "TEXT"
		}
		return out
	}

	for c := range out {
		var seen uint8
		for r, row := range result.Rows {
			var cell string
			if c < len(row) {
				cell = row[c]
			}
			// Classify by the kind the cell will actually be bound as, not by
			// the kind recorded at scan time: a column declared INTEGER coerces
			// even a text-bound value, so an unparsable "integer" must widen the
			// column rather than be silently converted on the way in.
			switch effectiveKind(cell, result.KindAt(r, c)) {
			case db.KindNull:
				// NULL is compatible with every affinity — a column of ints and
				// NULLs is still an INTEGER column — so it never widens the set.
			case db.KindInt:
				seen |= seenInt
			case db.KindFloat:
				seen |= seenFloat
			case db.KindBlob:
				seen |= seenBlob
			default: // KindText, KindEmpty
				seen |= seenText
			}
		}
		switch seen {
		case 0: // no rows, or every value NULL
			out[c] = ""
		case seenInt:
			out[c] = "INTEGER"
		case seenFloat:
			out[c] = "REAL"
		case seenText:
			out[c] = "TEXT"
		case seenBlob:
			out[c] = "BLOB"
		default:
			// Mixed storage classes, including int+float: REAL affinity would
			// be tempting here but it coerces every bound integer to a float,
			// so 9007199254740993 comes back as 9007199254740992. Declaring
			// nothing costs nothing — SQLite still compares integers and reals
			// numerically, so ORDER BY and JOIN behave the same either way.
			out[c] = ""
		}
	}
	return out
}

const (
	seenInt uint8 = 1 << iota
	seenFloat
	seenText
	seenBlob
)

func createTable(ctx context.Context, tx *sql.Tx, quote func(string) string, tableName string, cols, affinities []string) error {
	var sb strings.Builder
	sb.WriteString("CREATE TABLE ")
	sb.WriteString(quote(tableName))
	sb.WriteString(" (")
	for i, c := range cols {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(quote(c))
		if i < len(affinities) && affinities[i] != "" {
			sb.WriteString(" ")
			sb.WriteString(affinities[i])
		}
	}
	sb.WriteString(")")

	if _, err := tx.ExecContext(ctx, sb.String()); err != nil {
		return fmt.Errorf("create table %s: %w", tableName, err)
	}
	return nil
}

func insertRows(ctx context.Context, tx *sql.Tx, quote func(string) string, tableName string, cols []string, result db.QueryResult) error {
	rows := result.Rows
	hasKinds := result.HasKinds()
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
				if hasKinds {
					args = append(args, bindValue(cell, result.KindAt(start+i, c)))
				} else {
					args = append(args, reverseSentinel(cell))
				}
			}
		}
		if _, err := tx.ExecContext(ctx, query.String(), args...); err != nil {
			return fmt.Errorf("insert into %s: %w", tableName, err)
		}
	}

	return nil
}

// effectiveKind reports the kind a cell will actually be bound as. Decoding is
// best-effort: a display string that does not parse as its recorded kind falls
// back to text rather than failing the whole bring, since a surprising cell is
// still worth observing. columnAffinities and bindValue both go through this
// function so a column's declared type always matches the values put into it.
func effectiveKind(cell string, kind db.Kind) db.Kind {
	switch kind {
	case db.KindInt:
		// Unsigned values above math.MaxInt64 land here: SQLite integers are
		// signed 64-bit, so they stay text rather than wrapping negative or
		// being widened to a lossy float.
		if _, err := strconv.ParseInt(cell, 10, 64); err != nil {
			return db.KindText
		}
	case db.KindFloat:
		if _, err := strconv.ParseFloat(cell, 64); err != nil {
			return db.KindText
		}
	case db.KindBlob:
		if _, err := hex.DecodeString(cell); err != nil {
			return db.KindText
		}
	}
	return kind
}

// bindValue turns a display string back into the SQL value it stands for,
// using the kind recorded when the source row was scanned.
func bindValue(cell string, kind db.Kind) any {
	switch effectiveKind(cell, kind) {
	case db.KindNull:
		return nil
	case db.KindEmpty:
		return ""
	case db.KindInt:
		n, _ := strconv.ParseInt(cell, 10, 64)
		return n
	case db.KindFloat:
		f, _ := strconv.ParseFloat(cell, 64)
		return f
	case db.KindBlob:
		b, _ := hex.DecodeString(cell)
		return b
	default:
		return cell
	}
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
