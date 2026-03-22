package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	data := []byte("hello world")
	if err := AtomicWrite(path, data, 0o600); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("content = %q, want %q", got, data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perm = %o, want 600", perm)
	}
}

func TestAtomicWriteOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := AtomicWrite(path, []byte("first"), 0o600); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := AtomicWrite(path, []byte("second"), 0o600); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("content = %q, want %q", got, "second")
	}
}

func TestAtomicWriteNoTempLeftOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := AtomicWrite(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
}

func TestAtomicWriteInvalidDir(t *testing.T) {
	err := AtomicWrite("/nonexistent-dir-xyz/file.txt", []byte("data"), 0o600)
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}
