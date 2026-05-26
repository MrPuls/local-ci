package config

import (
	"strings"
	"testing"
)

func TestMergeJobsScalarOverlay(t *testing.T) {
	base := JobConfig{Image: "alpine", Stage: "build", Workdir: "/old"}
	overlay := JobConfig{Image: "ubuntu", Workdir: ""}
	out := mergeJobs(base, overlay)
	if out.Image != "ubuntu" {
		t.Errorf("image: expected ubuntu (overlay wins), got %q", out.Image)
	}
	if out.Stage != "build" {
		t.Errorf("stage: expected build (base kept), got %q", out.Stage)
	}
	if out.Workdir != "/old" {
		t.Errorf("workdir: empty overlay should not clobber base, got %q", out.Workdir)
	}
}

func TestMergeJobsScriptReplaces(t *testing.T) {
	base := JobConfig{Script: []string{"setup", "test"}}
	overlay := JobConfig{Script: []string{"custom"}}
	out := mergeJobs(base, overlay)
	if len(out.Script) != 1 || out.Script[0] != "custom" {
		t.Errorf("expected script to be replaced, got %v", out.Script)
	}
}

func TestMergeJobsScriptKeptIfOverlayEmpty(t *testing.T) {
	base := JobConfig{Script: []string{"setup"}}
	overlay := JobConfig{}
	out := mergeJobs(base, overlay)
	if len(out.Script) != 1 || out.Script[0] != "setup" {
		t.Errorf("expected base script kept, got %v", out.Script)
	}
}

func TestMergeJobsVariablesDeepMerge(t *testing.T) {
	base := JobConfig{Variables: map[string]string{"A": "1", "B": "base"}}
	overlay := JobConfig{Variables: map[string]string{"B": "overlay", "C": "3"}}
	out := mergeJobs(base, overlay)
	want := map[string]string{"A": "1", "B": "overlay", "C": "3"}
	for k, v := range want {
		if out.Variables[k] != v {
			t.Errorf("Variables[%s]: expected %q, got %q", k, v, out.Variables[k])
		}
	}
}

func TestMergeJobsVariablesIsolation(t *testing.T) {
	base := JobConfig{Variables: map[string]string{"A": "1"}}
	overlay := JobConfig{Variables: map[string]string{"B": "2"}}
	out := mergeJobs(base, overlay)
	out.Variables["A"] = "mutated"
	if base.Variables["A"] != "1" {
		t.Errorf("mergeJobs leaked mutation into base.Variables: %v", base.Variables)
	}
}

func TestMergeJobsPointerFields(t *testing.T) {
	cacheBase := &CacheConfig{Key: "k1"}
	cacheOverlay := &CacheConfig{Key: "k2"}
	out := mergeJobs(JobConfig{Cache: cacheBase}, JobConfig{Cache: cacheOverlay})
	if out.Cache != cacheOverlay {
		t.Errorf("expected overlay cache to win")
	}
	out2 := mergeJobs(JobConfig{Cache: cacheBase}, JobConfig{})
	if out2.Cache != cacheBase {
		t.Errorf("expected base cache to be kept when overlay is nil")
	}
}

func TestMergeJobsParallelPointerSemantics(t *testing.T) {
	tru, fal := true, false

	// Explicit false in overlay overrides true in base.
	out := mergeJobs(JobConfig{Parallel: &tru}, JobConfig{Parallel: &fal})
	if out.Parallel == nil || *out.Parallel {
		t.Error("expected overlay parallel:false to override base parallel:true")
	}

	// Nil overlay keeps base's value.
	out = mergeJobs(JobConfig{Parallel: &tru}, JobConfig{Parallel: nil})
	if out.Parallel == nil || !*out.Parallel {
		t.Error("expected base parallel:true to be preserved when overlay is nil")
	}

	// Overlay true wins over base nil.
	out = mergeJobs(JobConfig{}, JobConfig{Parallel: &tru})
	if out.Parallel == nil || !*out.Parallel {
		t.Error("expected overlay parallel:true to win over nil base")
	}

	// Both nil stays nil.
	out = mergeJobs(JobConfig{}, JobConfig{})
	if out.Parallel != nil {
		t.Error("expected nil when neither side sets parallel")
	}
}

func TestResolveJobSingleExtend(t *testing.T) {
	registry := map[string]*JobConfig{
		".base": {Name: ".base", Image: "alpine", Stage: "build"},
	}
	job := JobConfig{Name: "Build", Extends: ExtendsList{".base"}, Script: []string{"go build"}}
	out, err := resolveJob(job, registry, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Image != "alpine" {
		t.Errorf("expected inherited image alpine, got %q", out.Image)
	}
	if out.Stage != "build" {
		t.Errorf("expected inherited stage build, got %q", out.Stage)
	}
	if len(out.Script) != 1 || out.Script[0] != "go build" {
		t.Errorf("expected local script kept, got %v", out.Script)
	}
	if out.Extends != nil {
		t.Errorf("expected Extends cleared after resolution, got %v", out.Extends)
	}
}

func TestResolveJobMultipleExtendsLeftToRight(t *testing.T) {
	registry := map[string]*JobConfig{
		".a": {Name: ".a", Image: "alpine", Variables: map[string]string{"X": "from-a", "A": "1"}},
		".b": {Name: ".b", Image: "ubuntu", Variables: map[string]string{"X": "from-b", "B": "2"}},
	}
	job := JobConfig{Name: "Job", Extends: ExtendsList{".a", ".b"}}
	out, err := resolveJob(job, registry, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Image != "ubuntu" {
		t.Errorf("expected later template to win, got image %q", out.Image)
	}
	if out.Variables["X"] != "from-b" {
		t.Errorf("expected X=from-b (later template wins), got %q", out.Variables["X"])
	}
	if out.Variables["A"] != "1" || out.Variables["B"] != "2" {
		t.Errorf("expected vars from both templates, got %v", out.Variables)
	}
}

func TestResolveJobTransitiveExtends(t *testing.T) {
	registry := map[string]*JobConfig{
		".base":  {Name: ".base", Image: "alpine"},
		".mid":   {Name: ".mid", Extends: ExtendsList{".base"}, Workdir: "/app"},
		".outer": {Name: ".outer", Extends: ExtendsList{".mid"}, Stage: "build"},
	}
	job := JobConfig{Name: "Job", Extends: ExtendsList{".outer"}, Script: []string{"go build"}}
	out, err := resolveJob(job, registry, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Image != "alpine" || out.Workdir != "/app" || out.Stage != "build" {
		t.Errorf("transitive resolution incomplete: %+v", out)
	}
}

func TestResolveJobCycleDetection(t *testing.T) {
	registry := map[string]*JobConfig{
		".a": {Name: ".a", Extends: ExtendsList{".b"}},
		".b": {Name: ".b", Extends: ExtendsList{".a"}},
	}
	job := JobConfig{Name: "Job", Extends: ExtendsList{".a"}}
	_, err := resolveJob(job, registry, nil)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("expected error to mention 'circular', got %v", err)
	}
}

func TestResolveJobUnknownTemplate(t *testing.T) {
	job := JobConfig{Name: "Job", Extends: ExtendsList{".nope"}}
	_, err := resolveJob(job, map[string]*JobConfig{}, nil)
	if err == nil {
		t.Fatal("expected unknown-template error")
	}
}

func TestResolveAllExtendsRemovesTemplates(t *testing.T) {
	cfg := &Config{
		Jobs: []JobConfig{
			{Name: ".base", Image: "alpine"},
			{Name: "Build", Extends: ExtendsList{".base"}, Stage: "build", Script: []string{"x"}},
		},
	}
	if err := resolveAllExtends(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Jobs) != 1 {
		t.Fatalf("expected templates to be removed, got %d jobs", len(cfg.Jobs))
	}
	if cfg.Jobs[0].Name != "Build" || cfg.Jobs[0].Image != "alpine" {
		t.Errorf("expected resolved Build job, got %+v", cfg.Jobs[0])
	}
}

func TestResolveAllExtendsAllowsNonTemplateBase(t *testing.T) {
	// extends should resolve any name in the registry, even a non-template job.
	cfg := &Config{
		Jobs: []JobConfig{
			{Name: "RealJob", Image: "alpine", Stage: "build", Script: []string{"a"}},
			{Name: "Copy", Extends: ExtendsList{"RealJob"}, Script: []string{"b"}},
		},
	}
	if err := resolveAllExtends(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var copyJob *JobConfig
	for i := range cfg.Jobs {
		if cfg.Jobs[i].Name == "Copy" {
			copyJob = &cfg.Jobs[i]
		}
	}
	if copyJob == nil {
		t.Fatal("Copy job missing after resolution")
	}
	if copyJob.Image != "alpine" || copyJob.Stage != "build" {
		t.Errorf("Copy did not inherit from RealJob: %+v", copyJob)
	}
}

func TestIsTemplate(t *testing.T) {
	if !isTemplate(".foo") {
		t.Error("expected .foo to be a template")
	}
	if isTemplate("foo") {
		t.Error("expected foo to not be a template")
	}
	if isTemplate("") {
		t.Error("expected empty name to not be a template")
	}
}
