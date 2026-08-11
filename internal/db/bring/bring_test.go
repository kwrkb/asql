package bring

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/kwrkb/asql/internal/db"
)

func TestMaterialize_NullSentinel(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "NULL"}},
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT id FROM t1 WHERE name IS NULL`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "1" {
		t.Fatalf("expected NULL sentinel to round-trip as real NULL, got %+v", got.Rows)
	}
}

func TestMaterialize_EmptyStringSentinel(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", `""`}},
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT id FROM t1 WHERE name = ''`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "1" {
		t.Fatalf(`expected "" sentinel to round-trip as empty string, got %+v`, got.Rows)
	}
}

func TestMaterialize_DuplicateColumnNames(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// The 3rd column is already named "id_2", which is the name the naive
	// suffix scheme would generate for the 2nd (duplicate) "id" column. The
	// disambiguator must not let the generated name steal the real id_2
	// column's identity.
	result := db.QueryResult{
		Columns: []string{"id", "id", "id_2"},
		Rows:    [][]string{{"a", "b", "c"}},
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT id, id_2 FROM t1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "a" || got.Rows[0][1] != "c" {
		t.Fatalf("expected original id/id_2 columns to round-trip untouched, got %+v", got.Rows)
	}

	// The renamed duplicate must still be reachable under some unique name.
	cols, err := adapter.Columns(context.Background(), "t1")
	if err != nil {
		t.Fatalf("Columns: %v", err)
	}
	if len(cols) != 3 {
		t.Fatalf("expected 3 columns, got %v", cols)
	}
}

func TestMaterialize_CaseInsensitiveDuplicateColumnNames(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// SQLite treats column names case-insensitively, so "ID" and "id" are
	// duplicates even though they differ by case.
	result := db.QueryResult{
		Columns: []string{"ID", "id"},
		Rows:    [][]string{{"1", "2"}},
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	cols, err := adapter.Columns(context.Background(), "t1")
	if err != nil {
		t.Fatalf("Columns: %v", err)
	}
	if len(cols) != 2 {
		t.Fatalf("expected 2 distinct columns, got %v", cols)
	}
}

func TestMaterialize_ReservedWordNames(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := db.QueryResult{
		Columns: []string{"select", "group"},
		Rows:    [][]string{{"1", "2"}},
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "order"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT "select", "group" FROM "order"`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "1" || got.Rows[0][1] != "2" {
		t.Fatalf("expected reserved-word identifiers to round-trip, got %+v", got.Rows)
	}
}

func TestMaterialize_Join(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	left := db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "alice"}, {"2", "bob"}},
	}
	right := db.QueryResult{
		Columns: []string{"id", "score"},
		Rows:    [][]string{{"1", "90"}, {"3", "70"}},
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, left); err != nil {
		t.Fatalf("Materialize left: %v", err)
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 2, Table: "t2"}, right); err != nil {
		t.Fatalf("Materialize right: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT t1.name, t2.score FROM t1 JOIN t2 ON t1.id = t2.id`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "alice" || got.Rows[0][1] != "90" {
		t.Fatalf("expected JOIN to match id=1 row only, got %+v", got.Rows)
	}
}

func TestMaterialize_EmptyResult(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := db.QueryResult{
		Columns: []string{"id", "name"},
		Rows:    [][]string{},
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT * FROM t1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Rows) != 0 {
		t.Fatalf("expected empty table, got %+v", got.Rows)
	}
}

func TestMaterialize_NoColumnsErrors(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := db.QueryResult{Columns: nil, Rows: nil}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err == nil {
		t.Fatal("expected an error for a result with no columns, not a CREATE TABLE with an empty column list")
	}
}

func TestMaterialize_TruncatedResult(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := db.QueryResult{
		Columns:   []string{"id"},
		Rows:      [][]string{{"1"}, {"2"}},
		Truncated: true,
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT COUNT(*) FROM t1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "2" {
		t.Fatalf("expected the fetched rows to materialize despite Truncated, got %+v", got.Rows)
	}
}

func TestMaterialize_BatchedInserts(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	rows := make([][]string, 0, maxBatchRows*2+10)
	for i := 0; i < maxBatchRows*2+10; i++ {
		rows = append(rows, []string{"x"})
	}
	result := db.QueryResult{Columns: []string{"id"}, Rows: rows}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT COUNT(*) FROM t1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := len(rows)
	if len(got.Rows) != 1 {
		t.Fatalf("expected one count row, got %+v", got.Rows)
	}
	count, err := strconv.Atoi(got.Rows[0][0])
	if err != nil {
		t.Fatalf("parse count: %v", err)
	}
	if count != want {
		t.Fatalf("expected %d rows across batches, got %d", want, count)
	}
}

func TestMaterialize_WideResultStaysUnderBoundParamLimit(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	const numCols = 70
	const numRows = 600 // well within dbutil.DefaultRowLimit (10,000)
	cols := make([]string, numCols)
	for i := range cols {
		cols[i] = fmt.Sprintf("c%d", i)
	}
	rows := make([][]string, numRows)
	for r := range rows {
		row := make([]string, numCols)
		for c := range row {
			row[c] = "x"
		}
		rows[r] = row
	}
	result := db.QueryResult{Columns: cols, Rows: rows}

	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "wide"}, result); err != nil {
		t.Fatalf("Materialize failed for a %d-col x %d-row result: %v", numCols, numRows, err)
	}

	got, err := adapter.Query(context.Background(), `SELECT COUNT(*) FROM wide`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	count, err := strconv.Atoi(got.Rows[0][0])
	if err != nil {
		t.Fatalf("parse count: %v", err)
	}
	if count != numRows {
		t.Fatalf("expected %d rows, got %d", numRows, count)
	}
}

func TestMaterialize_FailureLeavesNoOrphanedTable(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// A row with more cells than columns causes insertRows to bind past the
	// declared column count for that row's extra cell... instead, force a
	// failure via a context that's already expired, which fails inside the
	// transaction after CREATE TABLE would otherwise have run.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
	}
	if err := Materialize(ctx, conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err == nil {
		t.Fatal("expected Materialize to fail with an already-cancelled context")
	}

	// Retrying with the same table name and a valid context must succeed —
	// the failed attempt must not have left behind a partially-created
	// table that collides with the retry.
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("expected retry with the same table name to succeed after a failed attempt, got: %v", err)
	}
}

func TestOpen_ConnectionSurvivesBeyondDefaultLifetime(t *testing.T) {
	// sqlite.NewAdapter (used by Open) applies a 5-minute connection
	// lifetime intended for file-based databases. For this private
	// ":memory:" database, database/sql closing the connection to
	// "refresh" it would destroy all brought tables. Open must disable
	// the lifetime so the connection — and the data in it — survives.
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}},
	}); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	stats := conn.Stats()
	if stats.MaxLifetimeClosed != 0 {
		t.Fatalf("expected no connections closed due to lifetime expiry, got %d", stats.MaxLifetimeClosed)
	}
}

// --- typed bring (QueryResult.Kinds) ---

// typedResult builds a QueryResult whose Kinds are set explicitly, the way
// dbutil.ScanRows would set them.
func typedResult(columns []string, rows [][]string, kinds [][]db.Kind) db.QueryResult {
	return db.QueryResult{Columns: columns, Rows: rows, Kinds: kinds}
}

func TestMaterialize_DeclaredAffinityFromKinds(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := typedResult(
		[]string{"i", "f", "s", "b", "mixed", "allnull"},
		[][]string{
			{"1", "1.5", "a", "ff00", "1", "NULL"},
			{"2", "2.5", "b", "0102", "x", "NULL"},
		},
		[][]db.Kind{
			{db.KindInt, db.KindFloat, db.KindText, db.KindBlob, db.KindInt, db.KindNull},
			{db.KindInt, db.KindFloat, db.KindText, db.KindBlob, db.KindText, db.KindNull},
		},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	// The declared type is what the UI shows in the column header, so assert on
	// exactly the string DatabaseTypeName reports back.
	got, err := adapter.Query(context.Background(), `SELECT * FROM t1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []string{"INTEGER", "REAL", "TEXT", "BLOB", "", ""}
	if len(got.ColumnTypes) != len(want) {
		t.Fatalf("ColumnTypes = %v, want %v", got.ColumnTypes, want)
	}
	for i, w := range want {
		if got.ColumnTypes[i] != w {
			t.Errorf("column %s: declared type = %q, want %q", result.Columns[i], got.ColumnTypes[i], w)
		}
	}
}

func TestMaterialize_MixedColumnKeepsPerValueStorageClass(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// A mixed column must not be declared TEXT: SQLite would then coerce the
	// bound int64 to text and the numeric ordering would be lost again.
	result := typedResult(
		[]string{"v"},
		[][]string{{"5"}, {"abc"}, {"2.5"}},
		[][]db.Kind{{db.KindInt}, {db.KindText}, {db.KindFloat}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT typeof(v) FROM t1 ORDER BY rowid`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []string{"integer", "text", "real"}
	if len(got.Rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got.Rows), len(want))
	}
	for i, w := range want {
		if got.Rows[i][0] != w {
			t.Errorf("row %d: typeof = %q, want %q", i, got.Rows[i][0], w)
		}
	}
}

func TestMaterialize_NumericOrderingNotLexicographic(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// Lexicographically "10" < "9"; numerically it is the other way round.
	result := typedResult(
		[]string{"n"},
		[][]string{{"9"}, {"10"}, {"100"}},
		[][]db.Kind{{db.KindInt}, {db.KindInt}, {db.KindInt}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT n FROM t1 ORDER BY n`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []string{"9", "10", "100"}
	for i, w := range want {
		if got.Rows[i][0] != w {
			t.Fatalf("ORDER BY n = %v, want %v (string sort would give 10,100,9)", got.Rows, want)
		}
	}
}

func TestMaterialize_NumericJoinAcrossBroughtTables(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	left := typedResult(
		[]string{"id", "name"},
		[][]string{{"1", "alice"}, {"2", "bob"}},
		[][]db.Kind{{db.KindInt, db.KindText}, {db.KindInt, db.KindText}},
	)
	right := typedResult(
		[]string{"id", "score"},
		[][]string{{"1", "10"}, {"2", "20"}},
		[][]db.Kind{{db.KindInt, db.KindInt}, {db.KindInt, db.KindInt}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, left); err != nil {
		t.Fatalf("Materialize t1: %v", err)
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 2, Table: "t2"}, right); err != nil {
		t.Fatalf("Materialize t2: %v", err)
	}

	got, err := adapter.Query(context.Background(),
		`SELECT t1.name, t2.score FROM t1 JOIN t2 ON t1.id = t2.id ORDER BY t2.score DESC`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Rows) != 2 || got.Rows[0][0] != "bob" || got.Rows[1][0] != "alice" {
		t.Fatalf("numeric JOIN + DESC ordering = %+v, want bob then alice", got.Rows)
	}
}

func TestMaterialize_LiteralNullTextIsNotSQLNull(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// Row 1 holds a real SQL NULL; row 2 holds the four characters N,U,L,L.
	// Without kinds these are the same display string and indistinguishable.
	result := typedResult(
		[]string{"id", "name"},
		[][]string{{"1", "NULL"}, {"2", "NULL"}},
		[][]db.Kind{{db.KindInt, db.KindNull}, {db.KindInt, db.KindText}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT id FROM t1 WHERE name IS NULL`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "1" {
		t.Fatalf("IS NULL matched %+v, want only the row scanned as SQL NULL", got.Rows)
	}

	got, err = adapter.Query(context.Background(), `SELECT id FROM t1 WHERE name = 'NULL'`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "2" {
		t.Fatalf(`= 'NULL' matched %+v, want only the row whose text is literally NULL`, got.Rows)
	}
}

func TestMaterialize_LiteralQuotePairIsNotEmptyString(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := typedResult(
		[]string{"id", "name"},
		[][]string{{"1", `""`}, {"2", `""`}},
		[][]db.Kind{{db.KindInt, db.KindEmpty}, {db.KindInt, db.KindText}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT id FROM t1 WHERE name = ''`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "1" {
		t.Fatalf(`= '' matched %+v, want only the row scanned as an empty string`, got.Rows)
	}
}

func TestMaterialize_BlobRoundTrip(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// 0xff 0x00 0xfe is not valid UTF-8, so StringifyValueKind hex-escapes it.
	result := typedResult(
		[]string{"b"},
		[][]string{{"ff00fe"}},
		[][]db.Kind{{db.KindBlob}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	var raw []byte
	if err := conn.QueryRowContext(context.Background(), `SELECT b FROM t1`).Scan(&raw); err != nil {
		t.Fatalf("scan blob: %v", err)
	}
	if string(raw) != string([]byte{0xff, 0x00, 0xfe}) {
		t.Fatalf("blob round-trip = % x, want ff 00 fe", raw)
	}
}

func TestMaterialize_UnparsableNumberFallsBackToText(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// A uint64 above math.MaxInt64 stringifies fine but does not fit a SQLite
	// signed 64-bit integer. It must stay text rather than wrap negative.
	result := typedResult(
		[]string{"n"},
		[][]string{{"18446744073709551615"}},
		[][]db.Kind{{db.KindInt}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT n, typeof(n) FROM t1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got.Rows[0][0] != "18446744073709551615" || got.Rows[0][1] != "text" {
		t.Fatalf("got %+v, want the value preserved as text", got.Rows)
	}
}

func TestMaterialize_NoKindsKeepsLegacyAllTextBehaviour(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := db.QueryResult{
		Columns: []string{"n"},
		Rows:    [][]string{{"1"}},
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT typeof(n) FROM t1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got.Rows[0][0] != "text" {
		t.Fatalf("typeof = %q, want text for a kindless result", got.Rows[0][0])
	}
}

func TestMaterialize_TypesSurviveAFullScanRoundTrip(t *testing.T) {
	// End-to-end: a real query through dbutil.ScanRows produces the kinds, and
	// bringing that result preserves each value's storage class. This is the
	// path the TUI actually takes.
	src, srcAdapter, err := Open()
	if err != nil {
		t.Fatalf("Open source: %v", err)
	}
	defer src.Close()

	if _, err := src.Exec(`CREATE TABLE src (i INTEGER, f REAL, s TEXT, e TEXT, n TEXT, b BLOB)`); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if _, err := src.Exec(`INSERT INTO src VALUES (42, 2.5, 'NULL', '', NULL, ?)`, []byte{0xff, 0xfe}); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	result, err := srcAdapter.Query(context.Background(), `SELECT * FROM src`)
	if err != nil {
		t.Fatalf("source Query: %v", err)
	}
	if !result.HasKinds() {
		t.Fatal("ScanRows did not record kinds")
	}

	dst, dstAdapter, err := Open()
	if err != nil {
		t.Fatalf("Open dest: %v", err)
	}
	defer dst.Close()

	if err := Materialize(context.Background(), dst, dstAdapter.QuoteIdentifier, Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := dstAdapter.Query(context.Background(),
		`SELECT typeof(i), typeof(f), typeof(s), typeof(e), typeof(n), typeof(b), s, e FROM t1`)
	if err != nil {
		t.Fatalf("dest Query: %v", err)
	}
	row := got.Rows[0]
	wantTypes := []string{"integer", "real", "text", "text", "null", "blob"}
	for i, w := range wantTypes {
		if row[i] != w {
			t.Errorf("column %d: typeof = %q, want %q", i, row[i], w)
		}
	}
	// The source column s holds the four characters NULL; it must survive as
	// text, not collapse into SQL NULL. Column e holds a real empty string.
	if row[6] != "NULL" {
		t.Errorf("literal text NULL = %q, want the display sentinel for the text NULL", row[6])
	}
	if row[7] != `""` {
		t.Errorf("empty string = %q, want the `\"\"` display sentinel", row[7])
	}
}

// --- provenance (_asql_bring) ---

func TestMaterialize_RecordsProvenance(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	first := db.QueryResult{
		Columns: []string{"id"},
		Rows:    [][]string{{"1"}, {"2"}},
	}
	second := db.QueryResult{
		Columns:   []string{"a", "b"},
		Rows:      [][]string{{"x", "y"}},
		Truncated: true,
	}

	err = Materialize(context.Background(), conn, adapter.QuoteIdentifier,
		Source{Seq: 1, Table: "t1", Conn: "prod", Query: "SELECT id FROM users"}, first)
	if err != nil {
		t.Fatalf("Materialize t1: %v", err)
	}
	err = Materialize(context.Background(), conn, adapter.QuoteIdentifier,
		Source{Seq: 2, Table: "t2", Conn: "staging", Query: "SELECT a, b FROM events"}, second)
	if err != nil {
		t.Fatalf("Materialize t2: %v", err)
	}

	got, err := adapter.Query(context.Background(),
		`SELECT n, table_name, source, row_count, col_count, truncated, query
		 FROM `+ProvenanceTable+` ORDER BY n`)
	if err != nil {
		t.Fatalf("Query provenance: %v", err)
	}
	want := [][]string{
		{"1", "t1", "prod", "2", "1", "0", "SELECT id FROM users"},
		{"2", "t2", "staging", "1", "2", "1", "SELECT a, b FROM events"},
	}
	if len(got.Rows) != len(want) {
		t.Fatalf("provenance has %d rows, want %d: %+v", len(got.Rows), len(want), got.Rows)
	}
	for r := range want {
		for c := range want[r] {
			if got.Rows[r][c] != want[r][c] {
				t.Errorf("row %d col %d = %q, want %q", r, c, got.Rows[r][c], want[r][c])
			}
		}
	}
}

func TestMaterialize_ProvenanceRolledBackWithFailedBring(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// Seed the provenance table via one successful bring.
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier,
		Source{Seq: 1, Table: "t1", Conn: "prod"},
		db.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"1"}}}); err != nil {
		t.Fatalf("Materialize t1: %v", err)
	}

	// Re-using an existing table name fails at CREATE TABLE.
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier,
		Source{Seq: 2, Table: "t1", Conn: "staging"},
		db.QueryResult{Columns: []string{"id"}, Rows: [][]string{{"9"}}}); err == nil {
		t.Fatal("expected an error when materializing over an existing table")
	}

	got, err := adapter.Query(context.Background(),
		`SELECT count(*) FROM `+ProvenanceTable)
	if err != nil {
		t.Fatalf("Query provenance: %v", err)
	}
	if got.Rows[0][0] != "1" {
		t.Errorf("provenance row count = %s, want 1 (the failed bring must leave no record)", got.Rows[0][0])
	}
}

func TestMaterialize_MixedIntFloatKeepsIntegerPrecision(t *testing.T) {
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	// 2^53+1 is not representable as a float64. A REAL-affinity column would
	// coerce it on the way in and return 9007199254740992.
	result := typedResult(
		[]string{"v"},
		[][]string{{"9007199254740993"}, {"1.5"}},
		[][]db.Kind{{db.KindInt}, {db.KindFloat}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier,
		Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT v, typeof(v) FROM t1 ORDER BY rowid`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got.Rows[0][0] != "9007199254740993" || got.Rows[0][1] != "integer" {
		t.Errorf("large integer = %q (%s), want it preserved exactly as an integer",
			got.Rows[0][0], got.Rows[0][1])
	}
	if got.Rows[1][1] != "real" {
		t.Errorf("float typeof = %q, want real", got.Rows[1][1])
	}

	// The column carries no declared type, so nothing coerces the values.
	schema, err := adapter.Query(context.Background(), `SELECT * FROM t1 LIMIT 0`)
	if err != nil {
		t.Fatalf("Query schema: %v", err)
	}
	if schema.ColumnTypes[0] != "" {
		t.Errorf("declared type = %q, want none for a mixed int/float column", schema.ColumnTypes[0])
	}
}

func TestMaterialize_MixedIntFloatStillOrdersAndJoinsNumerically(t *testing.T) {
	// Dropping the REAL affinity must not cost the numeric behaviour it was
	// there for: SQLite compares integer and real storage classes numerically.
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	left := typedResult(
		[]string{"v"},
		[][]string{{"10"}, {"2.5"}, {"3"}, {"1.25"}},
		[][]db.Kind{{db.KindInt}, {db.KindFloat}, {db.KindInt}, {db.KindFloat}},
	)
	right := typedResult(
		[]string{"v", "tag"},
		[][]string{{"3.0", "three"}},
		[][]db.Kind{{db.KindFloat, db.KindText}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier,
		Source{Seq: 1, Table: "t1"}, left); err != nil {
		t.Fatalf("Materialize t1: %v", err)
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier,
		Source{Seq: 2, Table: "t2"}, right); err != nil {
		t.Fatalf("Materialize t2: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT v FROM t1 ORDER BY v`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []string{"1.25", "2.5", "3", "10"}
	for i, w := range want {
		if got.Rows[i][0] != w {
			t.Fatalf("ORDER BY v = %+v, want %v", got.Rows, want)
		}
	}

	got, err = adapter.Query(context.Background(),
		`SELECT t2.tag FROM t1 JOIN t2 ON t1.v = t2.v`)
	if err != nil {
		t.Fatalf("Query join: %v", err)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "three" {
		t.Errorf("integer 3 JOIN real 3.0 = %+v, want one match", got.Rows)
	}
}

func TestMaterialize_BinaryColumnWithUTF8ContentStaysABlob(t *testing.T) {
	// A BLOB column whose bytes happen to be valid UTF-8 must still come across
	// as a blob, or an X'..' comparison against the original bytes stops
	// matching. The kind comes from the column type, not from UTF-8 validity.
	src, srcAdapter, err := Open()
	if err != nil {
		t.Fatalf("Open source: %v", err)
	}
	defer src.Close()

	if _, err := src.Exec(`CREATE TABLE s (b BLOB)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := src.Exec(`INSERT INTO s VALUES (?)`, []byte("abc")); err != nil {
		t.Fatalf("insert: %v", err)
	}

	result, err := srcAdapter.Query(context.Background(), `SELECT b FROM s`)
	if err != nil {
		t.Fatalf("source Query: %v", err)
	}
	if got := result.KindAt(0, 0); got != db.KindBlob {
		t.Fatalf("kind = %d, want KindBlob for a declared BLOB column", got)
	}

	dst, dstAdapter, err := Open()
	if err != nil {
		t.Fatalf("Open dest: %v", err)
	}
	defer dst.Close()
	if err := Materialize(context.Background(), dst, dstAdapter.QuoteIdentifier,
		Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := dstAdapter.Query(context.Background(),
		`SELECT typeof(b), b = X'616263' FROM t1`)
	if err != nil {
		t.Fatalf("dest Query: %v", err)
	}
	if got.Rows[0][0] != "blob" {
		t.Errorf("typeof = %q, want blob", got.Rows[0][0])
	}
	if got.Rows[0][1] != "1" {
		t.Errorf("b = X'616263' returned %q, want a match against the original bytes", got.Rows[0][1])
	}
}

func TestMaterialize_ZeroLengthBlobStaysABlob(t *testing.T) {
	// A zero-length blob stringifies to "", which then takes the empty-value
	// display sentinel. The sentinel must not downgrade it to an empty string,
	// or typeof and X'' comparisons change on the way across.
	src, srcAdapter, err := Open()
	if err != nil {
		t.Fatalf("Open source: %v", err)
	}
	defer src.Close()

	if _, err := src.Exec(`CREATE TABLE s (b BLOB)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := src.Exec(`INSERT INTO s VALUES (X''), (X'616263')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	result, err := srcAdapter.Query(context.Background(), `SELECT b FROM s ORDER BY rowid`)
	if err != nil {
		t.Fatalf("source Query: %v", err)
	}
	if got := result.KindAt(0, 0); got != db.KindBlob {
		t.Fatalf("kind of the zero-length blob = %d, want KindBlob", got)
	}
	// The display contract is unchanged: it still shows the empty sentinel so
	// the cell does not render blank.
	if result.Rows[0][0] != db.EmptySentinel {
		t.Errorf("display = %q, want the empty sentinel", result.Rows[0][0])
	}

	dst, dstAdapter, err := Open()
	if err != nil {
		t.Fatalf("Open dest: %v", err)
	}
	defer dst.Close()
	if err := Materialize(context.Background(), dst, dstAdapter.QuoteIdentifier,
		Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := dstAdapter.Query(context.Background(),
		`SELECT typeof(b), length(b), b = X'' FROM t1 ORDER BY rowid`)
	if err != nil {
		t.Fatalf("dest Query: %v", err)
	}
	if got.Rows[0][0] != "blob" {
		t.Errorf("typeof = %q, want blob for a zero-length blob", got.Rows[0][0])
	}
	if got.Rows[0][1] != "0" {
		t.Errorf("length = %q, want 0", got.Rows[0][1])
	}
	if got.Rows[0][2] != "1" {
		t.Errorf("b = X'' returned %q, want a match", got.Rows[0][2])
	}
	// The non-empty blob in the same column must be unaffected.
	if got.Rows[1][0] != "blob" || got.Rows[1][1] != "3" {
		t.Errorf("non-empty blob = typeof %q length %q, want blob/3", got.Rows[1][0], got.Rows[1][1])
	}
}

func TestMaterialize_EmptyStringIsStillAnEmptyString(t *testing.T) {
	// The blob carve-out must not leak into text columns.
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := typedResult(
		[]string{"v"},
		[][]string{{db.EmptySentinel}},
		[][]db.Kind{{db.KindEmpty}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier,
		Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT typeof(v), length(v) FROM t1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got.Rows[0][0] != "text" || got.Rows[0][1] != "0" {
		t.Errorf("got typeof %q length %q, want text/0", got.Rows[0][0], got.Rows[0][1])
	}
}

func TestMaterialize_NaNStaysTextRatherThanBecomingNull(t *testing.T) {
	// SQLite stores a bound NaN as SQL NULL. Letting that happen would make a
	// NaN indistinguishable from a real NULL — precisely the confusion kinds
	// exist to remove — so it must stay text. Infinities bind fine and must
	// still round-trip as reals.
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := typedResult(
		[]string{"v"},
		[][]string{{"NaN"}, {"+Inf"}, {"-Inf"}, {"1.5"}},
		[][]db.Kind{{db.KindFloat}, {db.KindFloat}, {db.KindFloat}, {db.KindFloat}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier,
		Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT v, typeof(v) FROM t1 ORDER BY rowid`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got.Rows[0][0] != "NaN" || got.Rows[0][1] != "text" {
		t.Errorf("NaN stored as %q (%s), want the text NaN", got.Rows[0][0], got.Rows[0][1])
	}
	for i, want := range map[int]string{1: "+Inf", 2: "-Inf", 3: "1.5"} {
		if got.Rows[i][0] != want || got.Rows[i][1] != "real" {
			t.Errorf("row %d = %q (%s), want %q as a real", i, got.Rows[i][0], got.Rows[i][1], want)
		}
	}

	// A NaN must not be reachable through IS NULL.
	nulls, err := adapter.Query(context.Background(), `SELECT count(*) FROM t1 WHERE v IS NULL`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if nulls.Rows[0][0] != "0" {
		t.Errorf("IS NULL matched %s rows, want 0", nulls.Rows[0][0])
	}
}

func TestMaterialize_BlobFromAnExpressionStaysABlob(t *testing.T) {
	// A blob produced by an expression has no declared column type, so the
	// column-type heuristic cannot see it. modernc.org/sqlite returns []byte
	// only for the BLOB storage class, which identifies it exactly.
	src, srcAdapter, err := Open()
	if err != nil {
		t.Fatalf("Open source: %v", err)
	}
	defer src.Close()

	cases := []struct {
		name string
		expr string
		hex  string
	}{
		{"blob literal", `SELECT X'616263' AS v`, "616263"},
		{"cast to blob", `SELECT CAST('abc' AS BLOB) AS v`, "616263"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := srcAdapter.Query(context.Background(), tc.expr)
			if err != nil {
				t.Fatalf("source Query: %v", err)
			}
			if got := result.KindAt(0, 0); got != db.KindBlob {
				t.Fatalf("kind = %d, want KindBlob (display %q)", got, result.Rows[0][0])
			}
			if result.Rows[0][0] != tc.hex {
				t.Errorf("display = %q, want the hex form %q", result.Rows[0][0], tc.hex)
			}

			dst, dstAdapter, err := Open()
			if err != nil {
				t.Fatalf("Open dest: %v", err)
			}
			defer dst.Close()
			if err := Materialize(context.Background(), dst, dstAdapter.QuoteIdentifier,
				Source{Seq: 1, Table: "t1"}, result); err != nil {
				t.Fatalf("Materialize: %v", err)
			}

			got, err := dstAdapter.Query(context.Background(),
				`SELECT typeof(v), v = X'616263' FROM t1`)
			if err != nil {
				t.Fatalf("dest Query: %v", err)
			}
			if got.Rows[0][0] != "blob" || got.Rows[0][1] != "1" {
				t.Errorf("typeof = %q, match = %q; want blob and a match", got.Rows[0][0], got.Rows[0][1])
			}
		})
	}
}

func TestMaterialize_TextFromAnExpressionStaysText(t *testing.T) {
	// The mirror case: text-producing expressions must not be swept up.
	src, srcAdapter, err := Open()
	if err != nil {
		t.Fatalf("Open source: %v", err)
	}
	defer src.Close()

	for _, expr := range []string{
		`SELECT 'abc' AS v`,
		`SELECT CAST(X'616263' AS TEXT) AS v`,
		`SELECT upper('abc') AS v`,
	} {
		result, err := srcAdapter.Query(context.Background(), expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got := result.KindAt(0, 0); got != db.KindText {
			t.Errorf("%s: kind = %d, want KindText (display %q)", expr, got, result.Rows[0][0])
		}
		if result.Rows[0][0] != "ABC" && result.Rows[0][0] != "abc" {
			t.Errorf("%s: display = %q, want readable text", expr, result.Rows[0][0])
		}
	}
}

func TestMaterialize_SqliteDynamicTypingBeatsTheDeclaredType(t *testing.T) {
	// SQLite lets a TEXT column hold a blob. The Go type follows the value, so
	// the blob survives even though the column says TEXT.
	src, srcAdapter, err := Open()
	if err != nil {
		t.Fatalf("Open source: %v", err)
	}
	defer src.Close()

	if _, err := src.Exec(`CREATE TABLE s (v TEXT)`); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := src.Exec(`INSERT INTO s VALUES (X'616263'), ('plain')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	result, err := srcAdapter.Query(context.Background(), `SELECT v FROM s ORDER BY rowid`)
	if err != nil {
		t.Fatalf("source Query: %v", err)
	}
	if got := result.KindAt(0, 0); got != db.KindBlob {
		t.Errorf("blob in a TEXT column: kind = %d, want KindBlob", got)
	}
	if got := result.KindAt(1, 0); got != db.KindText {
		t.Errorf("text in a TEXT column: kind = %d, want KindText", got)
	}
}

func TestMaterialize_DecimalColumnSortsNumerically(t *testing.T) {
	// Stands in for a MySQL DECIMAL column: the driver hands the digits back as
	// text, and dbutil classifies them by the column type (see numericKindOf),
	// so they reach Materialize already carrying numeric kinds. Without that,
	// the column would be TEXT and 10 would sort before 2.
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := typedResult(
		[]string{"price"},
		[][]string{{"10"}, {"2"}, {"2.50"}},
		[][]db.Kind{{db.KindInt}, {db.KindInt}, {db.KindFloat}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier,
		Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT price FROM t1 ORDER BY price`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []string{"2", "2.5", "10"}
	for i, w := range want {
		if got.Rows[i][0] != w {
			t.Fatalf("ORDER BY price = %+v, want %v (string sort would give 10, 2, 2.50)", got.Rows, want)
		}
	}
}

func TestMaterialize_HighPrecisionDecimalStaysText(t *testing.T) {
	// A decimal beyond float64's exact range keeps its digits rather than being
	// silently rounded — dbutil leaves it KindText, and it must survive as such.
	conn, adapter, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	result := typedResult(
		[]string{"price"},
		[][]string{{"12345678901234567.89"}},
		[][]db.Kind{{db.KindText}},
	)
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier,
		Source{Seq: 1, Table: "t1"}, result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT price, typeof(price) FROM t1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got.Rows[0][0] != "12345678901234567.89" || got.Rows[0][1] != "text" {
		t.Errorf("got %q (%s), want the digits preserved as text", got.Rows[0][0], got.Rows[0][1])
	}
}
