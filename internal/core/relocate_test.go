package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveToRelocatesToolsCacheAndConfig(t *testing.T) {
	oldRoot := t.TempDir()
	newRoot := filepath.Join(t.TempDir(), "new") // does not exist yet

	old := PathsAt(oldRoot)
	if err := old.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	// a tool installation with a nested file, a cached download, and
	// the global config
	toolFile := filepath.Join(old.Tools, "kit", "0.1.1", "bin", "kit")
	if err := os.MkdirAll(filepath.Dir(toolFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(toolFile, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old.Cache, "kit-0.1.1.tar.gz"), []byte("archive"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (Config{Global: map[string]string{"kit": "0.1.1"}}).Save(old.ConfigFile()); err != nil {
		t.Fatal(err)
	}

	newP := PathsAt(newRoot)
	if err := newP.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	if err := old.MoveTo(newP); err != nil {
		t.Fatalf("MoveTo: %v", err)
	}

	if data, err := os.ReadFile(filepath.Join(newP.Tools, "kit", "0.1.1", "bin", "kit")); err != nil || string(data) != "binary" {
		t.Fatalf("tool file not moved correctly: %v %q", err, data)
	}
	if data, err := os.ReadFile(filepath.Join(newP.Cache, "kit-0.1.1.tar.gz")); err != nil || string(data) != "archive" {
		t.Fatalf("cache file not moved correctly: %v %q", err, data)
	}
	cfg, err := LoadConfig(newP.ConfigFile())
	if err != nil || cfg.Global["kit"] != "0.1.1" {
		t.Fatalf("config not moved correctly: %v %v", err, cfg)
	}

	// the old tree's moved directories should now be empty
	for _, dir := range []string{old.Tools, old.Cache} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Errorf("%s still has entries after MoveTo: %v", dir, entries)
		}
	}
	if _, err := os.Stat(old.ConfigFile()); !os.IsNotExist(err) {
		t.Errorf("old config.yaml still exists: %v", err)
	}
}

func TestMoveToIsNoOpOnEmptySource(t *testing.T) {
	old := PathsAt(t.TempDir())
	if err := old.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	newP := PathsAt(t.TempDir())
	if err := newP.EnsureLayout(); err != nil {
		t.Fatal(err)
	}
	if err := old.MoveTo(newP); err != nil {
		t.Fatalf("MoveTo on empty install: %v", err)
	}
}
