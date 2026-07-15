// Package java installs JDKs from the foojay Disco API
// (https://api.foojay.io), which aggregates Temurin, Corretto, Zulu,
// GraalVM, Liberica and other OpenJDK distributions behind one
// normalized API.
package java

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/ki-kneip/dem/internal/archive"
	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/httpx"
	"github.com/ki-kneip/dem/internal/toolmeta"
)

const (
	apiBase       = "https://api.foojay.io/disco/v3.0"
	defaultVendor = "temurin"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (*Provider) Name() string { return "java" }

func (*Provider) Executables() []string { return toolmeta.Executables["java"] }

func (*Provider) EnvVars(dir string) map[string]string { return toolmeta.EnvVars("java", dir) }

type pkg struct {
	ID              string `json:"id"`
	Distribution    string `json:"distribution"`
	MajorVersion    int    `json:"major_version"`
	JavaVersion     string `json:"java_version"`
	TermOfSupport   string `json:"term_of_support"`
	OperatingSystem string `json:"operating_system"`
	Architecture    string `json:"architecture"`
	ArchiveType     string `json:"archive_type"`
}

type packagesResponse struct {
	Result []pkg `json:"result"`
}

type idDetail struct {
	DirectDownloadURI string `json:"direct_download_uri"`
	Checksum          string `json:"checksum"`
	ChecksumType      string `json:"checksum_type"`
}

type idResponse struct {
	Result []idDetail `json:"result"`
}

// vendorPrefix matches a bare, alphabetic-only vendor token before the
// first "-" (e.g. "corretto" in "corretto-21"), so that EA specs like
// "21-ea" (numeric left part) are not mistaken for a vendor prefix.
var vendorPrefix = regexp.MustCompile(`^[A-Za-z_]+$`)

// splitVendorSpec splits a spec like "corretto-21" into ("corretto",
// "21"); a spec with no recognized vendor prefix, like "21" or
// "21-ea", defaults to defaultVendor.
func splitVendorSpec(spec string) (vendor, versionSpec string) {
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) == 2 && vendorPrefix.MatchString(parts[0]) {
		return strings.ToLower(parts[0]), parts[1]
	}
	return defaultVendor, spec
}

// ltsMajorsPerVendor is how many of a vendor's most recent LTS major
// lines ListRemote surfaces (e.g. 25, 21, 17), so a user can pick
// between the current and previous LTS lines instead of only ever
// seeing the newest one.
const ltsMajorsPerVendor = 3

// ListRemote returns each vendor's most recent LTS major lines,
// grouped by vendor: the overview a "dem list java --remote" user
// wants. Disco's own "latest=available" does not guarantee one result
// per distro (it is dominated by whichever vendor has the most GA
// builds, Zulu in practice) and includes non-LTS releases, so both
// the LTS filter and the per-vendor grouping are applied here rather
// than trusted to the API response.
func (p *Provider) ListRemote(ctx context.Context) ([]core.Version, error) {
	q := url.Values{
		"operating_system": {discoOS()},
		"architecture":     {discoArch()},
		"archive_type":     {discoArchiveType()},
		"package_type":     {"jdk"},
		"release_status":   {"ga"},
		"latest":           {"available"},
		"term_of_support":  {"lts"},
		"javafx_bundled":   {"false"},
	}
	pkgs, err := p.fetchPackages(ctx, q)
	if err != nil {
		return nil, err
	}
	return groupLTSByVendor(pkgs, ltsMajorsPerVendor), nil
}

// groupLTSByVendor collapses packages into up to limit entries per
// vendor — its most recent LTS major lines, newest first — grouped in
// vendor-alphabetical order. Disco's "latest=available" scoped to
// term_of_support=lts already returns roughly one build per (vendor,
// major); it still doubles up when a distro ships multiple
// libc/packaging variants of the same release (e.g. glibc vs musl on
// Linux), so the first one seen per (vendor, major) is kept.
func groupLTSByVendor(pkgs []pkg, limit int) []core.Version {
	type build struct {
		major   int
		version string
	}
	var vendors []string
	byVendor := make(map[string][]build)
	seen := make(map[string]bool, len(pkgs))
	for _, pk := range pkgs {
		key := pk.Distribution + "|" + strconv.Itoa(pk.MajorVersion)
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, ok := byVendor[pk.Distribution]; !ok {
			vendors = append(vendors, pk.Distribution)
		}
		byVendor[pk.Distribution] = append(byVendor[pk.Distribution], build{pk.MajorVersion, pk.JavaVersion})
	}
	sort.Strings(vendors)

	versions := make([]core.Version, 0, len(vendors)*limit)
	for _, vendor := range vendors {
		builds := byVendor[vendor]
		sort.Slice(builds, func(i, j int) bool { return builds[i].major > builds[j].major })
		if len(builds) > limit {
			builds = builds[:limit]
		}
		for _, b := range builds {
			versions = append(versions, core.Version{
				Raw:   vendor + "-" + b.version,
				LTS:   true,
				Group: vendor,
			})
		}
	}
	return versions
}

func (p *Provider) Resolve(ctx context.Context, spec string) (core.Version, error) {
	vendor, versionSpec := splitVendorSpec(spec)
	q := url.Values{
		"distro":           {vendor},
		"operating_system": {discoOS()},
		"architecture":     {discoArch()},
		"archive_type":     {discoArchiveType()},
		"package_type":     {"jdk"},
		"release_status":   {"ga"},
		"latest":           {"available"},
	}
	pkgs, err := p.fetchPackages(ctx, q)
	if err != nil {
		return core.Version{}, err
	}
	if len(pkgs) == 0 {
		return core.Version{}, fmt.Errorf("no %s builds found for %s/%s", vendor, discoOS(), discoArch())
	}
	plain := make([]core.Version, 0, len(pkgs))
	for _, pk := range pkgs {
		plain = append(plain, core.Version{Raw: pk.JavaVersion, LTS: pk.TermOfSupport == "lts"})
	}
	v, err := core.MatchSpec(versionSpec, plain)
	if err != nil {
		return core.Version{}, fmt.Errorf("java %s: %w", vendor, err)
	}
	return core.Version{Raw: vendor + "-" + v.Raw, LTS: v.LTS}, nil
}

func (p *Provider) Download(ctx context.Context, v core.Version, cacheDir string) (core.Artifact, error) {
	vendor, javaVersion, ok := strings.Cut(v.Raw, "-")
	if !ok {
		return core.Artifact{}, fmt.Errorf("invalid java version %q", v.Raw)
	}
	q := url.Values{
		"distro":           {vendor},
		"version":          {javaVersion},
		"operating_system": {discoOS()},
		"architecture":     {discoArch()},
		"archive_type":     {discoArchiveType()},
		"package_type":     {"jdk"},
		"latest":           {"available"},
	}
	pkgs, err := p.fetchPackages(ctx, q)
	if err != nil {
		return core.Artifact{}, err
	}
	if len(pkgs) == 0 {
		return core.Artifact{}, fmt.Errorf("java %s has no build for %s/%s", v.Raw, discoOS(), discoArch())
	}

	var detail idResponse
	if err := httpx.GetJSON(ctx, apiBase+"/ids/"+pkgs[0].ID, &detail); err != nil {
		return core.Artifact{}, fmt.Errorf("querying foojay package %s: %w", pkgs[0].ID, err)
	}
	if len(detail.Result) == 0 || detail.Result[0].DirectDownloadURI == "" {
		return core.Artifact{}, fmt.Errorf("java %s: no download URI returned by foojay", v.Raw)
	}
	d := detail.Result[0]

	sum := ""
	if strings.EqualFold(d.ChecksumType, "sha256") {
		sum = d.Checksum
	}

	dest := filepath.Join(cacheDir, filepath.Base(d.DirectDownloadURI))
	if err := httpx.Download(ctx, d.DirectDownloadURI, dest, sum, "java "+v.Raw); err != nil {
		return core.Artifact{}, err
	}
	return core.Artifact{Path: dest, Checksum: sum}, nil
}

func (*Provider) Install(a core.Artifact, dir string) error {
	// Disco archives ship everything inside one top-level directory.
	if err := archive.Extract(a.Path, dir, 1); err != nil {
		return err
	}
	// macOS builds nest the real JDK home under Contents/Home; flatten
	// it so JAVA_HOME and core.FindExecutable's dir/dir-bin lookup work
	// the same way on every OS.
	nested := filepath.Join(dir, "Contents", "Home")
	if info, err := os.Stat(nested); err == nil && info.IsDir() {
		return flatten(nested, dir)
	}
	return nil
}

// flatten moves everything in src up into dest, then removes src's
// now-empty ancestor (dest/Contents).
func flatten(src, dest string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.Rename(filepath.Join(src, e.Name()), filepath.Join(dest, e.Name())); err != nil {
			return err
		}
	}
	return os.RemoveAll(filepath.Join(dest, "Contents"))
}

func (*Provider) fetchPackages(ctx context.Context, q url.Values) ([]pkg, error) {
	var resp packagesResponse
	u := apiBase + "/packages?" + q.Encode()
	if err := httpx.GetJSON(ctx, u, &resp); err != nil {
		return nil, fmt.Errorf("querying foojay: %w", err)
	}
	return resp.Result, nil
}

func discoOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	default:
		return runtime.GOOS // "linux", "windows"
	}
}

func discoArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "arm64":
		return "aarch64"
	default:
		return runtime.GOARCH
	}
}

func discoArchiveType() string {
	if runtime.GOOS == "windows" {
		return "zip"
	}
	return "tar.gz"
}
