package snippet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	snippets, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snippets) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(snippets))
	}
}

func TestLoadEmptyFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "asql", "snippets.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	snippets, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snippets) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(snippets))
	}
}

func TestLoadValidFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "asql", "snippets.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `- name: "active users"
  query: "SELECT * FROM users WHERE active = 1;"
- name: "recent orders"
  query: "SELECT * FROM orders ORDER BY created_at DESC LIMIT 10;"
`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	snippets, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snippets) != 2 {
		t.Fatalf("expected 2 snippets, got %d", len(snippets))
	}
	if snippets[0].Name != "active users" {
		t.Errorf("expected name 'active users', got %q", snippets[0].Name)
	}
	if snippets[1].Query != "SELECT * FROM orders ORDER BY created_at DESC LIMIT 10;" {
		t.Errorf("unexpected query: %q", snippets[1].Query)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "asql", "snippets.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	original := []Snippet{
		{Name: "test query", Query: "SELECT 1;"},
		{Name: "another", Query: "SELECT * FROM foo;"},
	}

	if err := Save(original); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded) != len(original) {
		t.Fatalf("expected %d snippets, got %d", len(original), len(loaded))
	}
	for i := range original {
		if loaded[i].Name != original[i].Name || loaded[i].Query != original[i].Query {
			t.Errorf("mismatch at %d: got %+v, want %+v", i, loaded[i], original[i])
		}
	}
}
