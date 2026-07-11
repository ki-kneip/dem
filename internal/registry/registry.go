// Package registry parses DEM's declarative tool registry — the
// descriptors for tools shipped as single binaries or archives on
// GitHub Releases (see registry.yaml at the repository root).
//
// This package deliberately has no network code: dem-shim links it to
// map executables back to tools, and the shim must stay lean. The
// periodic refresh from the repository lives in the providers package.
package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	dem "github.com/ki-kneip/dem"
)

// SupportedSchema is the registry schema this dem build understands.
// A fetched registry with a different schema is ignored, so newer
// registry formats never break older installations.
const SupportedSchema = 1

// CacheFileName is the on-disk name of the fetched registry copy,
// stored in the cache directory.
const CacheFileName = "registry.yaml"

// CacheTTL is how long a fetched copy is considered fresh.
const CacheTTL = 24 * time.Hour

// Tool describes one GitHub-release tool.
type Tool struct {
	Repo        string            `yaml:"repo"`
	Executables []string          `yaml:"executables"`
	Assets      map[string]string `yaml:"assets"`
	// Checksums is the optional name of a sha256sum-format asset used
	// to verify downloads.
	Checksums string `yaml:"checksums"`
}

// Registry is the parsed registry file.
type Registry struct {
	Schema int             `yaml:"schema"`
	Tools  map[string]Tool `yaml:"tools"`
}

// Parse decodes and validates a registry document.
func Parse(data []byte) (Registry, error) {
	var r Registry
	if err := yaml.Unmarshal(data, &r); err != nil {
		return r, err
	}
	if r.Schema != SupportedSchema {
		return r, fmt.Errorf("registry schema %d is not supported by this dem build (supports %d)", r.Schema, SupportedSchema)
	}
	for name, t := range r.Tools {
		if t.Repo == "" || len(t.Executables) == 0 || len(t.Assets) == 0 {
			return r, fmt.Errorf("registry tool %q is missing repo, executables or assets", name)
		}
	}
	return r, nil
}

// ParseFile decodes and validates a registry file on disk.
func ParseFile(path string) (Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Registry{}, err
	}
	return Parse(data)
}

// Embedded returns the registry snapshot bundled into the binary.
func Embedded() Registry {
	r, err := Parse(dem.RegistryYAML)
	if err != nil {
		// the embedded registry is validated by tests; failing to
		// parse it is a build defect, not a runtime condition
		panic("embedded registry.yaml is invalid: " + err.Error())
	}
	return r
}

// LoadLocal returns the best registry available without touching the
// network: the cached fetched copy when valid, otherwise the embedded
// snapshot.
func LoadLocal(cacheDir string) Registry {
	if r, err := ParseFile(filepath.Join(cacheDir, CacheFileName)); err == nil {
		return r
	}
	return Embedded()
}

// CacheFresh reports whether the cached copy exists and is younger
// than CacheTTL.
func CacheFresh(cacheDir string) bool {
	info, err := os.Stat(filepath.Join(cacheDir, CacheFileName))
	return err == nil && time.Since(info.ModTime()) < CacheTTL
}

// TouchCache resets the cached copy's freshness window, used when a
// refresh attempt decided to keep the current copy.
func TouchCache(cacheDir string) {
	now := time.Now()
	os.Chtimes(filepath.Join(cacheDir, CacheFileName), now, now)
}

// ToolFor finds the registry tool that owns an executable.
func (r Registry) ToolFor(execName string) (string, bool) {
	for name, t := range r.Tools {
		for _, e := range t.Executables {
			if e == execName {
				return name, true
			}
		}
	}
	return "", false
}

// AssetFor resolves the asset name for the current platform and the
// given version, expanding the {version} placeholder.
func (t Tool) AssetFor(version string) (string, error) {
	key := runtime.GOOS + "-" + runtime.GOARCH
	tpl, ok := t.Assets[key]
	if !ok {
		return "", fmt.Errorf("no asset declared for %s", key)
	}
	return strings.ReplaceAll(tpl, "{version}", version), nil
}
