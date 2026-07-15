package github

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/registry"
)

func TestInstallRawBinaryRealizesAliases(t *testing.T) {
	dir := t.TempDir()
	asset := filepath.Join(dir, "pnpm-linux-x64")
	if err := os.WriteFile(asset, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	p := New("pnpm", registry.Tool{
		Executables: []string{"pnpm", "pnpx"},
		Aliases:     map[string]string{"pnpx": "pnpm"},
	})
	installDir := t.TempDir()
	if err := p.Install(core.Artifact{Path: asset}, installDir); err != nil {
		t.Fatal(err)
	}

	if _, ok := core.FindExecutable(installDir, "pnpm"); !ok {
		t.Fatal("pnpm was not installed")
	}
	if _, ok := core.FindExecutable(installDir, "pnpx"); !ok {
		t.Fatal("pnpx alias was not realized")
	}
}

func TestInstallArchiveRealizesAliases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture below is a .tar.gz; windows assets are .zip and not exercised here")
	}

	dir := t.TempDir()
	archivePath := filepath.Join(dir, "pnpm-linux-x64.tar.gz")
	writeTarGz(t, archivePath, map[string]string{"pnpm": "binary"})

	p := New("pnpm", registry.Tool{
		Executables: []string{"pnpm", "pnpx"},
		Aliases:     map[string]string{"pnpx": "pnpm"},
	})
	installDir := t.TempDir()
	if err := p.Install(core.Artifact{Path: archivePath}, installDir); err != nil {
		t.Fatal(err)
	}

	if _, ok := core.FindExecutable(installDir, "pnpm"); !ok {
		t.Fatal("pnpm was not extracted")
	}
	pnpx, ok := core.FindExecutable(installDir, "pnpx")
	if !ok {
		t.Fatal("pnpx alias was not realized after archive extraction")
	}
	got, err := os.ReadFile(pnpx)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "binary" {
		t.Fatalf("alias content = %q, want %q", got, "binary")
	}
}

func TestInstallMissingAliasTargetFails(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "tool.tar.gz")
	writeTarGz(t, archivePath, map[string]string{"other": "binary"})

	p := New("pnpm", registry.Tool{
		Executables: []string{"pnpm", "pnpx"},
		Aliases:     map[string]string{"pnpx": "pnpm"},
	})
	if err := p.Install(core.Artifact{Path: archivePath}, t.TempDir()); err == nil {
		t.Fatal("expected an error when the alias target was never installed")
	}
}

func writeTarGz(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}
