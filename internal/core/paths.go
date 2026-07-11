package core

import (
	"os"
	"path/filepath"
	"strings"
)

// Paths groups the directories DEM uses on the user's machine.
type Paths struct {
	Root  string // ~/.dem (or DEM_HOME, or the directory chosen at setup)
	Bin   string // the dem binary
	Shims string // interceptor binaries on PATH
	Tools string // installations: tools/<tool>/<version>
	Cache string // verified downloads
}

// DefaultRoot is ~/.dem on every platform, so docs and support have
// a single canonical path.
func DefaultRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dem"), nil
}

// PathsAt builds the layout from a chosen root.
func PathsAt(root string) Paths {
	return Paths{
		Root:  root,
		Bin:   filepath.Join(root, "bin"),
		Shims: filepath.Join(root, "shims"),
		Tools: filepath.Join(root, "tools"),
		Cache: filepath.Join(root, "cache"),
	}
}

// ResolvePaths decides which root applies, in order: DEM_HOME, the
// installation the running binary lives in, the default.
func ResolvePaths() (Paths, error) {
	if root := os.Getenv("DEM_HOME"); root != "" {
		return PathsAt(root), nil
	}
	if root, ok := rootFromExecutable(); ok {
		return PathsAt(root), nil
	}
	root, err := DefaultRoot()
	if err != nil {
		return Paths{}, err
	}
	return PathsAt(root), nil
}

// rootFromExecutable discovers the root when the binary runs from
// inside an installation (<root>/bin or <root>/shims, identified by
// the config.yaml next to them). This is what makes a custom-directory
// installation work without any environment variable.
func rootFromExecutable() (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved // shims on Unix are symlinks
	}
	dir := filepath.Dir(exe)
	switch strings.ToLower(filepath.Base(dir)) {
	case "bin", "shims":
	default:
		return "", false
	}
	root := filepath.Dir(dir)
	if _, err := os.Stat(PathsAt(root).ConfigFile()); err != nil {
		return "", false
	}
	return root, true
}

// EnsureLayout creates the directory tree if it does not exist.
func (p Paths) EnsureLayout() error {
	for _, dir := range []string{p.Root, p.Bin, p.Shims, p.Tools, p.Cache} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// ConfigFile is the path of the global config.
func (p Paths) ConfigFile() string {
	return filepath.Join(p.Root, "config.yaml")
}
