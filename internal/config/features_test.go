package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func loadConfig(t *testing.T, yaml string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".local-ci.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		return nil, err
	}
	return cfg, ValidateConfig(cfg)
}

const baseJob = `
stages:
  - build
  - test
`

func TestTimeoutAndRetryParsing(t *testing.T) {
	cfg, err := loadConfig(t, baseJob+`
Build:
  stage: build
  image: alpine
  script: ["true"]
  timeout: 10m
  retry: 2
Test:
  stage: test
  image: alpine
  script: ["true"]
  timeout: 90
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	byName := map[string]JobConfig{}
	for _, j := range cfg.Jobs {
		byName[j.Name] = j
	}
	if d := byName["Build"].Timeout.Std(); d != 10*time.Minute {
		t.Errorf("Build timeout = %s, want 10m", d)
	}
	if byName["Build"].Retry != 2 {
		t.Errorf("Build retry = %d, want 2", byName["Build"].Retry)
	}
	if d := byName["Test"].Timeout.Std(); d != 90*time.Second {
		t.Errorf("Test timeout = %s, want 90s (bare int = seconds)", d)
	}
}

func TestRetryOutOfRangeRejected(t *testing.T) {
	_, err := loadConfig(t, baseJob+`
Build:
  stage: build
  image: alpine
  script: ["true"]
  retry: 11
`)
	if err == nil || !strings.Contains(err.Error(), "retry") {
		t.Errorf("want retry range error, got %v", err)
	}
}

func TestServiceShorthandAndAlias(t *testing.T) {
	cfg, err := loadConfig(t, baseJob+`
Test:
  stage: test
  image: alpine
  script: ["true"]
  services:
    - postgres:16
    - image: registry.example.com/group/redis:7-alpine
      alias: cache
      variables:
        FOO: bar
      ready:
        command: redis-cli ping
        timeout: 30s
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	svcs := cfg.Jobs[0].Services
	if len(svcs) != 2 {
		t.Fatalf("services = %d, want 2", len(svcs))
	}
	if svcs[0].Image != "postgres:16" || svcs[0].EffectiveAlias() != "postgres" {
		t.Errorf("shorthand service = %+v alias %q", svcs[0], svcs[0].EffectiveAlias())
	}
	if svcs[1].EffectiveAlias() != "cache" || svcs[1].Variables["FOO"] != "bar" {
		t.Errorf("mapping service = %+v", svcs[1])
	}
	if svcs[1].Ready == nil || svcs[1].Ready.Command != "redis-cli ping" || svcs[1].Ready.Timeout.Std() != 30*time.Second {
		t.Errorf("ready = %+v", svcs[1].Ready)
	}
}

func TestServiceAliasDerivation(t *testing.T) {
	cases := map[string]string{
		"postgres:16": "postgres",
		"redis":       "redis",
		"registry.example.com/a/b/mysql@sha256:abc": "mysql",
	}
	for image, want := range cases {
		s := ServiceConfig{Image: image}
		if got := s.EffectiveAlias(); got != want {
			t.Errorf("EffectiveAlias(%q) = %q, want %q", image, got, want)
		}
	}
}

func TestServiceValidation(t *testing.T) {
	cases := []struct {
		name, yaml, wantErr string
	}{
		{
			"duplicate alias",
			`
Test:
  stage: test
  image: alpine
  script: ["true"]
  services: [postgres:16, postgres:15]
`,
			"duplicate service alias",
		},
		{
			"host mode conflict",
			`
Test:
  stage: test
  image: alpine
  script: ["true"]
  network:
    host_mode: true
  services: [postgres:16]
`,
			"host_mode",
		},
	}
	for _, c := range cases {
		_, err := loadConfig(t, baseJob+c.yaml)
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: want error containing %q, got %v", c.name, c.wantErr, err)
		}
	}
}

func TestArtifactsValidation(t *testing.T) {
	_, err := loadConfig(t, baseJob+`
Build:
  stage: build
  image: alpine
  script: ["true"]
  artifacts:
    paths: ["../escape"]
`)
	if err == nil || !strings.Contains(err.Error(), "..") {
		t.Errorf("want path traversal rejection, got %v", err)
	}
}

func TestNeedsValidation(t *testing.T) {
	cases := []struct {
		name, yaml, wantErr string
	}{
		{
			"unknown job",
			`
Build:
  stage: build
  image: alpine
  script: ["true"]
  needs: nope
`,
			"unknown job",
		},
		{
			"self need",
			`
Build:
  stage: build
  image: alpine
  script: ["true"]
  needs: [Build]
`,
			"needs itself",
		},
		{
			"later stage",
			`
Build:
  stage: build
  image: alpine
  script: ["true"]
  needs: [Test]
Test:
  stage: test
  image: alpine
  script: ["true"]
`,
			"later stage",
		},
		{
			"cycle",
			`
A:
  stage: build
  image: alpine
  script: ["true"]
  needs: [B]
B:
  stage: build
  image: alpine
  script: ["true"]
  needs: [A]
`,
			"cycle",
		},
	}
	for _, c := range cases {
		_, err := loadConfig(t, baseJob+c.yaml)
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: want error containing %q, got %v", c.name, c.wantErr, err)
		}
	}

	// And a valid graph loads.
	if _, err := loadConfig(t, baseJob+`
Build:
  stage: build
  image: alpine
  script: ["true"]
Unit:
  stage: test
  image: alpine
  script: ["true"]
  needs: Build
`); err != nil {
		t.Errorf("valid needs graph rejected: %v", err)
	}
}

func TestExtendsCarriesNewFields(t *testing.T) {
	cfg, err := loadConfig(t, baseJob+`
.tmpl:
  image: alpine
  script: ["true"]
  retry: 1
  timeout: 5m
  services: [postgres:16]
  artifacts:
    paths: [dist/]
Build:
  stage: build
  extends: .tmpl
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	j := cfg.Jobs[0]
	if j.Retry != 1 || j.Timeout.Std() != 5*time.Minute {
		t.Errorf("retry/timeout not inherited: %+v", j)
	}
	if len(j.Services) != 1 || j.Artifacts == nil {
		t.Errorf("services/artifacts not inherited: %+v", j)
	}
}
