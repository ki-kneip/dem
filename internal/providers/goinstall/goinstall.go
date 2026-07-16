// Package goinstall is the generic provider for registry tools built
// locally with the Go toolchain (type: go-install in registry.yaml):
// CLIs that are not distributed as GitHub Release binaries, e.g.
// Wails.
//
// Every build runs with its own GOBIN/GOMODCACHE/GOCACHE, wiped as
// soon as the resulting binary is extracted. This trades rebuild
// speed (every install downloads its dependency graph from scratch)
// for never leaving module or build cache bloat behind for a tool
// the user only needed once.
package goinstall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/mod/semver"

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
}

// moduleMajorRe extracts the major version segment from a Go module
// path using the vN import path convention, e.g.
// ".../wails/v2/cmd/wails" -> "2".
var moduleMajorRe = regexp.MustCompile(`/v(\d+)/`)

// fetch lists the repo's stable releases (no drafts/prereleases)
// whose tag is valid semver and, when the module path carries a
// major version suffix, matches it. This keeps a monorepo's
// unrelated tags (other majors, submodules) out of the version list.
func (p *Provider) fetch(ctx context.Context) ([]ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=100", p.spec.Repo)
	var all []ghRelease
	if err := httpx.GetJSON(ctx, url, &all); err != nil {
		return nil, fmt.Errorf("querying %s releases: %w", p.spec.Repo, err)
	}

	wantMajor, hasMajor := "", false
	if m := moduleMajorRe.FindStringSubmatch(p.spec.Package); m != nil {
		wantMajor, hasMajor = "v"+m[1], true
	}

	filtered := all[:0]
	for _, r := range all {
		if r.Prerelease || r.Draft {
			continue
		}
		tag := "v" + strings.TrimPrefix(r.TagName, "v")
		if !semver.IsValid(tag) {
			continue
		}
		if hasMajor && semver.Major(tag) != wantMajor {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered, nil
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
	core.SortVersionsDesc(versionRaws(versions))
	return versions, nil
}

func versionRaws(versions []core.Version) []string {
	raws := make([]string, len(versions))
	for i, v := range versions {
		raws[i] = v.Raw
	}
	return raws
}

func (p *Provider) Resolve(ctx context.Context, spec string) (core.Version, error) {
	versions, err := p.ListRemote(ctx)
	if err != nil {
		return core.Version{}, err
	}
	return core.MatchSpec(strings.TrimPrefix(spec, "v"), versions)
}

// requireRe parses a single Requires entry, e.g. "go>=1.21".
var requireRe = regexp.MustCompile(`^([a-zA-Z0-9_-]+)>=(.+)$`)

// checkRequires validates the declared dependencies are met before
// spending time on a build, and returns the go binary the build
// should use. Only "<tool>>=<version>" is supported today, and only
// "go" is a recognized dependency.
//
// It prefers a go dem itself manages (the active "dem install go"
// version) over whatever "go" resolves to on PATH, so the build the
// user gets matches what "dem install go" would give them elsewhere.
// A PATH go that satisfies the requirement is still accepted (with a
// warning) rather than forcing a redundant "dem install go" when the
// user already has a suitable toolchain; no go anywhere satisfying
// the requirement is a hard failure — the tool is not installed.
func (p *Provider) checkRequires(ctx context.Context) (goBin string, err error) {
	for _, req := range p.spec.Requires {
		m := requireRe.FindStringSubmatch(req)
		if m == nil {
			return "", fmt.Errorf("%s: malformed requirement %q (expected \"tool>=version\")", p.name, req)
		}
		tool, want := m[1], m[2]
		if tool != "go" {
			return "", fmt.Errorf("%s: unsupported requirement %q (only \"go\" is currently supported)", p.name, req)
		}
		bin, demManaged, err := resolveGo(ctx, want)
		if err != nil {
			return "", fmt.Errorf("%s requires go>=%s: %w", p.name, want, err)
		}
		if !demManaged {
			fmt.Fprintf(os.Stderr, "warning: %s is building with go at %s, which dem does not manage; consider 'dem install go@%s' for a reproducible build\n", p.name, bin, want)
		}
		goBin = bin
	}
	return goBin, nil
}

// resolveGo finds a go binary satisfying "go>=want", preferring the
// one dem itself has installed and made active over whatever "go"
// resolves to on the system PATH. Returns an error only when neither
// source has a go new enough (or a go at all).
func resolveGo(ctx context.Context, want string) (bin string, demManaged bool, err error) {
	if demBin, ok := demGo(); ok {
		if err := checkGoVersionAt(ctx, demBin, want); err == nil {
			return demBin, true, nil
		}
	}
	if pathBin, lookErr := exec.LookPath("go"); lookErr == nil {
		if err := checkGoVersionAt(ctx, pathBin, want); err == nil {
			return pathBin, false, nil
		}
	}
	return "", false, fmt.Errorf("no go>=%s found (checked dem's own go install and PATH); install it with 'dem install go@%s'", want, want)
}

// demGo returns the go binary for dem's own active "go" version, if
// one is configured and actually installed.
func demGo() (bin string, ok bool) {
	paths, err := core.ResolvePaths()
	if err != nil {
		return "", false
	}
	cfg, err := core.LoadConfig(paths.ConfigFile())
	if err != nil {
		return "", false
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	version, _, err := core.ResolveActive("go", cwd, cfg)
	if err != nil || !paths.IsInstalled("go", version) {
		return "", false
	}
	return core.FindExecutable(paths.InstallDir("go", version), "go")
}

// goVersionRe pulls the leading X.Y[.Z] out of a "go version" token,
// discarding any build suffix some toolchains append (e.g. custom
// builds reporting "go1.26.5-someflag" instead of a plain "go1.26.5").
var goVersionRe = regexp.MustCompile(`^(\d+\.\d+(?:\.\d+)?)`)

func checkGoVersionAt(ctx context.Context, goBin, want string) error {
	out, err := exec.CommandContext(ctx, goBin, "version").Output()
	if err != nil {
		return fmt.Errorf("running %s version: %w", goBin, err)
	}
	var got string
	for _, f := range strings.Fields(string(out)) {
		if !strings.HasPrefix(f, "go") || len(f) <= 2 || f[2] < '0' || f[2] > '9' {
			continue
		}
		if m := goVersionRe.FindStringSubmatch(strings.TrimPrefix(f, "go")); m != nil {
			got = m[1]
			break
		}
	}
	if got == "" {
		return fmt.Errorf("could not parse 'go version' output: %q", strings.TrimSpace(string(out)))
	}
	if semver.Compare("v"+got, "v"+want) < 0 {
		return fmt.Errorf("found go %s, need >= %s", got, want)
	}
	return nil
}

// Download builds the tool with `go install` in an isolated,
// throwaway GOBIN/GOMODCACHE/GOCACHE, then extracts the resulting
// binary into cacheDir. The build environment is removed as soon as
// the binary is copied out, so no module or build cache is left
// behind on the host.
func (p *Provider) Download(ctx context.Context, v core.Version, cacheDir string) (core.Artifact, error) {
	goBin, err := p.checkRequires(ctx)
	if err != nil {
		return core.Artifact{}, err
	}
	if goBin == "" {
		goBin = "go" // no "go>=..." requirement declared; fall back to PATH
	}

	buildDir, err := os.MkdirTemp(cacheDir, "goinstall-"+p.name+"-*")
	if err != nil {
		return core.Artifact{}, err
	}
	defer removeBuildDir(buildDir)

	gobin := filepath.Join(buildDir, "bin")
	target := p.spec.Package + "@v" + v.Raw

	cmd := exec.CommandContext(ctx, goBin, "install", target)
	cmd.Env = append(os.Environ(),
		"GOBIN="+gobin,
		"GOMODCACHE="+filepath.Join(buildDir, "mod"),
		"GOCACHE="+filepath.Join(buildDir, "cache"),
	)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return core.Artifact{}, fmt.Errorf("go install %s: %w", target, err)
	}

	p.runPostInstall(ctx)

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	binName := p.spec.Executables[0]
	built := filepath.Join(gobin, binName+ext)
	if _, err := os.Stat(built); err != nil {
		return core.Artifact{}, fmt.Errorf("go install %s did not produce %s", target, binName+ext)
	}

	dest := filepath.Join(cacheDir, fmt.Sprintf("%s-%s%s", p.name, v.Raw, ext))
	if err := copyExecutable(built, dest); err != nil {
		return core.Artifact{}, err
	}
	return core.Artifact{Path: dest}, nil
}

// runPostInstall runs each declared hook best-effort: a cleanup step
// failing should not undo an otherwise successful build.
func (p *Provider) runPostInstall(ctx context.Context) {
	for _, raw := range p.spec.PostInstall {
		fields := strings.Fields(raw)
		if len(fields) == 0 {
			continue
		}
		cmd := exec.CommandContext(ctx, fields[0], fields[1:]...)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s post-install step %q failed: %v\n", p.name, raw, err)
		}
	}
}

// Install copies the built artifact into dir/bin under each declared
// executable name.
func (p *Provider) Install(a core.Artifact, dir string) error {
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

// removeBuildDir deletes the throwaway build environment. GOMODCACHE
// entries are written read-only by the go tool (to guard against
// accidental edits), so a plain RemoveAll silently leaves them behind
// on most platforms; restore write permission first.
func removeBuildDir(dir string) {
	filepath.WalkDir(dir, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		os.Chmod(path, 0o755)
		return nil
	})
	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not remove temporary build dir %s: %v\n", dir, err)
	}
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
	if _, err := out.ReadFrom(in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
