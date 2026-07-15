//go:build !windows

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ki-kneip/dem/internal/core"
)

// persistPath appends a PATH export to the active shell's rc file
// (detected from $SHELL), skipping the write if it's already there.
// Returns false, nil for shells it doesn't recognize (or when $SHELL
// isn't set), falling back to the manual instructions — there is no
// single place to edit across every shell and dotfile manager, so
// unrecognized setups stay untouched rather than guessed at.
func persistPath(paths core.Paths) (bool, error) {
	rcPath, line, ok := rcFileFor(paths)
	if !ok {
		return false, nil
	}

	existing, err := os.ReadFile(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if strings.Contains(string(existing), paths.Bin) && strings.Contains(string(existing), paths.Shims) {
		return true, nil // already configured, e.g. re-running setup
	}

	if err := os.MkdirAll(filepath.Dir(rcPath), 0o755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	defer f.Close()

	if _, err := f.WriteString("\n# added by 'dem setup'\n" + line + "\n"); err != nil {
		return false, err
	}
	return true, nil
}

// unpersistPath removes the exact block persistPath appended for
// paths, when the active shell's rc file is recognized and still
// contains it verbatim. A missing rc file, an unrecognized shell, or
// a block that no longer matches (hand-edited, already removed) is
// not an error — relocate falls back to printing manual instructions.
func unpersistPath(paths core.Paths) error {
	rcPath, line, ok := rcFileFor(paths)
	if !ok {
		return nil
	}
	data, err := os.ReadFile(rcPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	block := "\n# added by 'dem setup'\n" + line + "\n"
	if !strings.Contains(string(data), block) {
		return nil
	}
	updated := strings.Replace(string(data), block, "", 1)
	return os.WriteFile(rcPath, []byte(updated), 0o644)
}

// rcFileFor maps the shell in $SHELL to its rc file and the line to
// append to it.
func rcFileFor(paths core.Paths) (rcPath, line string, ok bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", false
	}
	switch filepath.Base(os.Getenv("SHELL")) {
	case "zsh":
		return filepath.Join(home, ".zshrc"),
			fmt.Sprintf(`export PATH="%s:%s:$PATH"`, paths.Shims, paths.Bin), true
	case "bash":
		return filepath.Join(home, ".bashrc"),
			fmt.Sprintf(`export PATH="%s:%s:$PATH"`, paths.Shims, paths.Bin), true
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"),
			fmt.Sprintf(`set -gx PATH %s %s $PATH`, paths.Shims, paths.Bin), true
	default:
		return "", "", false
	}
}
