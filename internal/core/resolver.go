package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

// ProjectFileName is the per-project versions file.
const ProjectFileName = "dem.yaml"

// ProjectFile is the dem.yaml at a project's root.
type ProjectFile struct {
	Tools map[string]string `yaml:"tools"`
}

// FindProjectFile walks up from dir looking for a dem.yaml.
func FindProjectFile(dir string) (path string, pf ProjectFile, found bool, err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", pf, false, err
	}
	for {
		candidate := filepath.Join(dir, ProjectFileName)
		data, err := os.ReadFile(candidate)
		if err == nil {
			if err := yaml.Unmarshal(data, &pf); err != nil {
				return candidate, pf, false, fmt.Errorf("invalid %s: %w", candidate, err)
			}
			return candidate, pf, true, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", pf, false, nil
		}
		dir = parent
	}
}

// SaveProjectFile writes the dem.yaml.
func SaveProjectFile(path string, pf ProjectFile) error {
	data, err := yaml.Marshal(pf)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ResolveActive decides which version of tool applies in startDir:
// the project's dem.yaml (walking up directories) > the global
// config. source describes where the decision came from, for the
// user's benefit.
func ResolveActive(tool, startDir string, cfg Config) (version, source string, err error) {
	path, pf, found, err := FindProjectFile(startDir)
	if err != nil {
		return "", "", err
	}
	if found {
		if v, ok := pf.Tools[tool]; ok && v != "" {
			return v, path, nil
		}
	}
	if v, ok := cfg.Global[tool]; ok && v != "" {
		return v, "global", nil
	}
	return "", "", fmt.Errorf("no version of %s configured; run 'dem install %s@latest'", tool, tool)
}

// MatchSpec picks, from a list ordered newest to oldest, the first
// version matching the user's spec: "latest", "lts", exact
// ("22.5.1") or prefix ("22", "1.22").
func MatchSpec(spec string, versions []Version) (Version, error) {
	if len(versions) == 0 {
		return Version{}, fmt.Errorf("no versions available")
	}
	switch strings.ToLower(spec) {
	case "", "latest":
		return versions[0], nil
	case "lts":
		for _, v := range versions {
			if v.LTS {
				return v, nil
			}
		}
		return Version{}, fmt.Errorf("no LTS version found")
	}
	for _, v := range versions {
		if v.Raw == spec || strings.HasPrefix(v.Raw, spec+".") {
			return v, nil
		}
	}
	return Version{}, fmt.Errorf("no version matches %q", spec)
}

// SortVersionsDesc sorts version strings newest to oldest, using
// semver whenever possible.
func SortVersionsDesc(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		a, b := "v"+strings.TrimPrefix(versions[i], "v"), "v"+strings.TrimPrefix(versions[j], "v")
		if semver.IsValid(a) && semver.IsValid(b) {
			return semver.Compare(a, b) > 0
		}
		return versions[i] > versions[j]
	})
}
