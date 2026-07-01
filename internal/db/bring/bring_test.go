package bring

import (
	"context"
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

	result := db.QueryResult{
		Columns: []string{"id", "id", "id_2"},
		Rows:    [][]string{{"a", "b", "c"}},
	}
	if err := Materialize(context.Background(), conn, adapter.QuoteIdentifier, "t1", result); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	got, err := adapter.Query(context.Background(), `SELECT id, id_2, id_2_2 FROM t1`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	want := []string{"a", "b", "c"}
	if len(got.Rows) != 1 || got.Rows[0][0] != want[0] || got.Rows[0][1] != want[1] || got.Rows[0][2] != want[2] {
		t.Fatalf("expected disambiguated columns to round-trip in order, got %+v", got.Rows)
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

	rows := make([][]string, 0, batchSize*2+10)
	for i := 0; i < batchSize*2+10; i++ {
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
