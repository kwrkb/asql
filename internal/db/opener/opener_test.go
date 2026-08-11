package opener

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kwrkb/asql/internal/db/readonly"
	"github.com/kwrkb/asql/internal/db/sqlite"
)

func TestOpen_SQLite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	// Create the file so SQLite can open it
	if err := os.WriteFile(dbPath, nil, 0600); err != nil {
		t.Fatal(err)
	}

	adapter, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open(%q) failed: %v", dbPath, err)
	}
	defer adapter.Close()

	if got := adapter.Type(); got != "sqlite" {
		t.Errorf("adapter.Type() = %q, want %q", got, "sqlite")
	}
}

func TestOpen_InvalidMySQL(t *testing.T) {
	// Invalid MySQL DSN should return an error, not panic
	_, err := Open("mysql://invalid:invalid@localhost:99999/nonexistent")
	if err == nil {
		t.Error("Open with invalid MySQL DSN should return error")
	}
}

func TestOpen_InvalidPostgres(t *testing.T) {
	// Invalid PostgreSQL DSN should return an error, not panic
	_, err := Open("postgres://invalid:invalid@localhost:99999/nonexistent")
	if err == nil {
		t.Error("Open with invalid PostgreSQL DSN should return error")
	}
}

// The wiring between the flag and the guard: OpenReadonly must return an
// adapter that refuses writes, not merely a connection that happens to be
// opened read-only. Both layers are checked here because a session gets
// whichever one this function attaches.
func TestOpenReadonlyGuardsAndRefuses(t *testing.T) {
	path := t.TempDir() + "/data.db"
	seed, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ctx := context.Background()
	if _, err := seed.Query(ctx, "CREATE TABLE t (a TEXT)"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	seed.Close()

	adapter, err := OpenReadonly(path)
	if err != nil {
		t.Fatalf("OpenReadonly: %v", err)
	}
	defer adapter.Close()

	if !readonly.IsWrapped(adapter) {
		t.Fatal("OpenReadonly returned an unguarded adapter")
	}
	_, err = adapter.Query(ctx, "INSERT INTO t VALUES ('x')")
	if err == nil {
		t.Fatal("INSERT was accepted")
	}
	if !readonly.IsRefused(err) {
		t.Fatalf("INSERT failed with %v, want the statement guard to refuse it first", err)
	}
	if _, err := adapter.Query(ctx, "SELECT * FROM t"); err != nil {
		t.Fatalf("SELECT: %v", err)
	}

	// Open must stay unguarded — readonly is opt-in.
	plain, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer plain.Close()
	if readonly.IsWrapped(plain) {
		t.Fatal("Open returned a guarded adapter")
	}
}
