// Package providers registers the tools DEM knows how to manage:
// the built-in runtime providers plus the registry-driven family of
// GitHub-release tools.
package providers

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/httpx"
	"github.com/ki-kneip/dem/internal/providers/github"
	"github.com/ki-kneip/dem/internal/providers/golang"
	"github.com/ki-kneip/dem/internal/providers/java"
	"github.com/ki-kneip/dem/internal/providers/node"
	"github.com/ki-kneip/dem/internal/registry"
)

// registryURL is where the freshest registry lives; fetched copies
// are cached and only adopted when their schema is supported.
const registryURL = "https://raw.githubusercontent.com/ki-kneip/dem/main/registry.yaml"

const fetchTimeout = 10 * time.Second

var (
	loadOnce sync.Once
	all      map[string]core.Provider
)

// Get returns the provider for a tool, if it exists.
func Get(tool string) (core.Provider, bool) {
	load()
	p, ok := all[tool]
	return p, ok
}

// Names lists the registered tools in alphabetical order.
func Names() []string {
	load()
	names := make([]string, 0, len(all))
	for n := range all {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func load() {
	loadOnce.Do(func() {
		all = map[string]core.Provider{}
		for _, p := range []core.Provider{golang.New(), node.New(), java.New()} {
			all[p.Name()] = p
		}
		reg := loadRegistry()
		for name, spec := range reg.Tools {
			if _, exists := all[name]; exists {
				continue // built-in providers win over registry entries
			}
			all[name] = github.New(name, spec)
		}
	})
}

// loadRegistry returns the freshest usable registry: the cached copy
// while it is fresh, otherwise it tries to refresh from the
// repository (silently keeping the local copy on any failure or when
// the remote schema is not supported by this build).
func loadRegistry() registry.Registry {
	paths, err := core.ResolvePaths()
	if err != nil {
		return registry.Embedded()
	}
	if !registry.CacheFresh(paths.Cache) {
		refreshRegistry(paths.Cache)
	}
	return registry.LoadLocal(paths.Cache)
}

func refreshRegistry(cacheDir string) {
	ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
	defer cancel()
	body, err := httpx.Get(ctx, registryURL)
	if err != nil {
		return
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return
	}
	if _, err := registry.Parse(data); err != nil {
		// unsupported schema or malformed: keep what we have, and
		// reset the TTL so every command does not retry the fetch
		registry.TouchCache(cacheDir)
		return
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return
	}
	os.WriteFile(filepath.Join(cacheDir, registry.CacheFileName), data, 0o644)
}
