package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}

	conn.SetMaxOpenConns(5)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxLifetime(5 * time.Minute)

	return &Adapter{conn: conn}, nil
}

// readonlyParam is the runtime parameter that carries readonly's second layer
// on PostgreSQL. pgx passes an unknown DSN parameter in the startup packet, so
// the server applies it as every new connection is made and the whole pool is
// covered — not just whichever connection a one-off SET happened to land on.
//
// Measured against PostgreSQL 17 with pgx v5.10.0 (see
// docs/readonly-design.md): five concurrently held connections each read back
// default_transaction_read_only=on, and INSERT, CREATE TABLE, CREATE TEMP TABLE
// and DROP TABLE were all refused with SQLSTATE 25006. The session can lift it,
// both with SET default_transaction_read_only = off and — without any SET — by
// opening an explicit BEGIN READ WRITE. That makes this layer weaker than
// SQLite's mode=ro, which cannot be lifted; refusing those statements is the
// statement guard's job, and the guard stays the layer asql relies on.
const readonlyParam = "default_transaction_read_only"

// OpenReadonly connects with the session's default transaction marked
// read-only where the server supports it.
//
// Whatever is at the far end — a server, or a pooler in front of one — refuses
// the connection outright if it will not take the parameter. Rather than turn a
// working DSN into a connection error, that case falls back to a plain
// connection: layer 2 is belt-and-braces, and the statement guard — which the
// caller wraps around this adapter — is unaffected either way.
func OpenReadonly(dsn string) (*Adapter, error) {
	return openReadonly(dsn, readonlyParam)
}

// openReadonly takes the parameter name so the fallback branch — the one that
// only fires against a server that does not have it — can be exercised by
// naming a parameter no server has.
//
// The fallback fires on any error the far end answered with, not on one chosen
// SQLSTATE. Two different rejections were measured (see
// docs/readonly-design.md): PostgreSQL 17 answers 42704 undefined_object, and
// PgBouncer 1.25.2 in its default configuration answers 08P01 "unsupported
// startup parameter: default_transaction_read_only" — it tracks a fixed set of
// startup parameters and this is not in it. Naming 42704 alone would leave
// every pooled deployment unable to connect under --readonly at all, which is
// the regression the fallback exists to prevent.
//
// It stays narrow in the way that matters: a timeout or a torn connection
// produces no PgError, so the retry never doubles a wait it could not have
// survived anyway. Where the far end did answer, the retry costs one round
// trip and returns that same answer if the DSN was simply wrong.
func openReadonly(dsn, parameter string) (*Adapter, error) {
	roDSN, err := withParam(dsn, parameter, "on")
	if err != nil {
		return nil, err
	}

	adapter, err := Open(roDSN)
	if err == nil {
		return adapter, nil
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil, err
	}
	return Open(dsn)
}

// withParam adds a runtime parameter to a DSN, leaving the caller's own
// parameters in place. db.DetectType only routes postgres:// and postgresql://
// URLs here, so the URL form is the only one to handle.
func withParam(dsn, name, value string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		// Not wrapped: url.Parse's error embeds the raw DSN and this reaches
		// both stderr and the TUI connection-switch display. See db.URLParseError.
		return "", db.URLParseError("PostgreSQL", dsn, err)
	}
	q := u.Query()
	q.Set(name, value)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (a *Adapter) Type() string { return "postgres" }

func (a *Adapter) QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

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

func (a *Adapter) Columns(ctx context.Context, tableName string) ([]string, error) {
	rows, err := a.conn.QueryContext(ctx, `
		SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position`, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
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
		col := fmt.Sprintf("  %s %s", a.QuoteIdentifier(name), dataType)
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

	quoted := a.QuoteIdentifier(tableName)
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

// postgresDialect defines the quoting styles recognized by PostgreSQL.
var postgresDialect = dbutil.Dialect{
	DollarQuote: true,
}

// containsReturning scans query for the RETURNING keyword using the shared scanner.
func containsReturning(query string) bool {
	return dbutil.ContainsReturning(query, postgresDialect)
}
