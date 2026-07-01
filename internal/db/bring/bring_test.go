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
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t1", result); err != nil {
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
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t1", result); err != nil {
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
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t1", result); err != nil {
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
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t1", result); err != nil {
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
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "order", result); err != nil {
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
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t1", left); err != nil {
		t.Fatalf("Materialize left: %v", err)
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t2", right); err != nil {
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
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t1", result); err != nil {
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
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t1", result); err != nil {
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
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t1", result); err != nil {
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

	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "wide", result); err != nil {
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
	if err := Materialize(ctx, conn, adapter.QuoteIdentifier, "t1", result); err == nil {
		t.Fatal("expected Materialize to fail with an already-cancelled context")
	}

	// Retrying with the same table name and a valid context must succeed —
	// the failed attempt must not have left behind a partially-created
	// table that collides with the retry.
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t1", result); err != nil {
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

	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t1", db.QueryResult{
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
