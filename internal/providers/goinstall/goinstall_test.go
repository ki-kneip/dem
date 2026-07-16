package goinstall

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ki-kneip/dem/internal/core"
	"github.com/ki-kneip/dem/internal/registry"
)

// TestMain supports a re-exec trick: when GO_WANT_FAKE_GO_VERSION is
// set, the compiled test binary itself prints a fake "go version"
// line and exits instead of running tests. That lets tests place a
// copy of this same binary on disk as a stand-in "go" executable
// (dem-managed or not) without needing a real second Go toolchain,
// and it works identically on every OS the CI matrix runs.
func TestMain(m *testing.M) {
	if v := os.Getenv("GO_WANT_FAKE_GO_VERSION"); v != "" {
		fmt.Printf("go version go%s %s/%s\n", v, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// writeFakeGo copies the running test binary to dst and arranges for
// it to behave like `go version <fakeVersion>` when GO_WANT_FAKE_GO_VERSION
// is exported before it runs (the caller sets that via t.Setenv).
func writeFakeGo(t *testing.T, dst string) {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(self)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func goExeName() string {
	if runtime.GOOS == "windows" {
		return "go.exe"
	}
	return "go"
}

// isolatedDemHome points DEM_HOME at a fresh temp dir so tests never
// touch a real ~/.dem, and returns its Paths.
func isolatedDemHome(t *testing.T) core.Paths {
	t.Helper()
	home := t.TempDir()
	t.Setenv("DEM_HOME", home)
	paths := core.PathsAt(home)
	if err := paths.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	return paths
}

// installFakeDemGo makes "go@version" look installed and active under
// the isolated DEM_HOME, backed by the fake-go re-exec trick.
func installFakeDemGo(t *testing.T, paths core.Paths, version, fakeVersion string) {
	t.Helper()
	t.Setenv("GO_WANT_FAKE_GO_VERSION", fakeVersion)
	writeFakeGo(t, filepath.Join(paths.InstallDir("go", version), "bin", goExeName()))
	cfg := core.Config{Global: map[string]string{"go": version}}
	if err := cfg.Save(paths.ConfigFile()); err != nil {
		t.Fatal(err)
	}
}

func TestModuleMajorRe(t *testing.T) {
	cases := []struct {
		pkg       string
		wantMajor string
		wantOK    bool
	}{
		{"github.com/wailsapp/wails/v2/cmd/wails", "2", true},
		{"github.com/wailsapp/wails/v3/cmd/wails3", "3", true},
		{"github.com/some/tool/cmd/tool", "", false},
	}
	for _, c := range cases {
		m := moduleMajorRe.FindStringSubmatch(c.pkg)
		if c.wantOK && (m == nil || m[1] != c.wantMajor) {
			t.Errorf("moduleMajorRe(%q) = %v, want major %q", c.pkg, m, c.wantMajor)
		}
		if !c.wantOK && m != nil {
			t.Errorf("moduleMajorRe(%q) = %v, want no match", c.pkg, m)
		}
	}
}

func TestCheckRequiresRejectsMalformed(t *testing.T) {
	p := New("wails", registry.Tool{Requires: []string{"not-a-requirement"}})
	if _, err := p.checkRequires(context.Background()); err == nil {
		t.Fatal("expected an error for a malformed requirement")
	}
}

func TestCheckRequiresRejectsUnsupportedTool(t *testing.T) {
	p := New("wails", registry.Tool{Requires: []string{"node>=20"}})
	if _, err := p.checkRequires(context.Background()); err == nil {
		t.Fatal("expected an error for an unsupported dependency")
	}
}

func TestCheckGoVersionAtParsesOutput(t *testing.T) {
	// go is required to build dem itself, so it is always on PATH in
	// this test environment; assert against a version we are certain
	// predates the running toolchain.
	if err := checkGoVersionAt(context.Background(), "go", "1.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckGoVersionAtRejectsTooNew(t *testing.T) {
	if err := checkGoVersionAt(context.Background(), "go", "99.0"); err == nil {
		t.Fatal("expected an error for an unreasonably high required version")
	}
}

func TestResolveGoPrefersDemManagedOverPATH(t *testing.T) {
	paths := isolatedDemHome(t)
	installFakeDemGo(t, paths, "1.99.0", "1.99.0")

	bin, demManaged, err := resolveGo(context.Background(), "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !demManaged {
		t.Fatalf("expected the dem-managed go to be preferred, got demManaged=false, bin=%q", bin)
	}
	want := filepath.Join(paths.InstallDir("go", "1.99.0"), "bin", goExeName())
	if bin != want {
		t.Fatalf("bin = %q, want %q", bin, want)
	}
}

func TestResolveGoFallsBackToPATHWhenDemGoTooOld(t *testing.T) {
	paths := isolatedDemHome(t)
	// the dem-managed go is real but too old for the requirement below,
	// so PATH's go (whatever built this test binary, definitely new
	// enough) must be the one resolveGo picks instead.
	installFakeDemGo(t, paths, "1.1.0", "1.1.0")

	bin, demManaged, err := resolveGo(context.Background(), "1.20")
	if err != nil {
		t.Fatal(err)
	}
	if demManaged {
		t.Fatalf("expected PATH's go to be used since the dem go (1.1.0) doesn't satisfy go>=1.20, got dem-managed bin=%q", bin)
	}
}

func TestResolveGoNoGoAnywhereFails(t *testing.T) {
	isolatedDemHome(t)
	t.Setenv("PATH", t.TempDir()) // hide the real go from PATH

	if _, _, err := resolveGo(context.Background(), "1.0"); err == nil {
		t.Fatal("expected an error when no go satisfies the requirement anywhere")
	}
}
