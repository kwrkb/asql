package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Load from non-existent file should return nil
	profiles, err := Load()
	if err != nil {
		t.Fatalf("Load from empty dir: %v", err)
	}
	if profiles != nil {
		t.Fatalf("expected nil, got %v", profiles)
	}

	// Save and reload
	want := []Profile{
		{Name: "local", DSN: "test.db"},
		{Name: "prod", DSN: "postgres://user:pass@host:5432/db"},
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d profiles, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].DSN != want[i].DSN {
			t.Errorf("profile[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSaveFilePermissions(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	if err := Save([]Profile{{Name: "test", DSN: "test.db"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(tmp, "asql", "profiles.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file perm = %o, want 0600", perm)
	}
}

func TestFind(t *testing.T) {
	profiles := []Profile{
		{Name: "a", DSN: "a.db"},
		{Name: "b", DSN: "b.db"},
	}

	if p := Find(profiles, "a"); p == nil || p.DSN != "a.db" {
		t.Errorf("Find(a) = %v, want a.db", p)
	}
	if p := Find(profiles, "missing"); p != nil {
		t.Errorf("Find(missing) = %v, want nil", p)
	}
}

func TestLoadEmptyFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir := filepath.Join(tmp, "asql")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "profiles.yaml"), []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	profiles, err := Load()
	if err != nil {
		t.Fatalf("Load empty file: %v", err)
	}
	if profiles != nil {
		t.Fatalf("expected nil, got %v", profiles)
	}
}
