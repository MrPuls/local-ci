package config

import (
	"strings"
	"testing"
)

func eqStages(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func src(path string, stages ...string) stageSource {
	return stageSource{path: path, stages: stages}
}

func TestExpandStagePlaceholdersNoPlaceholders(t *testing.T) {
	in := []string{"build", "test", "deploy"}
	got, err := expandStagePlaceholders(in, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !eqStages(got, in) {
		t.Errorf("expected unchanged %v, got %v", in, got)
	}
}

func TestExpandStagePlaceholdersBasicSplice(t *testing.T) {
	ds := map[string][]stageSource{
		"extra":      {src("/x/extra.yaml", "test1", "test2")},
		"extra.yaml": {src("/x/extra.yaml", "test1", "test2")},
	}
	got, err := expandStagePlaceholders([]string{"build", ".extra", "deploy"}, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"build", "test1", "test2", "deploy"}
	if !eqStages(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExpandStagePlaceholdersFullNameMatch(t *testing.T) {
	ds := map[string][]stageSource{
		"extra":      {src("/x/extra.yaml", "t1")},
		"extra.yaml": {src("/x/extra.yaml", "t1")},
	}
	got, err := expandStagePlaceholders([]string{"build", ".extra.yaml"}, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !eqStages(got, []string{"build", "t1"}) {
		t.Errorf("got %v", got)
	}
}

func TestExpandStagePlaceholdersMultiple(t *testing.T) {
	ds := map[string][]stageSource{
		"a": {src("/x/a.yaml", "a1")},
		"b": {src("/x/b.yaml", "b1", "b2")},
	}
	got, err := expandStagePlaceholders([]string{".a", "mid", ".b"}, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"a1", "mid", "b1", "b2"}
	if !eqStages(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExpandStagePlaceholdersEmptyStagesFile(t *testing.T) {
	ds := map[string][]stageSource{
		"empty": {src("/x/empty.yaml")}, // no stages
	}
	got, err := expandStagePlaceholders([]string{"build", ".empty", "deploy"}, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"build", "deploy"} // placeholder contributes nothing
	if !eqStages(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExpandStagePlaceholdersDedup(t *testing.T) {
	ds := map[string][]stageSource{
		"extra": {src("/x/extra.yaml", "setup", "test")},
	}
	// "setup" is declared both as a real stage and inside the spliced file.
	got, err := expandStagePlaceholders([]string{"setup", ".extra", "deploy"}, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"setup", "test", "deploy"}
	if !eqStages(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExpandStagePlaceholdersUnknown(t *testing.T) {
	_, err := expandStagePlaceholders([]string{"build", ".nope"}, map[string][]stageSource{})
	if err == nil {
		t.Fatal("expected error for unknown placeholder")
	}
	if !strings.Contains(err.Error(), "no included file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExpandStagePlaceholdersAmbiguousBasename(t *testing.T) {
	ds := map[string][]stageSource{
		"foo": {src("/x/foo.yaml", "a"), src("/x/foo.yml", "b")},
	}
	_, err := expandStagePlaceholders([]string{".foo"}, ds)
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "full file name") {
		t.Errorf("expected full-file-name hint, got: %v", err)
	}
}

func TestExpandStagePlaceholdersAmbiguousFullName(t *testing.T) {
	ds := map[string][]stageSource{
		"foo.yaml": {src("/a/foo.yaml", "a"), src("/b/foo.yaml", "b")},
	}
	_, err := expandStagePlaceholders([]string{".foo.yaml"}, ds)
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "rename") {
		t.Errorf("expected rename hint for same-name-different-dir, got: %v", err)
	}
}

func TestExpandStagePlaceholdersDisambiguateWithFullName(t *testing.T) {
	ds := map[string][]stageSource{
		"foo":      {src("/x/foo.yaml", "a"), src("/x/foo.yml", "b")},
		"foo.yaml": {src("/x/foo.yaml", "a")},
		"foo.yml":  {src("/x/foo.yml", "b")},
	}
	got, err := expandStagePlaceholders([]string{".foo.yaml"}, ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !eqStages(got, []string{"a"}) {
		t.Errorf("got %v", got)
	}
}

// --- Integration through LoadConfig ---

func TestLoadConfigStagePlaceholderEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "stages-extra.yaml", "stages:\n  - lint\n  - vet\n")
	main := writeFile(t, dir, "main.yaml", `
stages:
  - build
  - .stages-extra
  - deploy

include:
  - stages-extra.yaml

Build:
  stage: build
  image: alpine
  script:
    - echo build
`)
	cfg := &Config{FileName: main}
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	want := []string{"build", "lint", "vet", "deploy"}
	if !eqStages(cfg.Stages, want) {
		t.Errorf("got %v, want %v", cfg.Stages, want)
	}
}

func TestLoadConfigStagePlaceholderInIncludedFileErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "leaf.yaml", "stages:\n  - x\n")
	writeFile(t, dir, "mid.yaml", `
stages:
  - .leaf

include:
  - leaf.yaml
`)
	main := writeFile(t, dir, "main.yaml", `
stages:
  - build

include:
  - mid.yaml
`)
	cfg := &Config{FileName: main}
	err := cfg.LoadConfig()
	if err == nil {
		t.Fatal("expected error: placeholders only allowed in main file")
	}
	if !strings.Contains(err.Error(), "only supported in the main config file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadConfigStagePlaceholderUnknownErrors(t *testing.T) {
	dir := t.TempDir()
	main := writeFile(t, dir, "main.yaml", `
stages:
  - build
  - .missing

Build:
  stage: build
  image: alpine
  script:
    - echo build
`)
	cfg := &Config{FileName: main}
	err := cfg.LoadConfig()
	if err == nil {
		t.Fatal("expected error for placeholder with no matching include")
	}
}
