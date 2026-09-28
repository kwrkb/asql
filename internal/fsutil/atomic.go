package fsutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// AtomicWrite writes data to path atomically by writing to a temporary file
// in the same directory and renaming it. This prevents data loss if the
// process crashes during the write.
func AtomicWrite(path string, data []byte, perm fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	// Flush the data before the rename: without it a power loss can persist
	// the rename but not the contents, leaving an empty or partial file.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

// syncDir fsyncs a directory so a rename inside it survives a crash.
// Windows cannot open a directory for syncing, so it is skipped there.
func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
