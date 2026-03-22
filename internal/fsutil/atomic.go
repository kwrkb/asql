package fsutil

import (
	"io/fs"
	"os"
	"path/filepath"
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
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
