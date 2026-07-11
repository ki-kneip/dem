// Package github is the generic provider for registry tools: CLIs
// published as a single binary or archive on GitHub Releases,
// described declaratively in registry.yaml.
package github

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ki-kneip/dem/internal/archive"
	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/httpx"
	"github.com/ki-kneip/dem/internal/registry"
)

type Provider struct {
	name string
	spec registry.Tool
}

func New(name string, spec registry.Tool) *Provider {
	return &Provider{name: name, spec: spec}
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) Executables() []string { return p.spec.Executables }

func (*Provider) EnvVars(dir string) map[string]string { return nil }

type ghRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (p *Provider) ListRemote(ctx context.Context) ([]core.Version, error) {
	releases, err := p.fetch(ctx)
	if err != nil {
		return nil, err
	}
	versions := make([]core.Version, 0, len(releases))
	for _, r := range releases {
		versions = append(versions, core.Version{Raw: strings.TrimPrefix(r.TagName, "v")})
	}
	return versions, nil
}

func (p *Provider) Resolve(ctx context.Context, spec string) (core.Version, error) {
	versions, err := p.ListRemote(ctx)
	if err != nil {
		return core.Version{}, err
	}
	return core.MatchSpec(strings.TrimPrefix(spec, "v"), versions)
}

func (p *Provider) Download(ctx context.Context, v core.Version, cacheDir string) (core.Artifact, error) {
	releases, err := p.fetch(ctx)
	if err != nil {
		return core.Artifact{}, err
	}
	asset, err := p.spec.AssetFor(v.Raw)
	if err != nil {
		return core.Artifact{}, fmt.Errorf("%s %s: %w", p.name, v.Raw, err)
	}
	for _, r := range releases {
		if strings.TrimPrefix(r.TagName, "v") != v.Raw {
			continue
		}
		url := assetURL(r, asset)
		if url == "" {
			return core.Artifact{}, fmt.Errorf("%s %s has no asset %s", p.name, v.Raw, asset)
		}
		sum, err := p.checksumFor(ctx, r, asset)
		if err != nil {
			return core.Artifact{}, err
		}
		dest := filepath.Join(cacheDir, fmt.Sprintf("%s-%s-%s", p.name, v.Raw, asset))
		if err := httpx.Download(ctx, url, dest, sum, p.name+" "+v.Raw); err != nil {
			return core.Artifact{}, err
		}
		return core.Artifact{Path: dest, Checksum: sum}, nil
	}
	return core.Artifact{}, fmt.Errorf("%s version %s not found in the releases", p.name, v.Raw)
}

// Install extracts archive assets into dir, or installs a raw binary
// asset under each executable name in dir/bin.
func (p *Provider) Install(a core.Artifact, dir string) error {
	if strings.HasSuffix(a.Path, ".zip") || strings.HasSuffix(a.Path, ".tar.gz") || strings.HasSuffix(a.Path, ".tgz") {
		return archive.Extract(a.Path, dir, 0)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	for _, name := range p.spec.Executables {
		if err := copyExecutable(a.Path, filepath.Join(binDir, name+ext)); err != nil {
			return err
		}
	}
	return nil
}

// checksumFor reads the tool's declared sha256sum-format asset, when
// it has one.
func (p *Provider) checksumFor(ctx context.Context, r ghRelease, asset string) (string, error) {
	if p.spec.Checksums == "" {
		return "", nil
	}
	url := assetURL(r, p.spec.Checksums)
	if url == "" {
		return "", fmt.Errorf("%s release %s declares checksums asset %s, but it is missing", p.name, r.TagName, p.spec.Checksums)
	}
	body, err := httpx.Get(ctx, url)
	if err != nil {
		return "", err
	}
	defer body.Close()
	sc := bufio.NewScanner(body)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// a leading "*" is sha256sum's binary-mode marker
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			return fields[0], nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("reading %s: %w", p.spec.Checksums, err)
	}
	return "", fmt.Errorf("%s has no entry for %s", p.spec.Checksums, asset)
}

func (p *Provider) fetch(ctx context.Context) ([]ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=50", p.spec.Repo)
	var all []ghRelease
	if err := httpx.GetJSON(ctx, url, &all); err != nil {
		return nil, fmt.Errorf("querying %s releases: %w", p.spec.Repo, err)
	}
	stable := all[:0]
	for _, r := range all {
		if !r.Prerelease && !r.Draft {
			stable = append(stable, r)
		}
	}
	return stable, nil
}

func assetURL(r ghRelease, name string) string {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

func copyExecutable(src, dst string) error {
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
