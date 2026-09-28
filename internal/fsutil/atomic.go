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
	syncDir(filepath.Dir(path))
	return nil
}

// syncDir fsyncs a directory so a rename inside it survives a crash. It is
// best effort: the file is already in place by now, and some filesystems
// (network mounts, WSL drvfs) refuse to sync a directory, which must not turn
// a completed write into a reported failure. Windows cannot open a directory
// for syncing, so it is skipped there.
func syncDir(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	defer d.Close()
	_ = d.Sync()
}
