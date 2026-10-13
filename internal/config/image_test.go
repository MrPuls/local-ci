package config

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

func TestImageConfig(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		want        ImageConfig
	}{
		{"shorthand", "ubuntu", ImageConfig{Name: "ubuntu"}},
		{"mapping defaults", "{name: ubuntu}", ImageConfig{Name: "ubuntu"}},
		{"force pull", "{name: ubuntu, force_pull: true}", ImageConfig{Name: "ubuntu", ForcePull: true}},
		{"explicit reuse", "{name: ubuntu, force_pull: false}", ImageConfig{Name: "ubuntu"}},
		{"tag", "registry.example.com/team/app:v2", ImageConfig{Name: "registry.example.com/team/app:v2"}},
		{"digest", "ubuntu@sha256:abc", ImageConfig{Name: "ubuntu@sha256:abc"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig(writeTestYAML(t, "stages: [build]\nBuild:\n  stage: build\n  image: "+tt.input+"\n  script: [echo hi]\n"))
			if err := cfg.LoadConfig(); err != nil {
				t.Fatal(err)
			}
			if err := ValidateConfig(cfg); err != nil {
				t.Fatal(err)
			}
			if got := cfg.Jobs[0].Image; got != tt.want {
				t.Fatalf("image = %+v, want %+v", got, tt.want)
			}
			encoded, err := yaml.Marshal(tt.want)
			if err != nil {
				t.Fatal(err)
			}
			var roundTrip ImageConfig
			if err := yaml.Unmarshal(encoded, &roundTrip); err != nil {
				t.Fatal(err)
			}
			if roundTrip != tt.want {
				t.Errorf("round trip = %+v, want %+v", roundTrip, tt.want)
			}
		})
	}
}

func TestImageConfigInvalid(t *testing.T) {
	for _, input := range []string{
		"[ubuntu]", "true", "123", "''", "'   '", "{}", "{force_pull: true}",
		"{name: ''}", "{name: null}", "{name: [ubuntu]}", "{name: ubuntu, force_pull: sometimes}",
	} {
		t.Run(input, func(t *testing.T) {
			var cfg Config
			err := yaml.Unmarshal([]byte("Build:\n  image: "+input+"\n"), &cfg)
			if err == nil || !strings.Contains(err.Error(), "failed to decode job Build") {
				t.Fatalf("expected contextual image error, got %v", err)
			}
		})
	}
}

func TestImageConfigInheritedAndOverridden(t *testing.T) {
	cfg := NewConfig(writeTestYAML(t, `
stages: [build]
.base:
  stage: build
  image:
    name: ubuntu
    force_pull: true
  script: [echo hi]
Inherited:
  extends: .base
  matrix:
    - VERSION: [one, two]
Shorthand:
  extends: .base
  image: alpine
Mapping:
  extends: .base
  image:
    name: ubuntu
ExplicitFalse:
  extends: .base
  image:
    name: ubuntu
    force_pull: false
`))
	if err := cfg.LoadConfig(); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	want := []ImageConfig{
		{Name: "ubuntu", ForcePull: true}, {Name: "alpine"}, {Name: "ubuntu"}, {Name: "ubuntu"},
	}
	for idx, job := range cfg.Jobs {
		variants, err := ExpandMatrix(job)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range variants {
			if v.Image != want[idx] {
				t.Errorf("job %s: image = %+v, want %+v", v.Name, v.Image, want[idx])
			}
		}
	}
}

func TestServiceImageConfig(t *testing.T) {
	var job JobConfig
	if err := yaml.Unmarshal([]byte(`
services:
  - postgres:16
  - image: redis:7
  - image:
      name: registry.example.com/team/postgres:16
      force_pull: true
    alias: db
`), &job); err != nil {
		t.Fatal(err)
	}
	want := []ImageConfig{
		{Name: "postgres:16"}, {Name: "redis:7"}, {Name: "registry.example.com/team/postgres:16", ForcePull: true},
	}
	for idx, svc := range job.Services {
		if svc.Image != want[idx] {
			t.Errorf("service %d: image = %+v, want %+v", idx, svc.Image, want[idx])
		}
	}
	if got := job.Services[2].EffectiveAlias(); got != "db" {
		t.Errorf("alias = %q, want db", got)
	}
}
