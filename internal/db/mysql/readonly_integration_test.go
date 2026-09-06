//go:build integration

// Measurements of readonly's connection-level layer against a real MySQL
// server. Run them with:
//
//	docker compose -f testdata/compose.yaml up -d --wait
//	ASQL_TEST_MYSQL_DSN='mysql://asql:asql@127.0.0.1:13306/asql' \
//	  go test -tags integration ./internal/db/...
//
// These deliberately stop at OpenReadonly and never wrap the adapter in
// internal/db/readonly. The statement guard refuses INSERT, CREATE TABLE and
// SET alike, so a test behind it would pass on every assertion here while
// measuring layer 1 twice. Writes also go through plain autocommit statements
// with no explicit BEGIN, because that is the only path asql uses.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	gomysql "github.com/go-sql-driver/mysql"
)

func mysqlDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("ASQL_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("ASQL_TEST_MYSQL_DSN not set; start testdata/compose.yaml to run this")
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
	t.Cleanup(func() {
		cleanup, err := Open(dsn)
		if err != nil {
			return
		}
		defer cleanup.Close()
		_, _ = cleanup.conn.ExecContext(context.Background(), "DROP TABLE IF EXISTS "+name)
	})
}

// The claim behind the DSN-parameter approach is that it covers the pool, not
// one connection. Holding MaxOpenConns handles at once forces the pool to open
// that many physical connections; each one is then asked what it thinks the
// variable is before being asked to write. The read-back is the part that
// matters: a parameter the server silently ignored looks exactly like a working
// one if the only evidence is that the write failed.
func TestReadonlyCoversEveryPooledConnection(t *testing.T) {
	dsn := mysqlDSN(t)
	seedTable(t, dsn, "asql_ro_pool")

	adapter, err := OpenReadonly(dsn)
	if err != nil {
		t.Fatalf("OpenReadonly: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	const want = 5 // matches SetMaxOpenConns in openConfig

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
		if err := c.QueryRowContext(ctx, "SELECT @@session.transaction_read_only").Scan(&v); err != nil {
			t.Fatalf("conn %d: reading transaction_read_only: %v", i, err)
		}
		if v != "1" {
			t.Errorf("conn %d: @@session.transaction_read_only = %q, want %q", i, v, "1")
		}
		if _, err := c.ExecContext(ctx, "INSERT INTO asql_ro_pool VALUES (1)"); err == nil {
			t.Errorf("conn %d: INSERT succeeded on a read-only connection", i)
		}
	}
}

// A read-only transaction is not the same promise as SQLite's mode=ro, so DDL
// is measured rather than assumed.
func TestReadonlyRefusesDDL(t *testing.T) {
	dsn := mysqlDSN(t)
	seedTable(t, dsn, "asql_ro_ddl")

	adapter, err := OpenReadonly(dsn)
	if err != nil {
		t.Fatalf("OpenReadonly: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	for _, stmt := range []string{
		"CREATE TABLE asql_ro_ddl_new (a INT)",
		"CREATE TEMPORARY TABLE asql_ro_ddl_tmp (a INT)",
		"DROP TABLE asql_ro_ddl",
	} {
		if _, err := adapter.conn.ExecContext(ctx, stmt); err == nil {
			t.Errorf("%s succeeded on a read-only connection", stmt)
		}
	}
}

// Recorded as it was measured, not as it would be preferable. Unlike SQLite's
// mode=ro, the MySQL session can turn this layer off, so layer 2 here is
// weaker; the statement guard is what refuses the SET. Should a future server
// stop allowing it, this test fails and docs/readonly-design.md needs the
// stronger claim written in.
func TestReadonlyCanBeLiftedBySession(t *testing.T) {
	dsn := mysqlDSN(t)
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

	if _, err := c.ExecContext(ctx, "SET SESSION transaction_read_only=0"); err != nil {
		t.Fatalf("lifting the restriction failed — layer 2 is stronger than documented: %v", err)
	}
	if _, err := c.ExecContext(ctx, "INSERT INTO asql_ro_lift VALUES (1)"); err != nil {
		t.Fatalf("write after lifting failed — layer 2 is stronger than documented: %v", err)
	}
}

// Adding the parameter must not cost the caller theirs, and must not turn a
// working DSN into a connection error. Both driver-level parameters (charset,
// parseTime) and server-level ones (sql_mode) are checked, because
// go-sql-driver routes the two differently.
func TestReadonlyKeepsCallerParameters(t *testing.T) {
	dsn := mysqlDSN(t)
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	dsn += sep + "charset=utf8mb4&parseTime=true&sql_mode=ANSI_QUOTES"

	adapter, err := OpenReadonly(dsn)
	if err != nil {
		t.Fatalf("OpenReadonly with caller parameters: %v", err)
	}
	defer adapter.Close()

	ctx := context.Background()
	for _, tc := range []struct{ query, want string }{
		{"SELECT @@session.character_set_client", "utf8mb4"},
		{"SELECT @@session.sql_mode", "ANSI_QUOTES"},
		{"SELECT @@session.transaction_read_only", "1"},
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
// the driver actually returns for an unknown system variable is worth pinning:
// if the shape changed, the fallback would stop firing and a server without the
// variable would be unable to connect at all.
func TestUnknownSystemVariableFailsWithKnownError(t *testing.T) {
	dsn := mysqlDSN(t)
	cfg, err := buildConfig(dsn)
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["asql_no_such_variable"] = "1"

	_, err = openConfig(cfg)
	if err == nil {
		t.Fatal("connecting with an unknown system variable succeeded")
	}
	var myErr *gomysql.MySQLError
	if !errors.As(err, &myErr) {
		t.Fatalf("error is %T (%v), want *mysql.MySQLError", err, err)
	}
	if myErr.Number != erUnknownSystemVariable {
		t.Errorf("error number = %d, want %d", myErr.Number, erUnknownSystemVariable)
	}
}

// The fallback: a server too old for transaction_read_only refuses the
// connection outright, and --readonly must still connect rather than turn a
// working DSN into an error. Named variable stands in for that server, since
// the containers under test are new enough to have the real one.
func TestReadonlyFallsBackWhenVariableIsUnknown(t *testing.T) {
	dsn := mysqlDSN(t)

	adapter, err := openReadonly(dsn, "asql_no_such_variable")
	if err != nil {
		t.Fatalf("openReadonly did not fall back to a plain connection: %v", err)
	}
	defer adapter.Close()

	var one int
	if err := adapter.conn.QueryRowContext(context.Background(), "SELECT 1").Scan(&one); err != nil {
		t.Fatalf("the fallback connection is not usable: %v", err)
	}
}
