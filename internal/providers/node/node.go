// Package node installs Node.js from nodejs.org/dist.
// npm and npx ship with the installation and get shims alongside.
package node

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ki-kneip/dem/internal/archive"
	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/httpx"
	"github.com/ki-kneip/dem/internal/toolmeta"
)

const distURL = "https://nodejs.org/dist"

type Provider struct{}

func New() *Provider { return &Provider{} }

func (*Provider) Name() string { return "node" }

func (*Provider) Executables() []string { return toolmeta.Executables["node"] }

func (*Provider) EnvVars(dir string) map[string]string { return toolmeta.EnvVars("node", dir) }

type release struct {
	Version string          `json:"version"` // "v22.5.1"
	LTS     json.RawMessage `json:"lts"`     // false or "Jod"
}

func (r release) isLTS() bool { return string(r.LTS) != "false" }

func (p *Provider) ListRemote(ctx context.Context) ([]core.Version, error) {
	var releases []release
	if err := httpx.GetJSON(ctx, distURL+"/index.json", &releases); err != nil {
		return nil, fmt.Errorf("querying nodejs.org: %w", err)
	}
	versions := make([]core.Version, 0, len(releases))
	for _, r := range releases {
		versions = append(versions, core.Version{
			Raw: strings.TrimPrefix(r.Version, "v"),
			LTS: r.isLTS(),
		})
	}
	return versions, nil // the index is already ordered newest to oldest
}

func (p *Provider) Resolve(ctx context.Context, spec string) (core.Version, error) {
	versions, err := p.ListRemote(ctx)
	if err != nil {
		return core.Version{}, err
	}
	return core.MatchSpec(strings.TrimPrefix(spec, "v"), versions)
}

func (p *Provider) Download(ctx context.Context, v core.Version, cacheDir string) (core.Artifact, error) {
	asset, err := assetName(v.Raw)
	if err != nil {
		return core.Artifact{}, err
	}
	base := distURL + "/v" + v.Raw

	sum, err := p.checksumFor(ctx, base, asset)
	if err != nil {
		return core.Artifact{}, err
	}

	dest := filepath.Join(cacheDir, asset)
	if err := httpx.Download(ctx, base+"/"+asset, dest, sum, "node "+v.Raw); err != nil {
		return core.Artifact{}, err
	}
	return core.Artifact{Path: dest, Checksum: sum}, nil
}

func (*Provider) Install(a core.Artifact, dir string) error {
	// the archive ships everything inside "node-vX.Y.Z-<platform>/"
	return archive.Extract(a.Path, dir, 1)
}

// checksumFor reads the version's official SHASUMS256.txt.
func (*Provider) checksumFor(ctx context.Context, baseURL, asset string) (string, error) {
	body, err := httpx.Get(ctx, baseURL+"/SHASUMS256.txt")
	if err != nil {
		return "", err
	}
	defer body.Close()
	sc := bufio.NewScanner(body)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("SHASUMS256.txt has no entry for %s", asset)
}

func assetName(version string) (string, error) {
	arch, ok := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("architecture %s is not supported by Node", runtime.GOARCH)
	}
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("node-v%s-win-%s.zip", version, arch), nil
	case "linux":
		return fmt.Sprintf("node-v%s-linux-%s.tar.gz", version, arch), nil
	case "darwin":
		return fmt.Sprintf("node-v%s-darwin-%s.tar.gz", version, arch), nil
	default:
		return "", fmt.Errorf("platform %s is not supported by Node", runtime.GOOS)
	}
}
