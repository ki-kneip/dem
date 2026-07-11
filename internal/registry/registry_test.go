package registry

import (
	"runtime"
	"strings"
	"testing"
)

func TestEmbeddedIsValid(t *testing.T) {
	r := Embedded() // panics if the bundled registry.yaml is broken
	if len(r.Tools) == 0 {
		t.Fatal("embedded registry has no tools")
	}
	for _, name := range []string{"pnpm", "kit"} {
		if _, ok := r.Tools[name]; !ok {
			t.Fatalf("embedded registry is missing %q", name)
		}
	}
}

func TestParseRejectsUnsupportedSchema(t *testing.T) {
	_, err := Parse([]byte("schema: 999\ntools: {}\n"))
	if err == nil || !strings.Contains(err.Error(), "schema") {
		t.Fatalf("expected a schema error, got %v", err)
	}
}

func TestParseRejectsIncompleteTool(t *testing.T) {
	doc := "schema: 1\ntools:\n  broken:\n    repo: a/b\n"
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error for a tool without executables/assets")
	}
}

func TestAssetForExpandsVersion(t *testing.T) {
	key := runtime.GOOS + "-" + runtime.GOARCH
	tool := Tool{Assets: map[string]string{key: "x_v{version}_" + key + ".zip"}}
	got, err := tool.AssetFor("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	want := "x_v1.2.3_" + key + ".zip"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestToolFor(t *testing.T) {
	r := Embedded()
	if tool, ok := r.ToolFor("pnpx"); !ok || tool != "pnpm" {
		t.Fatalf("pnpx should map to pnpm, got %q ok=%v", tool, ok)
	}
	if _, ok := r.ToolFor("definitely-not-a-tool"); ok {
		t.Fatal("unexpected match")
	}
}
