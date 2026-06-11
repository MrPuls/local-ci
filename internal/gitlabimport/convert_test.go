package gitlabimport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrPuls/local-ci/internal/config"
)

const sampleGitlabCI = `
stages:
  - build
  - test
  - deploy

variables:
  GO_VERSION: "1.26"
  VERBOSE:
    value: "false"
    description: "chatty logs"

default:
  image: golang:1.26
  before_script:
    - go version

include:
  - template: Security/SAST.gitlab-ci.yml

.go-cache:
  cache:
    key: go-mod
    paths:
      - /go/pkg/mod

build:
  stage: build
  extends: .go-cache
  script:
    - go build -o bin/app ./...
  artifacts:
    paths:
      - bin/
    expire_in: 1 week
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"

test:
  stage: test
  timeout: 30 minutes
  retry:
    max: 2
    when: runner_system_failure
  services:
    - name: postgres:16
      alias: db
    - redis:7-alpine
  variables:
    DATABASE_URL: postgres://db:5432/x
  needs: [build]
  script:
    - go test ./...
  after_script:
    - echo done

lint:
  stage: test
  parallel:
    matrix:
      - GOOS: [linux, darwin]
  script: golangci-lint run
  tags: [docker]

deploy:
  stage: deploy
  environment: production
  when: manual
  script:
    - ./deploy.sh
`

func TestConvertSample(t *testing.T) {
	res, err := Convert([]byte(sampleGitlabCI))
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	out := string(res.YAML)

	// The converted YAML must load and validate as a local-ci config.
	path := filepath.Join(t.TempDir(), ".local-ci.yaml")
	if err := os.WriteFile(path, res.YAML, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.NewConfig(path)
	if err := cfg.LoadConfig(); err != nil {
		t.Fatalf("imported config does not load: %v\n---\n%s", err, out)
	}
	if err := config.ValidateConfig(cfg); err != nil {
		t.Fatalf("imported config does not validate: %v\n---\n%s", err, out)
	}

	byName := map[string]config.JobConfig{}
	for _, j := range cfg.Jobs {
		byName[j.Name] = j
	}

	// default image + before_script folded in.
	build := byName["build"]
	if build.Image != "golang:1.26" {
		t.Errorf("build image = %q, want default golang:1.26", build.Image)
	}
	if len(build.Script) != 2 || build.Script[0] != "go version" {
		t.Errorf("build script = %v, want before_script first", build.Script)
	}
	if build.Cache == nil || build.Cache.Key != "go-mod" {
		t.Errorf("extends cache not carried: %+v", build.Cache)
	}
	if build.Artifacts == nil || build.Artifacts.Paths[0] != "bin/" {
		t.Errorf("artifacts = %+v", build.Artifacts)
	}

	test := byName["test"]
	if test.Timeout.Std().String() != "30m0s" {
		t.Errorf("timeout = %s, want 30m", test.Timeout.Std())
	}
	if test.Retry != 2 {
		t.Errorf("retry = %d, want 2", test.Retry)
	}
	if len(test.Services) != 2 || test.Services[0].EffectiveAlias() != "db" || test.Services[1].Image != "redis:7-alpine" {
		t.Errorf("services = %+v", test.Services)
	}
	if len(test.Needs) != 1 || test.Needs[0] != "build" {
		t.Errorf("needs = %v", test.Needs)
	}

	lint := byName["lint"]
	if len(lint.Matrix) != 1 {
		t.Errorf("parallel:matrix not converted: %+v", lint.Matrix)
	}
	if len(lint.Script) != 2 { // before_script + the string-form script
		t.Errorf("lint script = %v", lint.Script)
	}

	// Notes must mention the dropped GitLab-isms.
	notes := strings.Join(res.Notes, "\n")
	for _, want := range []string{"rules", "after_script", "include", "environment", "tags", "when"} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes missing %q:\n%s", want, notes)
		}
	}
}

func TestParseGitlabDuration(t *testing.T) {
	cases := map[string]string{
		"30 minutes":        "30m0s",
		"1 hour 30 minutes": "1h30m0s",
		"1h30m":             "1h30m0s",
		"90s":               "1m30s",
		"2 days":            "48h0m0s",
	}
	for in, want := range cases {
		d, err := parseGitlabDuration(in)
		if err != nil {
			t.Errorf("parse %q: %v", in, err)
			continue
		}
		if d.String() != want {
			t.Errorf("parse %q = %s, want %s", in, d, want)
		}
	}
}
