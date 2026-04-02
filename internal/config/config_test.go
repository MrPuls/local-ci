package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test yaml: %v", err)
	}
	return path
}

func TestLoadConfig_BasicPipeline(t *testing.T) {
	path := writeTestYAML(t, `
stages:
  - build
  - test

variables:
  GLOBAL_VAR: "hello"

Build:
  stage: build
  image: golang:1.21
  script:
    - go build

Test:
  stage: test
  image: golang:1.21
  script:
    - go test ./...
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(cfg.Stages))
	}
	if len(cfg.Jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(cfg.Jobs))
	}
}

func TestLoadConfig_GlobalVariablesMergedIntoJobs(t *testing.T) {
	path := writeTestYAML(t, `
stages:
  - build

variables:
  FOO: "global"
  BAR: "global"

Build:
  stage: build
  image: alpine
  variables:
    FOO: "overridden"
  script:
    - echo hi
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(cfg.Jobs))
	}

	job := cfg.Jobs[0]
	if job.Variables["FOO"] != "overridden" {
		t.Errorf("expected FOO to be overridden, got %q", job.Variables["FOO"])
	}
	if job.Variables["BAR"] != "global" {
		t.Errorf("expected BAR to be inherited from global, got %q", job.Variables["BAR"])
	}
}

func TestLoadConfig_DefaultWorkdir(t *testing.T) {
	path := writeTestYAML(t, `
stages:
  - build

Build:
  stage: build
  image: alpine
  script:
    - echo hi
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Jobs[0].Workdir != "/" {
		t.Errorf("expected default workdir '/', got %q", cfg.Jobs[0].Workdir)
	}
}

func TestLoadConfig_CustomWorkdir(t *testing.T) {
	path := writeTestYAML(t, `
stages:
  - build

Build:
  stage: build
  image: alpine
  workdir: /app
  script:
    - echo hi
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Jobs[0].Workdir != "/app" {
		t.Errorf("expected workdir '/app', got %q", cfg.Jobs[0].Workdir)
	}
}

func TestLoadConfig_BootstrapAndCleanup(t *testing.T) {
	path := writeTestYAML(t, `
stages:
  - build

bootstrap:
  run:
    - docker compose up -d
  timeout: 10

cleanup:
  run:
    - docker compose down
  timeout: 5

Build:
  stage: build
  image: alpine
  script:
    - echo hi
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Bootstrap == nil {
		t.Fatal("expected bootstrap to be parsed")
	}
	if len(cfg.Bootstrap.Run) != 1 || cfg.Bootstrap.Run[0] != "docker compose up -d" {
		t.Errorf("unexpected bootstrap run: %v", cfg.Bootstrap.Run)
	}
	if cfg.Bootstrap.Timeout != 10 {
		t.Errorf("expected bootstrap timeout 10, got %d", cfg.Bootstrap.Timeout)
	}

	if cfg.Cleanup == nil {
		t.Fatal("expected cleanup to be parsed")
	}
	if len(cfg.Cleanup.Run) != 1 || cfg.Cleanup.Run[0] != "docker compose down" {
		t.Errorf("unexpected cleanup run: %v", cfg.Cleanup.Run)
	}
	if cfg.Cleanup.Timeout != 5 {
		t.Errorf("expected cleanup timeout 5, got %d", cfg.Cleanup.Timeout)
	}
}

func TestLoadConfig_CacheConfig(t *testing.T) {
	path := writeTestYAML(t, `
stages:
  - build

Build:
  stage: build
  image: node:16
  cache:
    key: node-deps
    paths:
      - node_modules
      - .npm
  script:
    - npm install
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	job := cfg.Jobs[0]
	if job.Cache == nil {
		t.Fatal("expected cache to be parsed")
	}
	if job.Cache.Key != "node-deps" {
		t.Errorf("expected cache key 'node-deps', got %q", job.Cache.Key)
	}
	if len(job.Cache.Paths) != 2 {
		t.Errorf("expected 2 cache paths, got %d", len(job.Cache.Paths))
	}
}

func TestLoadConfig_NetworkConfig(t *testing.T) {
	path := writeTestYAML(t, `
stages:
  - build

HostAccess:
  stage: build
  image: alpine
  network:
    host_access: true
  script:
    - echo hi

HostMode:
  stage: build
  image: alpine
  network:
    host_mode: true
  script:
    - echo hi
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(cfg.Jobs))
	}

	for _, job := range cfg.Jobs {
		if job.Network == nil {
			t.Errorf("expected network config on job %s", job.Name)
			continue
		}
		switch job.Name {
		case "HostAccess":
			if !job.Network.HostAccess {
				t.Error("expected host_access to be true")
			}
		case "HostMode":
			if !job.Network.HostMode {
				t.Error("expected host_mode to be true")
			}
		}
	}
}

func TestLoadConfig_RemoteProvider(t *testing.T) {
	path := writeTestYAML(t, `
stages:
  - build

remote_provider:
  url: "gitlab.example.com"
  project_id: 12345
  access_token: "my-token"

Build:
  stage: build
  image: alpine
  script:
    - echo hi
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.RemoteProvider == nil {
		t.Fatal("expected remote provider to be parsed")
	}
	if cfg.RemoteProvider.Url != "gitlab.example.com" {
		t.Errorf("expected URL 'gitlab.example.com', got %q", cfg.RemoteProvider.Url)
	}
	if cfg.RemoteProvider.ProjectId != 12345 {
		t.Errorf("expected project ID 12345, got %d", cfg.RemoteProvider.ProjectId)
	}
	if cfg.RemoteProvider.Token != "my-token" {
		t.Errorf("expected token 'my-token', got %q", cfg.RemoteProvider.Token)
	}
}

func TestLoadConfig_JobBootstrapAndCleanup(t *testing.T) {
	path := writeTestYAML(t, `
stages:
  - build

Build:
  stage: build
  image: alpine
  job_bootstrap:
    run:
      - echo "setting up"
    timeout: 3
  job_cleanup:
    run:
      - echo "tearing down"
    timeout: 2
  script:
    - echo hi
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	job := cfg.Jobs[0]
	if job.JobBootstrap == nil {
		t.Fatal("expected job_bootstrap to be parsed")
	}
	if len(job.JobBootstrap.Run) != 1 || job.JobBootstrap.Run[0] != "echo \"setting up\"" {
		t.Errorf("unexpected job_bootstrap run: %v", job.JobBootstrap.Run)
	}
	if job.JobBootstrap.Timeout != 3 {
		t.Errorf("expected job_bootstrap timeout 3, got %d", job.JobBootstrap.Timeout)
	}

	if job.JobCleanup == nil {
		t.Fatal("expected job_cleanup to be parsed")
	}
	if len(job.JobCleanup.Run) != 1 || job.JobCleanup.Run[0] != "echo \"tearing down\"" {
		t.Errorf("unexpected job_cleanup run: %v", job.JobCleanup.Run)
	}
	if job.JobCleanup.Timeout != 2 {
		t.Errorf("expected job_cleanup timeout 2, got %d", job.JobCleanup.Timeout)
	}
}

func TestLoadConfig_JobBootstrapWithoutCleanup(t *testing.T) {
	path := writeTestYAML(t, `
stages:
  - build

Build:
  stage: build
  image: alpine
  job_bootstrap:
    run:
      - echo "setup"
  script:
    - echo hi
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	job := cfg.Jobs[0]
	if job.JobBootstrap == nil {
		t.Fatal("expected job_bootstrap to be parsed")
	}
	if job.JobCleanup != nil {
		t.Error("expected job_cleanup to be nil")
	}
}

func TestLoadConfig_NonExistentFile(t *testing.T) {
	cfg := NewConfig("/nonexistent/path.yaml")
	if err := cfg.LoadConfig(); err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	path := writeTestYAML(t, `
stages:
  - build
  invalid indentation
    broken: yaml
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err == nil {
		t.Error("expected error for invalid YAML")
	}
}
