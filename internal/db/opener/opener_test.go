package opener

import (
	"os"
	"path/filepath"
	"testing"
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
