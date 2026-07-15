// Package update implements DEM's self-update from the project's
// GitHub Releases.
//
// Policy: update automatically only within the same major version
// (no breaking change). Crossing a major requires an explicit
// --allow-major.
package update

import (
	"bufio"
	"context"
	"crypto"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/minio/selfupdate"
	"golang.org/x/mod/semver"
)

const (
	repoOwner = "ki-kneip"
	repoName  = "dem"
)

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// Release is a published release with its assets (name -> URL).
type Release struct {
	Version string // normalized tag, e.g. "v1.2.3"
	Assets  map[string]string
}

type releasePayload struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Latest queries GitHub for the most recent release. GitHub's
// releases/latest endpoint (the default) never returns a prerelease,
// so self-update never picks one up on its own; pass
// includePrerelease to opt into testing one deliberately.
func Latest(ctx context.Context, includePrerelease bool) (*Release, error) {
	var payload releasePayload
	if includePrerelease {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=1", repoOwner, repoName)
		var list []releasePayload
		if err := fetchJSON(ctx, url, &list); err != nil {
			return nil, err
		}
		if len(list) == 0 {
			return nil, fmt.Errorf("no release published at %s/%s yet", repoOwner, repoName)
		}
		payload = list[0]
	} else {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
		if err := fetchJSON(ctx, url, &payload); err != nil {
			return nil, err
		}
	}

	rel := &Release{Version: normalize(payload.TagName), Assets: map[string]string{}}
	for _, a := range payload.Assets {
		rel.Assets[a.Name] = a.URL
	}
	if !semver.IsValid(rel.Version) {
		return nil, fmt.Errorf("release tag is not valid semver: %q", payload.TagName)
	}
	return rel, nil
}

func fetchJSON(ctx context.Context, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("no release published at %s/%s yet", repoOwner, repoName)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub answered %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// AssetName is the expected binary name for this platform, following
// the dem-<os>-<arch>[.exe] release convention.
func AssetName() string {
	name := fmt.Sprintf("dem-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// Guard decides whether the current -> latest update is allowed.
// Returns a descriptive error when it is not.
func Guard(current, latest string, allowMajor bool) error {
	current, latest = normalize(current), normalize(latest)
	if !semver.IsValid(current) {
		return fmt.Errorf("this is a development build (%s); self-update only works on release builds", current)
	}
	if !semver.IsValid(latest) {
		return fmt.Errorf("invalid remote version: %s", latest)
	}
	if semver.Compare(latest, current) <= 0 {
		return ErrUpToDate
	}
	if semver.Major(latest) != semver.Major(current) && !allowMajor {
		return fmt.Errorf("%s -> %s crosses a major version (breaking change); run with --allow-major to accept", current, latest)
	}
	return nil
}

// ErrUpToDate signals there is nothing to do.
var ErrUpToDate = fmt.Errorf("already at the latest version")

// Checksum fetches the asset's expected sha256 from the release's
// checksums.txt ("hash  name" format). Returns nil when the release
// publishes no checksums.
func (r *Release) Checksum(ctx context.Context, assetName string) ([]byte, error) {
	url, ok := r.Assets["checksums.txt"]
	if !ok {
		return nil, nil
	}
	body, err := get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	sc := bufio.NewScanner(body)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		// a leading "*" is sha256sum's binary-mode marker
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == assetName {
			return hex.DecodeString(fields[0])
		}
	}
	return nil, fmt.Errorf("checksums.txt has no entry for %s", assetName)
}

// Apply downloads the binary and atomically replaces the current
// executable (on Windows, the running exe is renamed before the swap).
func Apply(ctx context.Context, r *Release) error {
	name := AssetName()
	url, ok := r.Assets[name]
	if !ok {
		return fmt.Errorf("release %s has no asset %s for this platform", r.Version, name)
	}
	sum, err := r.Checksum(ctx, name)
	if err != nil {
		return err
	}
	body, err := get(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close()

	opts := selfupdate.Options{}
	if sum != nil {
		opts.Checksum = sum
		opts.Hash = crypto.SHA256
	}
	return selfupdate.Apply(body, opts)
}

func get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download of %s failed: %s", url, resp.Status)
	}
	return resp.Body, nil
}

// normalize guarantees the "v" prefix the semver package requires.
func normalize(v string) string {
	v = strings.TrimSpace(v)
	if v != "" && !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}
