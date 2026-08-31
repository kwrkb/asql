package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kwrkb/asql/internal/db"
)

func TestContainsReturning(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"insert returning", "INSERT INTO t VALUES (1) RETURNING id", true},
		{"update returning", "UPDATE t SET a=1 RETURNING a", true},
		{"delete returning", "DELETE FROM t WHERE id=1 RETURNING *", true},
		{"insert no returning", "INSERT INTO t VALUES (1)", false},
		{"returning in string literal", "VALUES ('returning')", false},
		{"returning as quoted identifier", `INSERT INTO "returning" VALUES (1)`, false},
		{"returning in line comment", "INSERT INTO t VALUES (1) -- RETURNING id", false},
		{"returning in block comment", "INSERT INTO t VALUES (1) /* RETURNING */", false},
		{"partial match in name", "INSERT INTO t_returning_log VALUES (1)", false},
		{"mixed case", "UPDATE t SET a=1 Returning a", true},
		{"returning in escaped double-quoted identifier", `UPDATE t SET name = "a ""returning"" name"`, false},
		{"returning as backtick-quoted identifier", "INSERT INTO `returning` VALUES (1)", false},
		{"returning as bracket-quoted identifier", "INSERT INTO [returning] VALUES (1)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsReturning(tt.query)
			if got != tt.want {
				t.Errorf("containsReturning(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestReturnsRows(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"select", "SELECT 1", true},
		{"pragma", "PRAGMA table_info(t)", true},
		{"with select", "WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"with delete", "WITH cte AS (SELECT 1) DELETE FROM t WHERE id IN (SELECT * FROM cte)", false},
		{"with delete returning", "WITH cte AS (SELECT 1) DELETE FROM t WHERE id IN (SELECT * FROM cte) RETURNING *", true},
		{"with update", "WITH cte AS (SELECT 1) UPDATE t SET a=1", false},
		{"with insert returning", "WITH cte AS (SELECT 1) INSERT INTO t SELECT * FROM cte RETURNING id", true},
		{"explain", "EXPLAIN SELECT 1", true},
		{"values", "VALUES (1, 2)", true},
		{"insert", "INSERT INTO t VALUES (1)", false},
		{"update", "UPDATE t SET a=1", false},
		{"delete", "DELETE FROM t", false},
		{"create", "CREATE TABLE t (id INTEGER)", false},
		{"empty", "", false},
		{"insert returning", "INSERT INTO t VALUES (1) RETURNING id", true},
		{"update returning", "UPDATE t SET a=1 RETURNING a", true},
		{"delete returning", "DELETE FROM t WHERE id=1 RETURNING *", true},
		{"returning in string", "INSERT INTO t VALUES ('returning')", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := returnsRows(tt.query)
			if got != tt.want {
				t.Errorf("returnsRows(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestOpen(t *testing.T) {
	t.Run("memory db opens successfully", func(t *testing.T) {
		a, err := Open(":memory:")
		if err != nil {
			t.Fatalf("Open(':memory:') failed: %v", err)
		}
		defer a.Close()
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		_, err := Open("/nonexistent/path/that/does/not/exist/db.sqlite")
		if err == nil {
			t.Error("expected error for invalid path, got nil")
		}
	})

	// The driver cuts a raw DSN at the first '?'; without escaping, a path
	// like "reports?2024.db" silently opens (and creates) the file "reports".
	t.Run("path containing '?' opens the named file", func(t *testing.T) {
		ctx := context.Background()
		dir := t.TempDir()
		path := filepath.Join(dir, "reports?2024.db")

		a, err := Open(path)
		if err != nil {
			t.Fatalf("Open(%q) failed: %v", path, err)
		}
		if _, err := a.Query(ctx, "CREATE TABLE t (id INTEGER)"); err != nil {
			a.Close()
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		a.Close()

		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %q to exist: %v", path, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "reports")); err == nil {
			t.Errorf("path was cut at '?': stray file %q was created", "reports")
		}

		// Reopening the same path must see the same database.
		a2, err := Open(path)
		if err != nil {
			t.Fatalf("reopening %q failed: %v", path, err)
		}
		defer a2.Close()
		tables, err := a2.Tables(ctx)
		if err != nil {
			t.Fatalf("Tables failed: %v", err)
		}
		if len(tables) != 1 || tables[0] != "t" {
			t.Errorf("Tables = %v, want [t]", tables)
		}
	})
}

func TestTables(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) *Adapter {
		t.Helper()
		a, err := Open(":memory:")
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		t.Cleanup(func() { a.Close() })
		return a
	}

	t.Run("empty database returns no tables", func(t *testing.T) {
		a := setup(t)
		tables, err := a.Tables(ctx)
		if err != nil {
			t.Fatalf("Tables() failed: %v", err)
		}
		if len(tables) != 0 {
			t.Errorf("expected 0 tables, got %d: %v", len(tables), tables)
		}
	})

	t.Run("returns tables sorted by name", func(t *testing.T) {
		a := setup(t)
		for _, ddl := range []string{
			"CREATE TABLE zebra (id INTEGER)",
			"CREATE TABLE alpha (id INTEGER)",
			"CREATE TABLE middle (id INTEGER)",
		} {
			if _, err := a.Query(ctx, ddl); err != nil {
				t.Fatalf("failed: %v", err)
			}
		}

		tables, err := a.Tables(ctx)
		if err != nil {
			t.Fatalf("Tables() failed: %v", err)
		}
		if len(tables) != 3 {
			t.Fatalf("expected 3 tables, got %d", len(tables))
		}
		if tables[0] != "alpha" || tables[1] != "middle" || tables[2] != "zebra" {
			t.Errorf("expected [alpha middle zebra], got %v", tables)
		}
	})

	t.Run("excludes views", func(t *testing.T) {
		a := setup(t)
		if _, err := a.Query(ctx, "CREATE TABLE t (id INTEGER)"); err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		if _, err := a.Query(ctx, "CREATE VIEW v AS SELECT * FROM t"); err != nil {
			t.Fatalf("CREATE VIEW failed: %v", err)
		}

		tables, err := a.Tables(ctx)
		if err != nil {
			t.Fatalf("Tables() failed: %v", err)
		}
		if len(tables) != 1 || tables[0] != "t" {
			t.Errorf("expected [t], got %v", tables)
		}
	})
}

func TestColumns(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) *Adapter {
		t.Helper()
		a, err := Open(":memory:")
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		t.Cleanup(func() { a.Close() })
		return a
	}

	t.Run("returns column names", func(t *testing.T) {
		a := setup(t)
		a.conn.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT)")

		cols, err := a.Columns(ctx, "users")
		if err != nil {
			t.Fatalf("Columns() failed: %v", err)
		}
		if len(cols) != 3 {
			t.Fatalf("expected 3 columns, got %d: %v", len(cols), cols)
		}
		if cols[0] != "id" || cols[1] != "name" || cols[2] != "email" {
			t.Errorf("expected [id name email], got %v", cols)
		}
	})

	t.Run("nonexistent table returns empty", func(t *testing.T) {
		a := setup(t)
		cols, err := a.Columns(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("Columns() failed: %v", err)
		}
		if len(cols) != 0 {
			t.Errorf("expected 0 columns, got %d: %v", len(cols), cols)
		}
	})
}

func TestSchema(t *testing.T) {
	ctx := context.Background()

	t.Run("empty database returns empty string", func(t *testing.T) {
		a, err := Open(":memory:")
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer a.Close()

		schema, err := a.Schema(ctx)
		if err != nil {
			t.Fatalf("Schema() failed: %v", err)
		}
		if schema != "" {
			t.Errorf("expected empty schema, got %q", schema)
		}
	})

	t.Run("returns CREATE TABLE statements", func(t *testing.T) {
		a, err := Open(":memory:")
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		defer a.Close()

		a.conn.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
		a.conn.ExecContext(ctx, "CREATE TABLE posts (id INTEGER, user_id INTEGER, body TEXT)")

		schema, err := a.Schema(ctx)
		if err != nil {
			t.Fatalf("Schema() failed: %v", err)
		}
		if !strings.Contains(schema, "CREATE TABLE posts") {
			t.Errorf("schema missing posts table: %q", schema)
		}
		if !strings.Contains(schema, "CREATE TABLE users") {
			t.Errorf("schema missing users table: %q", schema)
		}
		// Sorted by name: posts before users
		postsIdx := strings.Index(schema, "posts")
		usersIdx := strings.Index(schema, "users")
		if postsIdx > usersIdx {
			t.Error("expected tables sorted by name (posts before users)")
		}
	})
}

func TestQuery(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) *Adapter {
		t.Helper()
		a, err := Open(":memory:")
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		t.Cleanup(func() { a.Close() })
		return a
	}

	t.Run("SELECT returns columns and rows", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE users (id INTEGER, name TEXT)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		_, err = a.Query(ctx, "INSERT INTO users VALUES (1, 'alice'), (2, 'bob')")
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}

		result, err := a.Query(ctx, "SELECT id, name FROM users ORDER BY id")
		if err != nil {
			t.Fatalf("SELECT failed: %v", err)
		}
		if len(result.Columns) != 2 {
			t.Errorf("expected 2 columns, got %d", len(result.Columns))
		}
		if len(result.Rows) != 2 {
			t.Errorf("expected 2 rows, got %d", len(result.Rows))
		}
		if result.Rows[0][0] != "1" || result.Rows[0][1] != "alice" {
			t.Errorf("unexpected first row: %v", result.Rows[0])
		}
		if !strings.Contains(result.Message, "2 row(s) returned") {
			t.Errorf("unexpected message: %q", result.Message)
		}
	})

	t.Run("INSERT returns rows affected message", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE t (v INTEGER)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}

		result, err := a.Query(ctx, "INSERT INTO t VALUES (1), (2), (3)")
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}
		if !strings.Contains(result.Message, "3 row(s) affected") {
			t.Errorf("unexpected message: %q", result.Message)
		}
		if len(result.Columns) != 0 {
			t.Errorf("expected no columns for DML, got %d", len(result.Columns))
		}
	})

	t.Run("empty query returns error", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "")
		if err == nil {
			t.Error("expected error for empty query, got nil")
		}
	})

	t.Run("invalid SQL returns error", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "NOT VALID SQL")
		if err == nil {
			t.Error("expected error for invalid SQL, got nil")
		}
	})

	t.Run("SELECT with no rows returns empty rows", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE empty_t (id INTEGER)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}

		result, err := a.Query(ctx, "SELECT * FROM empty_t")
		if err != nil {
			t.Fatalf("SELECT failed: %v", err)
		}
		if len(result.Rows) != 0 {
			t.Errorf("expected 0 rows, got %d", len(result.Rows))
		}
		if len(result.Columns) != 1 {
			t.Errorf("expected 1 column, got %d", len(result.Columns))
		}
	})

	t.Run("INSERT RETURNING returns columns and rows", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}

		result, err := a.Query(ctx, "INSERT INTO t VALUES (1, 'hello') RETURNING id, v")
		if err != nil {
			t.Fatalf("INSERT RETURNING failed: %v", err)
		}
		if len(result.Columns) != 2 {
			t.Errorf("expected 2 columns, got %d", len(result.Columns))
		}
		if len(result.Rows) != 1 {
			t.Errorf("expected 1 row, got %d", len(result.Rows))
		}
		if result.Rows[0][0] != "1" || result.Rows[0][1] != "hello" {
			t.Errorf("unexpected row: %v", result.Rows[0])
		}
		if !strings.Contains(result.Message, "1 row(s) returned") {
			t.Errorf("unexpected message: %q", result.Message)
		}
	})

	t.Run("UPDATE RETURNING returns updated values", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE t (id INTEGER, v INTEGER)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		_, err = a.Query(ctx, "INSERT INTO t VALUES (1, 10)")
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}

		result, err := a.Query(ctx, "UPDATE t SET v=20 WHERE id=1 RETURNING v")
		if err != nil {
			t.Fatalf("UPDATE RETURNING failed: %v", err)
		}
		if len(result.Rows) != 1 || result.Rows[0][0] != "20" {
			t.Errorf("unexpected rows: %v", result.Rows)
		}
	})

	t.Run("DELETE RETURNING returns deleted rows", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE t (id INTEGER, v TEXT)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		_, err = a.Query(ctx, "INSERT INTO t VALUES (1, 'bye')")
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}

		result, err := a.Query(ctx, "DELETE FROM t WHERE id=1 RETURNING id, v")
		if err != nil {
			t.Fatalf("DELETE RETURNING failed: %v", err)
		}
		if len(result.Rows) != 1 || result.Rows[0][0] != "1" || result.Rows[0][1] != "bye" {
			t.Errorf("unexpected rows: %v", result.Rows)
		}
	})

	t.Run("BLOB column displays as hex", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE t (data BLOB)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		_, err = a.Query(ctx, "INSERT INTO t VALUES (X'DEADBEEF')")
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}

		result, err := a.Query(ctx, "SELECT data FROM t")
		if err != nil {
			t.Fatalf("SELECT failed: %v", err)
		}
		if len(result.Rows) != 1 || result.Rows[0][0] != "deadbeef" {
			t.Errorf("expected hex 'deadbeef', got %v", result.Rows)
		}
	})

	t.Run("whitespace-only query returns error", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "   ")
		if err == nil {
			t.Error("expected error for whitespace-only query, got nil")
		}
	})

	t.Run("NULL value SELECT returns NULL string", func(t *testing.T) {
		a := setup(t)
		result, err := a.Query(ctx, "SELECT NULL")
		if err != nil {
			t.Fatalf("SELECT NULL failed: %v", err)
		}
		if len(result.Rows) != 1 || result.Rows[0][0] != "NULL" {
			t.Errorf("expected 'NULL', got %v", result.Rows)
		}
	})

	t.Run("empty string is displayed as double-quoted empty", func(t *testing.T) {
		a := setup(t)
		result, err := a.Query(ctx, "SELECT '', NULL, 0")
		if err != nil {
			t.Fatalf("SELECT failed: %v", err)
		}
		if len(result.Rows) != 1 {
			t.Fatalf("expected 1 row, got %d", len(result.Rows))
		}
		row := result.Rows[0]
		if row[0] != `""` {
			t.Errorf("empty string: expected %q, got %q", `""`, row[0])
		}
		if row[1] != "NULL" {
			t.Errorf("NULL: expected %q, got %q", "NULL", row[1])
		}
		if row[2] != "0" {
			t.Errorf("zero: expected %q, got %q", "0", row[2])
		}
	})

	t.Run("ColumnTypes are populated for typed columns", func(t *testing.T) {
		a := setup(t)
		_, err := a.Query(ctx, "CREATE TABLE typed (id INTEGER, name TEXT, val REAL)")
		if err != nil {
			t.Fatalf("CREATE TABLE failed: %v", err)
		}
		_, err = a.Query(ctx, "INSERT INTO typed VALUES (1, 'a', 1.5)")
		if err != nil {
			t.Fatalf("INSERT failed: %v", err)
		}
		result, err := a.Query(ctx, "SELECT id, name, val FROM typed")
		if err != nil {
			t.Fatalf("SELECT failed: %v", err)
		}
		if len(result.ColumnTypes) != 3 {
			t.Fatalf("expected 3 column types, got %d", len(result.ColumnTypes))
		}
		// SQLite driver returns "INTEGER", "TEXT", "REAL"
		if result.ColumnTypes[0] != "INTEGER" {
			t.Errorf("expected INTEGER, got %q", result.ColumnTypes[0])
		}
		if result.ColumnTypes[1] != "TEXT" {
			t.Errorf("expected TEXT, got %q", result.ColumnTypes[1])
		}
		if result.ColumnTypes[2] != "REAL" {
			t.Errorf("expected REAL, got %q", result.ColumnTypes[2])
		}
	})
}

func TestQueryKinds(t *testing.T) {
	ctx := context.Background()
	a, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer a.Close()

	if _, err := a.conn.ExecContext(ctx, `CREATE TABLE k (i INTEGER, f REAL, s TEXT, b BLOB)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := a.conn.ExecContext(ctx,
		`INSERT INTO k VALUES (1, 1.5, 'a', ?), (NULL, NULL, '', ?)`,
		[]byte{0xff, 0xfe}, nil); err != nil {
		t.Fatalf("insert: %v", err)
	}

	res, err := a.Query(ctx, `SELECT i, f, s, b FROM k ORDER BY rowid`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if !res.HasKinds() {
		t.Fatal("expected Kinds to be populated")
	}
	if len(res.Kinds) != len(res.Rows) {
		t.Fatalf("Kinds has %d rows, Rows has %d", len(res.Kinds), len(res.Rows))
	}
	for r, row := range res.Rows {
		if len(res.Kinds[r]) != len(row) {
			t.Fatalf("row %d: %d kinds for %d cells", r, len(res.Kinds[r]), len(row))
		}
	}

	want := [][]db.Kind{
		{db.KindInt, db.KindFloat, db.KindText, db.KindBlob},
		{db.KindNull, db.KindNull, db.KindEmpty, db.KindNull},
	}
	for r := range want {
		for c := range want[r] {
			if got := res.KindAt(r, c); got != want[r][c] {
				t.Errorf("KindAt(%d,%d) = %d, want %d (cell %q)", r, c, got, want[r][c], res.Rows[r][c])
			}
		}
	}

	// The display contract must be unchanged: NULL renders as NULL, an empty
	// string renders as the `""` sentinel.
	if res.Rows[1][0] != "NULL" {
		t.Errorf("NULL cell rendered as %q, want NULL", res.Rows[1][0])
	}
	if res.Rows[1][2] != `""` {
		t.Errorf("empty cell rendered as %q, want the \"\" sentinel", res.Rows[1][2])
	}

	// Out-of-range access degrades to text rather than panicking.
	if res.KindAt(99, 0) != db.KindText || res.KindAt(0, 99) != db.KindText {
		t.Error("out-of-range KindAt should return KindText")
	}
}

func TestQueryKinds_MessageOnlyResultHasNoKinds(t *testing.T) {
	ctx := context.Background()
	a, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer a.Close()

	res, err := a.Query(ctx, `CREATE TABLE x (v TEXT)`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.HasKinds() {
		t.Errorf("message-only result should carry no kinds, got %v", res.Kinds)
	}

	res, err = a.Query(ctx, `SELECT v FROM x`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if res.HasKinds() {
		t.Errorf("zero-row result should carry no kinds, got %v", res.Kinds)
	}
}

func TestReadonlyURI(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"relative path", "chinook.db", "file:chinook.db?mode=ro"},
		{"absolute path", "/data/chinook.db", "file:/data/chinook.db?mode=ro"},
		{"path with question mark", "odd?name.db", "file:odd%3Fname.db?mode=ro"},
		{"path with hash", "odd#name.db", "file:odd%23name.db?mode=ro"},
		{"path with percent", "odd%name.db", "file:odd%25name.db?mode=ro"},
		{"already a uri", "file:chinook.db", "file:chinook.db?mode=ro"},
		{"uri with other params", "file:chinook.db?_pragma=busy_timeout(10)", "file:chinook.db?_pragma=busy_timeout%2810%29&mode=ro"},
		{"uri already read-write", "file:chinook.db?mode=rw", "file:chinook.db?mode=ro"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readonlyURI(tt.path); got != tt.want {
				t.Errorf("readonlyURI(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// The connection-level layer has to hold on its own, and it has to hold
// against a session that tries to lift it: query_only can be switched off from
// inside the session, mode=ro cannot.
func TestOpenReadonlyRefusesWrites(t *testing.T) {
	path := t.TempDir() + "/data.db"

	writable, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ctx := context.Background()
	if _, err := writable.Query(ctx, "CREATE TABLE t (a TEXT)"); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	if _, err := writable.Query(ctx, "INSERT INTO t VALUES ('x')"); err != nil {
		t.Fatalf("INSERT: %v", err)
	}
	if err := writable.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ro, err := OpenReadonly(path)
	if err != nil {
		t.Fatalf("OpenReadonly: %v", err)
	}
	defer ro.Close()

	result, err := ro.Query(ctx, "SELECT a FROM t")
	if err != nil {
		t.Fatalf("SELECT on a read-only connection: %v", err)
	}
	if len(result.Rows) != 1 || result.Rows[0][0] != "x" {
		t.Fatalf("SELECT returned %v, want one row holding x", result.Rows)
	}

	if _, err := ro.Query(ctx, "INSERT INTO t VALUES ('y')"); err == nil {
		t.Fatal("INSERT succeeded on a read-only connection")
	}

	// PRAGMA query_only(0) is accepted by SQLite in its function form; what
	// matters is that it does not hand the session write access back.
	_, _ = ro.Query(ctx, "PRAGMA query_only(0)")
	if _, err := ro.Query(ctx, "CREATE TABLE b (v TEXT)"); err == nil {
		t.Fatal("CREATE TABLE succeeded after PRAGMA query_only(0) — mode=ro was lifted")
	}
}
