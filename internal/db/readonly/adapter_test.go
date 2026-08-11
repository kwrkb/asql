package readonly

import (
	"context"
	"testing"

	"github.com/kwrkb/asql/internal/db"
)

// fakeAdapter records what reached the wrapped adapter.
type fakeAdapter struct {
	dbType     string
	queries    []string
	tablesHit  int
	columnsHit int
	schemaHit  int
	closeHit   int
}

func (f *fakeAdapter) Type() string { return f.dbType }

func (f *fakeAdapter) Query(_ context.Context, query string) (db.QueryResult, error) {
	f.queries = append(f.queries, query)
	return db.QueryResult{Columns: []string{"a"}}, nil
}

func (f *fakeAdapter) Tables(context.Context) ([]string, error) {
	f.tablesHit++
	return []string{"t"}, nil
}

func (f *fakeAdapter) Columns(context.Context, string) ([]string, error) {
	f.columnsHit++
	return []string{"a"}, nil
}

func (f *fakeAdapter) Schema(context.Context) (string, error) {
	f.schemaHit++
	return "CREATE TABLE t (a TEXT)", nil
}

func (f *fakeAdapter) QuoteIdentifier(name string) string { return `"` + name + `"` }

func (f *fakeAdapter) Close() error {
	f.closeHit++
	return nil
}

func TestAdapterRefusesWritesBeforeReachingTheDatabase(t *testing.T) {
	inner := &fakeAdapter{dbType: "sqlite"}
	guarded := Wrap(inner)

	if _, err := guarded.Query(context.Background(), "DELETE FROM t"); err == nil {
		t.Fatal("Query(DELETE) = nil error, want refusal")
	}
	if len(inner.queries) != 0 {
		t.Fatalf("refused query reached the adapter: %v", inner.queries)
	}

	if _, err := guarded.Query(context.Background(), "SELECT * FROM t"); err != nil {
		t.Fatalf("Query(SELECT) = %v, want success", err)
	}
	if len(inner.queries) != 1 || inner.queries[0] != "SELECT * FROM t" {
		t.Fatalf("adapter saw %v, want the SELECT", inner.queries)
	}
}

// Tables/Columns/Schema build their own SQL inside the adapters, so the guard
// must not stand in their way — the sidebar would go empty if it did.
func TestAdapterPassesMetadataThrough(t *testing.T) {
	inner := &fakeAdapter{dbType: "sqlite"}
	guarded := Wrap(inner)
	ctx := context.Background()

	if _, err := guarded.Tables(ctx); err != nil {
		t.Fatalf("Tables: %v", err)
	}
	if _, err := guarded.Columns(ctx, "t"); err != nil {
		t.Fatalf("Columns: %v", err)
	}
	if _, err := guarded.Schema(ctx); err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if err := guarded.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if inner.tablesHit != 1 || inner.columnsHit != 1 || inner.schemaHit != 1 || inner.closeHit != 1 {
		t.Fatalf("pass-through counts: tables=%d columns=%d schema=%d close=%d, want 1 each",
			inner.tablesHit, inner.columnsHit, inner.schemaHit, inner.closeHit)
	}
	if guarded.Type() != "sqlite" {
		t.Errorf("Type() = %q, want sqlite", guarded.Type())
	}
	if guarded.QuoteIdentifier("a b") != `"a b"` {
		t.Errorf("QuoteIdentifier not delegated: %q", guarded.QuoteIdentifier("a b"))
	}
}

// The dialect used to scan a statement comes from the wrapped adapter, so a
// backtick-quoted identifier is understood on MySQL and a dollar-quoted string
// on PostgreSQL.
func TestAdapterUsesTheWrappedDialect(t *testing.T) {
	mysqlGuard := Wrap(&fakeAdapter{dbType: "mysql"})
	if _, err := mysqlGuard.Query(context.Background(), "SELECT `;` FROM t"); err != nil {
		t.Errorf("MySQL backtick identifier refused: %v", err)
	}

	pgGuard := Wrap(&fakeAdapter{dbType: "postgres"})
	if _, err := pgGuard.Query(context.Background(), "SELECT $$;$$"); err != nil {
		t.Errorf("PostgreSQL dollar-quoted string refused: %v", err)
	}
}

func TestWrapIsIdempotent(t *testing.T) {
	inner := &fakeAdapter{dbType: "sqlite"}
	once := Wrap(inner)
	twice := Wrap(once)
	if once != twice {
		t.Fatal("Wrap wrapped an already-guarded adapter again")
	}
	if !IsWrapped(once) {
		t.Fatal("IsWrapped(guarded) = false")
	}
	if IsWrapped(inner) {
		t.Fatal("IsWrapped(plain adapter) = true")
	}
	if Wrap(nil) != nil {
		t.Fatal("Wrap(nil) should stay nil")
	}
}
