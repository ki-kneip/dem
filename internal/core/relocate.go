package core

import (
	"os"
	"path/filepath"
)

// MoveTo relocates tools/, cache/ and config.yaml from p into dst,
// which must already exist (see EnsureLayout). Shims and the dem
// binary are deliberately not moved here: the caller rebuilds shims
// fresh at the new location (see ShimNames/EnsureShims) and installs
// the binary separately, since both need OS-specific care around a
// dem binary that may currently be the one running this command.
func (p Paths) MoveTo(dst Paths) error {
	for _, pair := range [][2]string{
		{p.Tools, dst.Tools},
		{p.Cache, dst.Cache},
	} {
		if err := moveContents(pair[0], pair[1]); err != nil {
			return err
		}
	}
	return moveFile(p.ConfigFile(), dst.ConfigFile())
}

// moveContents moves every entry of srcDir into dstDir, which must
// already exist. Renaming is tried first (instant, same filesystem);
// an entry that cannot be renamed (crossing a filesystem boundary)
// falls back to a recursive copy followed by removing the source.
func moveContents(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(dstDir, e.Name())
		if err := os.Rename(src, dst); err == nil {
			continue
		}
		if err := copyTree(src, dst); err != nil {
			return err
		}
		if err := os.RemoveAll(src); err != nil {
			return err
		}
	}
	return nil
}

// moveFile moves a single file, falling back to copy+remove across a
// filesystem boundary. A missing src is not an error (e.g. no
// config.yaml written yet).
func moveFile(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// copyTree recursively copies src into dst; only used by moveContents
// when a rename cannot cross a filesystem boundary.
func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
