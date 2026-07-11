package core

import (
	"os"
	"path/filepath"
	"runtime"
)

// InstallDir is where a tool version lives: tools/<tool>/<version>.
func (p Paths) InstallDir(tool, version string) string {
	return filepath.Join(p.Tools, tool, version)
}

// IsInstalled checks whether the version exists on disk.
func (p Paths) IsInstalled(tool, version string) bool {
	info, err := os.Stat(p.InstallDir(tool, version))
	return err == nil && info.IsDir()
}

// InstalledVersions lists the installed versions of tool, newest to
// oldest.
func (p Paths) InstalledVersions(tool string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(p.Tools, tool))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	SortVersionsDesc(versions)
	return versions, nil
}

// FindExecutable looks for the binary name inside an installation,
// checking the root and bin/ (covers Go, Node and single binaries).
func FindExecutable(installDir, name string) (string, bool) {
	exts := []string{""}
	if runtime.GOOS == "windows" {
		exts = []string{".exe", ".cmd", ".bat", ""}
	}
	for _, dir := range []string{installDir, filepath.Join(installDir, "bin")} {
		for _, ext := range exts {
			candidate := filepath.Join(dir, name+ext)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, true
			}
		}
	}
	return "", false
}
