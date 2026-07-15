package goinstall

import (
	"context"
	"testing"

	"github.com/ki-kneip/dem/internal/registry"
)

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
	if err := p.checkRequires(context.Background()); err == nil {
		t.Fatal("expected an error for a malformed requirement")
	}
}

func TestCheckRequiresRejectsUnsupportedTool(t *testing.T) {
	p := New("wails", registry.Tool{Requires: []string{"node>=20"}})
	if err := p.checkRequires(context.Background()); err == nil {
		t.Fatal("expected an error for an unsupported dependency")
	}
}

func TestCheckGoVersionParsesOutput(t *testing.T) {
	// go is required to build dem itself, so it is always on PATH in
	// this test environment; assert against a version we are certain
	// predates the running toolchain.
	if err := checkGoVersion(context.Background(), "1.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckGoVersionRejectsTooNew(t *testing.T) {
	if err := checkGoVersion(context.Background(), "99.0"); err == nil {
		t.Fatal("expected an error for an unreasonably high required version")
	}
}
