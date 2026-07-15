package core

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ShimBinaryName is the name of the executor extracted into shims/.
func ShimBinaryName() string {
	if runtime.GOOS == "windows" {
		return "dem-shim.exe"
	}
	return "dem-shim"
}

// EnsureShims writes the dem-shim binary (payload, embedded in dem)
// into shims/ and creates each name in names as a hardlink to it
// (symlink on Unix). The disk cost is the shim once, no matter how
// many tools get a shim.
func (p Paths) EnsureShims(payload []byte, names []string) error {
	if err := os.MkdirAll(p.Shims, 0o755); err != nil {
		return err
	}
	target := filepath.Join(p.Shims, ShimBinaryName())
	if err := writeIfChanged(target, payload); err != nil {
		return fmt.Errorf("extracting dem-shim: %w", err)
	}
	for _, name := range names {
		if runtime.GOOS == "windows" {
			if err := LinkOrCopy(target, filepath.Join(p.Shims, name+".exe")); err != nil {
				return fmt.Errorf("shim %s: %w", name, err)
			}
			continue
		}
		dst := filepath.Join(p.Shims, name)
		os.Remove(dst)
		if err := os.Symlink(target, dst); err != nil {
			return fmt.Errorf("shim %s: %w", name, err)
		}
	}
	p.sweepOldShims()
	return nil
}

// RefreshShims re-extracts dem-shim and relinks every existing shim
// (after a self-update, hardlinks keep pointing at the old content
// until recreated).
func (p Paths) RefreshShims(payload []byte) error {
	names, err := p.ShimNames()
	if err != nil {
		return err
	}
	return p.EnsureShims(payload, names)
}

// ShimNames lists the tool names currently linked in shims/ (not
// dem-shim itself or leftover .old files) — what a name in shims/
// resolves back to, independent of the registry or providers.
func (p Paths) ShimNames() ([]string, error) {
	entries, err := os.ReadDir(p.Shims)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasSuffix(name, ".old") || name == ShimBinaryName() {
			continue
		}
		names = append(names, strings.TrimSuffix(name, ".exe"))
	}
	return names, nil
}

// writeIfChanged writes content to path only when the content
// differs. An existing path is renamed to .old first — Windows
// allows renaming a running executable, but not overwriting it.
func writeIfChanged(path string, content []byte) error {
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, content) {
		return nil
	}
	if _, err := os.Lstat(path); err == nil {
		old := path + ".old"
		os.Remove(old)
		if err := os.Rename(path, old); err != nil {
			return err
		}
	}
	return os.WriteFile(path, content, 0o755)
}

// LinkOrCopy creates dst as a hardlink to src, falling back to a
// copy when the filesystem does not support it. If dst is already
// the same file, it does nothing. An existing dst is renamed to
// .old before the swap.
func LinkOrCopy(src, dst string) error {
	if isSameFile(src, dst) {
		return nil
	}
	if _, err := os.Lstat(dst); err == nil {
		old := dst + ".old"
		os.Remove(old)
		if err := os.Rename(dst, old); err != nil {
			return err
		}
	}
	if err := os.Link(src, dst); err != nil {
		return copyFile(src, dst)
	}
	return nil
}

// sweepOldShims deletes the *.old files left over from swaps; any
// still in use fail silently and are caught by the next sweep.
func (p Paths) sweepOldShims() {
	matches, _ := filepath.Glob(filepath.Join(p.Shims, "*.old"))
	for _, m := range matches {
		os.Remove(m)
	}
}

func isSameFile(a, b string) bool {
	ia, err := os.Stat(a)
	if err != nil {
		return false
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ia, ib)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
