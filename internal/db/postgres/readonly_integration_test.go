//go:build integration

// Measurements of readonly's connection-level layer against a real PostgreSQL
// server. Run them with:
//
//	docker compose -f testdata/compose.yaml up -d --wait
//	ASQL_TEST_POSTGRES_DSN='postgres://asql:asql@127.0.0.1:15432/asql?sslmode=disable' \
//	  go test -tags integration ./internal/db/...
//
// These deliberately stop at OpenReadonly and never wrap the adapter in
// internal/db/readonly. The statement guard refuses INSERT, CREATE TABLE and
// SET alike, so a test behind it would pass on every assertion here while
// measuring layer 1 twice. Writes also go through plain autocommit statements
// with no explicit BEGIN, because that is the only path asql uses.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// addParam appends a query parameter to a DSN that may or may not already
// carry one.
func addParam(dsn, param string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + param
	}
	return dsn + "?" + param
}

func postgresDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("ASQL_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("ASQL_TEST_POSTGRES_DSN not set; start testdata/compose.yaml to run this")
	}
	return dsn
}

// seedTable creates a table through a writable connection and registers its
// cleanup, so the readonly assertions fail on being refused rather than on the
// target not existing.
func seedTable(t *testing.T, dsn, name string) {
	t.Helper()
	writable, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open (writable seed): %v", err)
	}
	defer writable.Close()

	ctx := context.Background()
	if _, err := writable.conn.ExecContext(ctx, "DROP TABLE IF EXISTS "+name); err != nil {
		t.Fatalf("drop seed table: %v", err)
	}
	if _, err := writable.conn.ExecContext(ctx, "CREATE TABLE "+name+" (a INT)"); err != nil {
		t.Fatalf("create seed table: %v", err)
	}
	dropOnCleanup(t, dsn, name)
}

// dropOnCleanup registers a drop for a table the test must not leave behind.
// A table a readonly assertion expects never to be created still needs one:
// were layer 2 to regress, the CREATE would go through, and every later run
// would then fail on "already exists" — an error, so a test asserting only
// that an error occurred would stay green right over the regression.
func dropOnCleanup(t *testing.T, dsn, name string) {
	t.Helper()
	t.Cleanup(func() {
		cleanup, err := Open(dsn)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.conn.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+name)
	})
}

// readOnlySQLTransaction is what PostgreSQL answers a write with while
// default_transaction_read_only is on (measured against postgres:17: SQLSTATE
// 25006, "cannot execute CREATE TABLE / DROP TABLE / INSERT in a read-only
// transaction"). The assertions name it rather than accepting any error, so a
// statement that fails for an unrelated reason cannot stand in for layer 2
// having held.
const readOnlySQLTransaction = "25006"

// refusedAsReadOnly fails the test unless err is that error.
func refusedAsReadOnly(t *testing.T, stmt string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s succeeded on a read-only connection", stmt)
		return
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != readOnlySQLTransaction {
		t.Errorf("%s failed with %v, want SQLSTATE %s (read-only transaction)",
			stmt, err, readOnlySQLTransaction)
	}
}

// The claim behind the DSN-parameter approach is that it covers the pool, not
// one connection. Holding MaxOpenConns handles at once forces the pool to open
// that many physical connections; each one is then asked what it thinks the
// parameter is before being asked to write. The read-back is the part that
// matters: a parameter the server silently ignored looks exactly like a working
// one if the only evidence is that the write failed.
func TestReadonlyCoversEveryPooledConnection(t *testing.T) {
	dsn := postgresDSN(t)
	seedTable(t, dsn, "asql_ro_pool")

	adapter, err := OpenReadonly(dsn)
	if err != nil {
		t.Fatalf("OpenReadonly: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	const want = 5 // matches SetMaxOpenConns in Open

	var held []*sql.Conn
	defer func() {
		for _, c := range held {
			c.Close()
		}
	}()
	for i := 0; i < want; i++ {
		c, err := adapter.conn.Conn(ctx)
		if err != nil {
			t.Fatalf("conn %d: %v", i, err)
		}
		held = append(held, c)
	}

	for i, c := range held {
		var v string
		if err := c.QueryRowContext(ctx, "SHOW default_transaction_read_only").Scan(&v); err != nil {
			t.Fatalf("conn %d: reading default_transaction_read_only: %v", i, err)
		}
		if v != "on" {
			t.Errorf("conn %d: default_transaction_read_only = %q, want %q", i, v, "on")
		}
		if _, err := c.ExecContext(ctx, "INSERT INTO asql_ro_pool VALUES (1)"); err == nil {
			t.Errorf("conn %d: INSERT succeeded on a read-only connection", i)
		}
	}
}

// A read-only transaction is not the same promise as SQLite's mode=ro, so DDL
// is measured rather than assumed — temporary tables separately, since those
// are the ones a read-only transaction could plausibly still allow.
func TestReadonlyRefusesDDL(t *testing.T) {
	dsn := postgresDSN(t)
	seedTable(t, dsn, "asql_ro_ddl")
	dropOnCleanup(t, dsn, "asql_ro_ddl_new")

	adapter, err := OpenReadonly(dsn)
	if err != nil {
		t.Fatalf("OpenReadonly: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	for _, stmt := range []string{
		"CREATE TABLE asql_ro_ddl_new (a INT)",
		"CREATE TEMP TABLE asql_ro_ddl_tmp (a INT)",
		"DROP TABLE asql_ro_ddl",
	} {
		_, err := adapter.conn.ExecContext(ctx, stmt)
		refusedAsReadOnly(t, stmt, err)
	}
}

// Recorded as it was measured, not as it would be preferable. Unlike SQLite's
// mode=ro, the PostgreSQL session can turn this layer off — with SET, and also
// with an explicit read-write transaction that needs no SET at all. The
// statement guard is what refuses both. Should a future server stop allowing
// them, this test fails and docs/readonly-design.md needs the stronger claim
// written in.
func TestReadonlyCanBeLiftedBySession(t *testing.T) {
	dsn := postgresDSN(t)
	seedTable(t, dsn, "asql_ro_lift")

	adapter, err := OpenReadonly(dsn)
	if err != nil {
		t.Fatalf("OpenReadonly: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()

	c, err := adapter.conn.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer c.Close()
	if _, err := c.ExecContext(ctx, "SET default_transaction_read_only = off"); err != nil {
		t.Fatalf("lifting the restriction failed — layer 2 is stronger than documented: %v", err)
	}
	if _, err := c.ExecContext(ctx, "INSERT INTO asql_ro_lift VALUES (1)"); err != nil {
		t.Fatalf("write after lifting failed — layer 2 is stronger than documented: %v", err)
	}

	// The second path: an explicit read-write transaction overrides the
	// session default without touching it.
	fresh, err := adapter.conn.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	defer fresh.Close()
	if _, err := fresh.ExecContext(ctx, "BEGIN READ WRITE; INSERT INTO asql_ro_lift VALUES (2); COMMIT"); err != nil {
		t.Fatalf("BEGIN READ WRITE write failed — layer 2 is stronger than documented: %v", err)
	}
}

// Adding the parameter must not cost the caller theirs, and must not turn a
// working DSN into a connection error.
func TestReadonlyKeepsCallerParameters(t *testing.T) {
	dsn := postgresDSN(t)
	withParam, err := withParam(addParam(dsn, "application_name=asql-test"), readonlyParam, "on")
	if err != nil {
		t.Fatalf("withReadonlyParam: %v", err)
	}

	adapter, err := Open(withParam)
	if err != nil {
		t.Fatalf("Open with caller parameters: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	for _, tc := range []struct{ query, want string }{
		{"SHOW application_name", "asql-test"},
		{"SHOW default_transaction_read_only", "on"},
	} {
		var got string
		if err := adapter.conn.QueryRowContext(ctx, tc.query).Scan(&got); err != nil {
			t.Fatalf("%s: %v", tc.query, err)
		}
		if got != tc.want {
			t.Errorf("%s = %q, want %q", tc.query, got, tc.want)
		}
	}
}

// OpenReadonly falls back to a plain connection on one specific error, so what
// the server actually returns for an unrecognised runtime parameter is worth
// pinning: if the shape changed, the fallback would stop firing and a server
// without the parameter would be unable to connect at all.
func TestUnknownRuntimeParameterFailsWithKnownError(t *testing.T) {
	dsn := postgresDSN(t)
	_, err := Open(addParam(dsn, "asql_no_such_parameter=1"))
	if err == nil {
		t.Fatal("connecting with an unrecognised runtime parameter succeeded")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error is %T (%v), want *pgconn.PgError", err, err)
	}
	if pgErr.Code != undefinedObject {
		t.Errorf("error code = %q, want %q", pgErr.Code, undefinedObject)
	}
}

// The fallback: a server that does not recognise default_transaction_read_only
// refuses the connection outright, and --readonly must still connect rather
// than turn a working DSN into an error. The named parameter stands in for that
// server, since the containers under test do recognise the real one.
func TestReadonlyFallsBackWhenParameterIsUnknown(t *testing.T) {
	dsn := postgresDSN(t)

	adapter, err := openReadonly(dsn, "asql_no_such_parameter")
	if err != nil {
		t.Fatalf("openReadonly did not fall back to a plain connection: %v", err)
	}
	defer adapter.Close()

	var one int
	if err := adapter.conn.QueryRowContext(context.Background(), "SELECT 1").Scan(&one); err != nil {
		t.Fatalf("the fallback connection is not usable: %v", err)
	}
}
