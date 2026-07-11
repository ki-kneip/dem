// Package httpx centralizes DEM's HTTP access: version index JSON
// and downloads with progress and checksum verification.
package httpx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"golang.org/x/term"
)

// ShowProgress can be disabled globally (--plain flag).
var ShowProgress = true

var client = &http.Client{Timeout: 30 * time.Minute}

// GetJSON fetches url and decodes the body into v.
func GetJSON(ctx context.Context, url string, v any) error {
	body, err := Get(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close()
	return json.NewDecoder(body).Decode(v)
}

// Get returns the body of url; the caller closes it.
func Get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp.Body, nil
}

// Download fetches url into dest with a progress bar (when there is
// a TTY) and validates the sha256 when sha256Hex != "". If dest
// already exists with the expected checksum, the cache is reused
// without downloading again.
func Download(ctx context.Context, url, dest, sha256Hex, label string) error {
	if sha256Hex != "" && fileMatches(dest, sha256Hex) {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download of %s failed: %s", url, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	h := sha256.New()
	writers := []io.Writer{f, h}
	if ShowProgress && term.IsTerminal(int(os.Stderr.Fd())) {
		bar := progressbar.DefaultBytes(resp.ContentLength, label)
		writers = append(writers, bar)
	}
	_, copyErr := io.Copy(io.MultiWriter(writers...), resp.Body)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		os.Remove(tmp)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}

	if sha256Hex != "" {
		got := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(got, sha256Hex) {
			os.Remove(tmp)
			return fmt.Errorf("checksum mismatch for %s (expected %s, got %s)", filepath.Base(dest), sha256Hex, got)
		}
	}

	os.Remove(dest) // on Windows, rename does not overwrite
	return os.Rename(tmp, dest)
}

func fileMatches(path, sha256Hex string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), sha256Hex)
}
