package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIsConfigFileName(t *testing.T) {
	yes := []string{
		".local-ci.yaml", ".local-ci.yml", "local-ci.yaml", "local-ci.yml",
		"build.local-ci.yaml", "deploy-local-ci.yml", "smoke_local-ci.yaml",
		".LOCAL-CI.YAML", "Build-Local-CI.yaml",
	}
	no := []string{
		"nonlocal-ci.yaml", "local-ci.json", "local-ci", ".local-ci.yaml.bak",
		"config.yaml", "locallyci.yaml", "local-ci.yaml.tmp",
	}
	for _, n := range yes {
		if !IsConfigFileName(n) {
			t.Errorf("IsConfigFileName(%q) = false, want true", n)
		}
	}
	for _, n := range no {
		if IsConfigFileName(n) {
			t.Errorf("IsConfigFileName(%q) = true, want false", n)
		}
	}
}

func TestDiscoverConfigs(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"zeta.local-ci.yaml", ".local-ci.yaml", "alpha-local-ci.yml",
		"README.md", "config.yaml",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("stages: []\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A matching directory name must be ignored.
	if err := os.Mkdir(filepath.Join(dir, "dir.local-ci.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverConfigs(dir)
	if err != nil {
		t.Fatalf("DiscoverConfigs: %v", err)
	}
	want := []string{".local-ci.yaml", "alpha-local-ci.yml", "zeta.local-ci.yaml"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DiscoverConfigs = %v, want %v", got, want)
	}
}

func TestDiscoverConfigsMissingDir(t *testing.T) {
	if _, err := DiscoverConfigs(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("DiscoverConfigs on a missing dir: want error, got nil")
	}
}
