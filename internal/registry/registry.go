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

// Tool describes one registry-managed tool: either a GitHub-release
// binary (the default) or a go-install tool, compiled locally with
// the Go toolchain.
type Tool struct {
	Repo        string            `yaml:"repo"`
	Executables []string          `yaml:"executables"`
	Assets      map[string]string `yaml:"assets"`
	// Checksums is the optional name of a sha256sum-format asset used
	// to verify downloads.
	Checksums string `yaml:"checksums"`

	// Type selects the provider: "" or "binary" (default, a GitHub
	// Release asset) or "go-install" (built locally via
	// `go install <package>@<version>`).
	Type string `yaml:"type,omitempty"`
	// Package is the Go module path installed for type: go-install
	// (e.g. github.com/wailsapp/wails/v2/cmd/wails).
	Package string `yaml:"package,omitempty"`
	// Requires lists dependencies the provider checks before
	// building, e.g. "go>=1.21".
	Requires []string `yaml:"requires,omitempty"`
	// PostInstall are commands run (each split on whitespace, no
	// shell) after a successful build, e.g. to trim caches the build
	// leaves behind.
	PostInstall []string `yaml:"postInstall,omitempty"`
}

// TypeBinary and TypeGoInstall are the recognized values of Type.
const (
	TypeBinary    = "binary"
	TypeGoInstall = "go-install"
)

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
		switch t.Type {
		case "", TypeBinary:
			if t.Repo == "" || len(t.Executables) == 0 || len(t.Assets) == 0 {
				return r, fmt.Errorf("registry tool %q is missing repo, executables or assets", name)
			}
		case TypeGoInstall:
			if t.Package == "" || len(t.Executables) == 0 {
				return r, fmt.Errorf("registry tool %q is missing package or executables", name)
			}
		default:
			return r, fmt.Errorf("registry tool %q has unknown type %q", name, t.Type)
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
