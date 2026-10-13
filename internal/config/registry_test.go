package config

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestRegistryConfigLoadAndInclude(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "first.yaml", `
registries:
  registry.example.com:
    username: first
    password: first-password
  localhost:5000:
    username: first
    password: first-password
`)
	writeFile(t, dir, "second.yaml", `
registries:
  registry.example.com:
    username: second
    password: second-password
  localhost:5000:
    username: second
    password: second-password
  index.docker.io:
    username: hub-user
    password_env: HUB_TOKEN
`)
	path := writeFile(t, dir, "main.yaml", `
include: [first.yaml, second.yaml]
registries:
  REGISTRY.EXAMPLE.COM:
    username_env: REGISTRY_USER
    password_env: REGISTRY_TOKEN
stages: [build]
Build:
  stage: build
  image: registry.example.com/team/build:latest
  script: [echo hi]
`)
	cfg := NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatal(err)
	}
	// Validation does not need the credentials in the current environment.
	t.Setenv("REGISTRY_USER", "")
	t.Setenv("REGISTRY_TOKEN", "")
	if err := ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Jobs) != 1 || len(cfg.Registries) != 3 {
		t.Fatalf("got %d jobs and %d registries, want 1 and 3", len(cfg.Jobs), len(cfg.Registries))
	}
	if got := cfg.Registries["registry.example.com"]; got.UsernameEnv != "REGISTRY_USER" || got.PasswordEnv != "REGISTRY_TOKEN" || got.Username != "" || got.Password != "" {
		t.Error("main config should replace the complete credential entry")
	}
	if got := cfg.Registries["localhost:5000"]; got.Username != "second" || got.Password != "second-password" {
		t.Error("later include should win for each registry host")
	}
	if got := cfg.Registries["docker.io"]; got.Username != "hub-user" || got.PasswordEnv != "HUB_TOKEN" {
		t.Error("Docker Hub alias should normalize to docker.io")
	}
}

func TestRegistryConfigValidation(t *testing.T) {
	for _, tt := range []struct {
		name, input, wantErr string
	}{
		{"list", "registries: []", "mapping"},
		{"credential shape", "registries: {registry.example.com: secret-value}", "mapping"},
		{"credential field type", "registries: {registry.example.com: {username: [secret-value], password: test}}", "strings"},
		{"URL", "registries: {https://registry.example.com: {username: test, password: test}}", "hostname"},
		{"path", "registries: {registry.example.com/team: {username: test, password: test}}", "hostname"},
		{"namespace", "registries: {team: {username: test, password: test}}", "hostname"},
		{"missing user", "registries: {registry.example.com: {password: secret-value}}", "username"},
		{"missing password", "registries: {registry.example.com: {username: user}}", "password"},
		{"two users", "registries: {registry.example.com: {username: user, username_env: USER_VAR, password: secret-value}}", "exactly one"},
		{"two passwords", "registries: {registry.example.com: {username: user, password: secret-value, password_env: PASS_VAR}}", "exactly one"},
		{"invalid env", "registries: {registry.example.com: {username: user, password_env: '${TOKEN}'}}", "environment variable name"},
		{"duplicate canonical host", "registries: {docker.io: {username: user, password: secret-value}, index.docker.io: {username: user, password: secret-value}}", "duplicate registry"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			err := yaml.Unmarshal([]byte(tt.input), &cfg)
			if err == nil {
				err = NewConfigValidator(&cfg).validateRegistries()
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
			if strings.Contains(err.Error(), "secret-value") {
				t.Fatal("credential value leaked in validation error")
			}
		})
	}
}

func TestRegistryCredentialsResolve(t *testing.T) {
	t.Setenv("LOCAL_CI_TEST_USER", "ci-user")
	t.Setenv("LOCAL_CI_TEST_TOKEN", "test$token")
	for _, cfg := range []RegistryConfig{
		{Username: "ci-user", Password: "test$token"},
		{UsernameEnv: "LOCAL_CI_TEST_USER", PasswordEnv: "LOCAL_CI_TEST_TOKEN"},
	} {
		user, password, err := cfg.Resolve()
		if err != nil {
			t.Fatal(err)
		}
		if user != "ci-user" || password != "test$token" {
			t.Error("resolved credentials do not match the configured values")
		}
	}
	t.Setenv("LOCAL_CI_TEST_TOKEN", "")
	_, _, err := (RegistryConfig{Username: "ci-user", PasswordEnv: "LOCAL_CI_TEST_TOKEN"}).Resolve()
	if err == nil || !strings.Contains(err.Error(), "LOCAL_CI_TEST_TOKEN") {
		t.Fatalf("expected missing token environment variable error, got %v", err)
	}
}

func TestRegistryHostNormalization(t *testing.T) {
	for input, want := range map[string]string{
		"REGISTRY.EXAMPLE.COM": "registry.example.com", "localhost:5000": "localhost:5000",
		"[::1]:5000": "[::1]:5000", "index.docker.io": "docker.io", "docker.io": "docker.io",
	} {
		got, err := normalizeRegistryHost(input)
		if err != nil || got != want {
			t.Errorf("normalize %q = %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestJobRegistriesWithImage(t *testing.T) {
	for _, image := range []string{
		"registry.example.com/team/build:latest",
		"{name: registry.example.com/team/build:latest, force_pull: true}",
	} {
		t.Run(image, func(t *testing.T) {
			cfg := NewConfig(writeTestYAML(t, `
stages: [build]
Build:
  stage: build
  registries:
    REGISTRY.EXAMPLE.COM:
      username: ci-user
      password_env: REGISTRY_TOKEN
  image: `+image+`
  script: [echo hi]
`))
			if err := cfg.LoadConfig(); err != nil {
				t.Fatal(err)
			}
			if err := ValidateConfig(cfg); err != nil {
				t.Fatal(err)
			}
			if len(cfg.Jobs) != 1 || cfg.Jobs[0].Image.Name != "registry.example.com/team/build:latest" {
				t.Fatal("job image was not preserved")
			}
			credentials := cfg.Jobs[0].Registries["registry.example.com"]
			if credentials.Username != "ci-user" || credentials.PasswordEnv != "REGISTRY_TOKEN" {
				t.Fatal("job credentials were not preserved")
			}
			if len(cfg.Registries) != 0 {
				t.Fatal("job credentials leaked into pipeline defaults")
			}
		})
	}
}

func TestJobRegistryInheritanceAndMatrix(t *testing.T) {
	cfg := NewConfig(writeTestYAML(t, `
stages: [build]
.base:
  stage: build
  image: registry.example.com/team/build:latest
  registries:
    registry.example.com:
      username: template-user
      password: template-password
    services.example.com:
      username: service-user
      password: service-password
  script: [echo hi]
Override:
  extends: .base
  registries:
    registry.example.com:
      username_env: JOB_USER
      password_env: JOB_TOKEN
  matrix:
    - VERSION: [one, two]
Inherited:
  extends: .base
`))
	if err := cfg.LoadConfig(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	override, inherited := cfg.Jobs[0], cfg.Jobs[1]
	credentials := override.Registries["registry.example.com"]
	if credentials.UsernameEnv != "JOB_USER" || credentials.PasswordEnv != "JOB_TOKEN" || credentials.Username != "" || credentials.Password != "" {
		t.Fatal("job should replace the complete credential entry for its registry")
	}
	if override.Registries["services.example.com"].Username != "service-user" {
		t.Fatal("job should inherit other registry hosts")
	}
	if inherited.Registries["registry.example.com"].Username != "template-user" {
		t.Fatal("override changed a sibling job's credentials")
	}
	variants, err := ExpandMatrix(override)
	if err != nil || len(variants) != 2 {
		t.Fatalf("expand matrix: %v (got %d variants)", err, len(variants))
	}
	if variants[0].Registries["registry.example.com"] != credentials || variants[1].Registries["registry.example.com"] != credentials {
		t.Fatal("matrix variants lost their job credentials")
	}
	delete(variants[0].Registries, "registry.example.com")
	if variants[1].Registries["registry.example.com"] != credentials || override.Registries["registry.example.com"] != credentials {
		t.Fatal("matrix variants share a mutable credential map")
	}
}

func TestJobRegistryValidation(t *testing.T) {
	cfg := NewConfig(writeTestYAML(t, `
stages: [build]
Build:
  stage: build
  image: registry.example.com/team/build:latest
  registries:
    registry.example.com:
      username: ci-user
  script: [echo hi]
`))
	if err := cfg.LoadConfig(); err != nil {
		t.Fatal(err)
	}
	err := ValidateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), `job "Build"`) || !strings.Contains(err.Error(), "password") {
		t.Fatalf("expected job-specific credential error, got %v", err)
	}
}
