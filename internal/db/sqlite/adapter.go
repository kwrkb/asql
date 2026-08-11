package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

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
	return newAdapter(conn)
}

// NewAdapter wraps an already-open *sql.DB as a sqlite Adapter. Used when the
// caller needs to retain the *sql.DB itself (e.g. to share the pool with code
// that writes to the database outside of Adapter.Query, such as the Bring &
// Join local database).
func NewAdapter(conn *sql.DB) (*Adapter, error) {
	return newAdapter(conn)
}

func newAdapter(conn *sql.DB) (*Adapter, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}

	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	conn.SetConnMaxLifetime(5 * time.Minute)

	return &Adapter{conn: conn}, nil
}

func (a *Adapter) Type() string { return "sqlite" }

func (a *Adapter) QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

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

func (a *Adapter) Columns(ctx context.Context, tableName string) ([]string, error) {
	quoted := a.QuoteIdentifier(tableName)
	rows, err := a.conn.QueryContext(ctx, "PRAGMA table_info("+quoted+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue *string
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
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

	// modernc.org/sqlite returns []byte only for the BLOB storage class, so the
	// Go type identifies a blob exactly — including for expressions, which
	// carry no declared column type at all.
	return dbutil.ScanRowsOpts(rows, dbutil.ScanOptions{
		Limit:          dbutil.DefaultRowLimit,
		BytesAreBinary: true,
	})
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
	case "select", "pragma", "explain", "values":
		return true
	case "with":
		body := dbutil.CteBodyKeyword(query)
		switch body {
		case "select", "values", "pragma", "explain":
			return true
		default:
			return containsReturning(query)
		}
	default:
		return containsReturning(query)
	}
}

// sqliteDialect defines the quoting styles recognized by SQLite.
var sqliteDialect = dbutil.Dialect{
	BracketQuote:  true,
	BacktickQuote: true,
}

// containsReturning scans query for the RETURNING keyword using the shared scanner.
func containsReturning(query string) bool {
	return dbutil.ContainsReturning(query, sqliteDialect)
}

// OpenReadonly opens the database at path with SQLite's read-only URI mode.
//
// This is the connection-level half of readonly and the only one asql has
// verified: with mode=ro the driver refuses writes, and — unlike the
// query_only pragma — the session cannot lift the restriction on itself
// (`PRAGMA query_only(0)` succeeds but writes still fail). ATTACH does succeed
// under mode=ro, so the statement guard has to refuse it separately.
func OpenReadonly(path string) (*Adapter, error) {
	return Open(readonlyURI(path))
}

// readonlyURI turns a SQLite path into a file: URI carrying mode=ro. A path
// that is already a URI keeps its other parameters and has mode overridden;
// anything else is escaped, since a bare path may contain the characters that
// delimit a URI.
func readonlyURI(path string) string {
	if strings.HasPrefix(path, "file:") {
		if u, err := url.Parse(path); err == nil {
			q := u.Query()
			q.Set("mode", "ro")
			u.RawQuery = q.Encode()
			return u.String()
		}
	}
	return "file:" + escapeURIPath(path) + "?mode=ro"
}

// escapeURIPath percent-encodes the characters that would otherwise be read as
// URI delimiters. Everything else is left alone so the path stays readable in
// error messages.
func escapeURIPath(path string) string {
	var b strings.Builder
	for i := 0; i < len(path); i++ {
		switch c := path[i]; c {
		case '?', '#', '%':
			fmt.Fprintf(&b, "%%%02X", c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
