// Package archive extracts the formats used by the tools' official
// sources: .zip (Windows) and .tar.gz (Linux/macOS).
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Extract unpacks src into dest, dropping the first strip path
// components (e.g. strip=1 removes the top-level "go/" or
// "node-v22.5.1-win-x64/" directory tarballs ship with).
func Extract(src, dest string, strip int) error {
	switch {
	case strings.HasSuffix(src, ".zip"):
		return extractZip(src, dest, strip)
	case strings.HasSuffix(src, ".tar.gz"), strings.HasSuffix(src, ".tgz"):
		return extractTarGz(src, dest, strip)
	default:
		return fmt.Errorf("unsupported archive format: %s", filepath.Base(src))
	}
}

func extractZip(src, dest string, strip int) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		rel, ok := stripPath(f.Name, strip)
		if !ok {
			continue
		}
		target := filepath.Join(dest, rel)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeFile(target, rc, f.Mode())
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(src, dest string, strip int) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		rel, ok := stripPath(hdr.Name, strip)
		if !ok {
			continue
		}
		target := filepath.Join(dest, rel)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := writeFile(target, tr, hdr.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// relative links inside tarballs (e.g. bin/npm -> ../lib/...)
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
}

// stripPath normalizes, guards against path traversal and drops the
// first strip components. Returns ok=false for entries that should
// be skipped.
func stripPath(name string, strip int) (string, bool) {
	name = path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == "/" || strings.HasPrefix(name, "../") || path.IsAbs(name) {
		return "", false
	}
	parts := strings.Split(name, "/")
	if len(parts) <= strip {
		return "", false
	}
	return filepath.Join(parts[strip:]...), true
}

func writeFile(target string, r io.Reader, mode os.FileMode) error {
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm()|0o200)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
