package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestResolveIncludePathAbsolute(t *testing.T) {
	abs := "/tmp/foo.yaml"
	got := resolveIncludePath(abs, "/other/dir")
	if got != filepath.Clean(abs) {
		t.Errorf("expected absolute path preserved, got %q", got)
	}
}

func TestResolveIncludePathRelative(t *testing.T) {
	got := resolveIncludePath("child.yaml", "/parent/dir")
	if got != "/parent/dir/child.yaml" {
		t.Errorf("expected relative join, got %q", got)
	}
}

func TestLoadConfigWithSingleInclude(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "template.yaml", `
.base:
  image: alpine
  stage: build
`)
	main := writeFile(t, dir, "main.yaml", `
stages:
  - build

include:
  - template.yaml

Build:
  extends: .base
  script:
    - echo hello
`)
	cfg, err := loadConfigWithIncludes(main, map[string]bool{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Jobs) != 2 {
		t.Fatalf("expected 2 jobs (template + Build), got %d: %v", len(cfg.Jobs), jobNamesList(cfg.Jobs))
	}
}

func TestLoadConfigIncludesAreCleared(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "leaf.yaml", `.x:
  image: alpine
`)
	main := writeFile(t, dir, "main.yaml", `
include:
  - leaf.yaml

stages:
  - build
`)
	cfg, err := loadConfigWithIncludes(main, map[string]bool{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Include != nil {
		t.Errorf("expected Include to be cleared after resolution, got %v", cfg.Include)
	}
}

func TestLoadConfigRecursiveInclude(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "sub/leaf.yaml", `.leaf:
  image: alpine
`)
	writeFile(t, dir, "sub/mid.yaml", `
include:
  - leaf.yaml

.mid:
  image: ubuntu
`)
	main := writeFile(t, dir, "main.yaml", `
include:
  - sub/mid.yaml

stages:
  - build
`)
	cfg, err := loadConfigWithIncludes(main, map[string]bool{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	names := jobNamesList(cfg.Jobs)
	if !contains(names, ".mid") || !contains(names, ".leaf") {
		t.Errorf("expected both .mid and .leaf to be loaded transitively, got %v", names)
	}
}

func TestLoadConfigCycleDetection(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.yaml", `
include:
  - b.yaml
`)
	writeFile(t, dir, "b.yaml", `
include:
  - a.yaml
`)
	_, err := loadConfigWithIncludes(a, map[string]bool{}, nil)
	if err == nil {
		t.Fatal("expected circular include error")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("expected error to mention 'circular', got %v", err)
	}
}

func TestLoadConfigLaterIncludeWinsOnConflict(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "first.yaml", `
Build:
  image: from-first
  stage: build
  script:
    - first
`)
	writeFile(t, dir, "second.yaml", `
Build:
  image: from-second
  stage: build
  script:
    - second
`)
	// Main does not define Build, so include precedence decides the winner.
	main := writeFile(t, dir, "main.yaml", `
stages:
  - build

include:
  - first.yaml
  - second.yaml
`)
	cfg, err := loadConfigWithIncludes(main, map[string]bool{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var build *JobConfig
	for i := range cfg.Jobs {
		if cfg.Jobs[i].Name == "Build" {
			build = &cfg.Jobs[i]
		}
	}
	if build == nil {
		t.Fatal("Build job missing")
	}
	if build.Image != "from-second" {
		t.Errorf("expected later include (second.yaml) to win, got image %q", build.Image)
	}
}

func TestLoadConfigLaterIncludeWinsGlobalVariable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "first.yaml", "variables:\n  X: from-first\n")
	writeFile(t, dir, "second.yaml", "variables:\n  X: from-second\n")
	main := writeFile(t, dir, "main.yaml", `
stages:
  - build

include:
  - first.yaml
  - second.yaml
`)
	cfg, err := loadConfigWithIncludes(main, map[string]bool{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GlobalVariables["X"] != "from-second" {
		t.Errorf("expected later include's global var to win, got %q", cfg.GlobalVariables["X"])
	}
}

func TestLoadConfigDiamondIncludeAllowed(t *testing.T) {
	// main -> [a, b]; a -> shared; b -> shared. The shared file is pulled in
	// via two branches but that is not a cycle and must not error.
	dir := t.TempDir()
	writeFile(t, dir, "shared.yaml", ".shared:\n  image: alpine\n")
	writeFile(t, dir, "a.yaml", `
include:
  - shared.yaml

.a:
  image: alpine
`)
	writeFile(t, dir, "b.yaml", `
include:
  - shared.yaml

.b:
  image: alpine
`)
	main := writeFile(t, dir, "main.yaml", `
stages:
  - build

include:
  - a.yaml
  - b.yaml
`)
	cfg, err := loadConfigWithIncludes(main, map[string]bool{}, nil)
	if err != nil {
		t.Fatalf("diamond include should not error, got: %v", err)
	}
	count := 0
	for _, n := range jobNamesList(cfg.Jobs) {
		if n == ".shared" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected .shared to appear exactly once after dedup, got %d", count)
	}
}

func TestLoadConfigMainWinsOnJobNameConflict(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tpl.yaml", `
Build:
  image: from-include
  stage: build
  script:
    - included
`)
	main := writeFile(t, dir, "main.yaml", `
stages:
  - build

include:
  - tpl.yaml

Build:
  image: from-main
  stage: build
  script:
    - main
`)
	cfg, err := loadConfigWithIncludes(main, map[string]bool{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var build *JobConfig
	for i := range cfg.Jobs {
		if cfg.Jobs[i].Name == "Build" {
			build = &cfg.Jobs[i]
		}
	}
	if build == nil {
		t.Fatal("Build job missing")
	}
	if build.Image != "from-main" {
		t.Errorf("expected main to win, got image %q", build.Image)
	}
}

func TestLoadConfigMergesGlobalVariables(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "tpl.yaml", `
variables:
  X: from-include
  ONLY_IN_INCLUDE: yes
`)
	main := writeFile(t, dir, "main.yaml", `
stages:
  - build

include:
  - tpl.yaml

variables:
  X: from-main
  ONLY_IN_MAIN: yes
`)
	cfg, err := loadConfigWithIncludes(main, map[string]bool{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GlobalVariables["X"] != "from-main" {
		t.Errorf("expected X=from-main, got %q", cfg.GlobalVariables["X"])
	}
	if cfg.GlobalVariables["ONLY_IN_INCLUDE"] != "yes" {
		t.Errorf("expected ONLY_IN_INCLUDE inherited from include, got %q", cfg.GlobalVariables["ONLY_IN_INCLUDE"])
	}
	if cfg.GlobalVariables["ONLY_IN_MAIN"] != "yes" {
		t.Errorf("expected ONLY_IN_MAIN preserved, got %q", cfg.GlobalVariables["ONLY_IN_MAIN"])
	}
}

func TestLoadConfigIntegrationWithExtends(t *testing.T) {
	// End-to-end: include + extends + matrix in one config.
	dir := t.TempDir()
	writeFile(t, dir, "tpl.yaml", `
.go-base:
  image: golang:1.22
  workdir: /app
  variables:
    GOFLAGS: -count=1
`)
	main := writeFile(t, dir, "main.yaml", `
stages:
  - test

include:
  - tpl.yaml

Test:
  extends: .go-base
  stage: test
  script:
    - go test ./...
`)
	cfg := &Config{FileName: main}
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if len(cfg.Jobs) != 1 {
		t.Fatalf("expected 1 final job (template removed), got %d: %v", len(cfg.Jobs), jobNamesList(cfg.Jobs))
	}
	job := cfg.Jobs[0]
	if job.Name != "Test" {
		t.Fatalf("expected job named Test, got %q", job.Name)
	}
	if job.Image != "golang:1.22" {
		t.Errorf("expected inherited image, got %q", job.Image)
	}
	if job.Workdir != "/app" {
		t.Errorf("expected inherited workdir, got %q", job.Workdir)
	}
	if job.Variables["GOFLAGS"] != "-count=1" {
		t.Errorf("expected inherited variable, got %q", job.Variables["GOFLAGS"])
	}
	if len(job.Extends) != 0 {
		t.Errorf("expected Extends cleared, got %v", job.Extends)
	}
}

func jobNamesList(jobs []JobConfig) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.Name
	}
	return out
}

func contains(s []string, needle string) bool {
	for _, v := range s {
		if v == needle {
			return true
		}
	}
	return false
}
