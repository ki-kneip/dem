// Package golang installs the Go toolchain from go.dev/dl.
package golang

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ki-kneip/dem/internal/archive"
	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/httpx"
	"github.com/ki-kneip/dem/internal/toolmeta"
)

const indexURL = "https://go.dev/dl/?mode=json&include=all"

type Provider struct{}

func New() *Provider { return &Provider{} }

func (*Provider) Name() string { return "go" }

func (*Provider) Executables() []string { return toolmeta.Executables["go"] }

func (*Provider) EnvVars(dir string) map[string]string { return toolmeta.EnvVars("go", dir) }

type release struct {
	Version string `json:"version"` // "go1.22.5"
	Stable  bool   `json:"stable"`
	Files   []file `json:"files"`
}

type file struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	SHA256   string `json:"sha256"`
	Kind     string `json:"kind"`
}

func (p *Provider) ListRemote(ctx context.Context) ([]core.Version, error) {
	releases, err := p.fetch(ctx)
	if err != nil {
		return nil, err
	}
	versions := make([]core.Version, 0, len(releases))
	for _, r := range releases {
		if r.Stable {
			versions = append(versions, core.Version{Raw: strings.TrimPrefix(r.Version, "go")})
		}
	}
	return versions, nil
}

func (p *Provider) Resolve(ctx context.Context, spec string) (core.Version, error) {
	versions, err := p.ListRemote(ctx)
	if err != nil {
		return core.Version{}, err
	}
	return core.MatchSpec(strings.TrimPrefix(spec, "go"), versions)
}

func (p *Provider) Download(ctx context.Context, v core.Version, cacheDir string) (core.Artifact, error) {
	releases, err := p.fetch(ctx)
	if err != nil {
		return core.Artifact{}, err
	}
	for _, r := range releases {
		if strings.TrimPrefix(r.Version, "go") != v.Raw {
			continue
		}
		for _, f := range r.Files {
			if f.OS == runtime.GOOS && f.Arch == runtime.GOARCH && f.Kind == "archive" {
				dest := filepath.Join(cacheDir, f.Filename)
				url := "https://go.dev/dl/" + f.Filename
				if err := httpx.Download(ctx, url, dest, f.SHA256, "go "+v.Raw); err != nil {
					return core.Artifact{}, err
				}
				return core.Artifact{Path: dest, Checksum: f.SHA256}, nil
			}
		}
		return core.Artifact{}, fmt.Errorf("go %s has no build for %s/%s", v.Raw, runtime.GOOS, runtime.GOARCH)
	}
	return core.Artifact{}, fmt.Errorf("go version %s not found in the index", v.Raw)
}

func (*Provider) Install(a core.Artifact, dir string) error {
	// the archive ships everything inside a top-level "go/" directory
	return archive.Extract(a.Path, dir, 1)
}

func (*Provider) fetch(ctx context.Context) ([]release, error) {
	var releases []release
	if err := httpx.GetJSON(ctx, indexURL, &releases); err != nil {
		return nil, fmt.Errorf("querying go.dev/dl: %w", err)
	}
	return releases, nil // the index is already ordered newest to oldest
}
