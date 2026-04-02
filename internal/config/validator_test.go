package config

import (
	"testing"
)

func TestValidateStages_Empty(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{}}
	v := NewConfigValidator(cfg)
	if err := v.validateStages(); err == nil {
		t.Error("expected error for empty stages")
	}
}

func TestValidateStages_Valid(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build", "test"}}
	v := NewConfigValidator(cfg)
	if err := v.validateStages(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateBootstrap_Nil(t *testing.T) {
	cfg := &Config{FileName: "test.yaml"}
	v := NewConfigValidator(cfg)
	if err := v.validateBootstrap(); err != nil {
		t.Errorf("unexpected error for nil bootstrap: %v", err)
	}
}

func TestValidateBootstrap_EmptyRun(t *testing.T) {
	cfg := &Config{
		FileName:  "test.yaml",
		Bootstrap: &BootstrapConfig{Run: []string{}},
	}
	v := NewConfigValidator(cfg)
	if err := v.validateBootstrap(); err == nil {
		t.Error("expected error for empty bootstrap run")
	}
}

func TestValidateBootstrap_NegativeTimeout(t *testing.T) {
	cfg := &Config{
		FileName:  "test.yaml",
		Bootstrap: &BootstrapConfig{Run: []string{"echo hi"}, Timeout: -1},
	}
	v := NewConfigValidator(cfg)
	if err := v.validateBootstrap(); err == nil {
		t.Error("expected error for negative timeout")
	}
}

func TestValidateBootstrap_Valid(t *testing.T) {
	cfg := &Config{
		FileName:  "test.yaml",
		Bootstrap: &BootstrapConfig{Run: []string{"echo hi"}, Timeout: 5},
	}
	v := NewConfigValidator(cfg)
	if err := v.validateBootstrap(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateCleanup_RequiresBootstrap(t *testing.T) {
	cfg := &Config{
		FileName: "test.yaml",
		Cleanup:  &CleanupConfig{Run: []string{"echo bye"}},
	}
	v := NewConfigValidator(cfg)
	if err := v.validateCleanup(); err == nil {
		t.Error("expected error for cleanup without bootstrap")
	}
}

func TestValidateCleanup_EmptyRun(t *testing.T) {
	cfg := &Config{
		FileName:  "test.yaml",
		Bootstrap: &BootstrapConfig{Run: []string{"echo hi"}},
		Cleanup:   &CleanupConfig{Run: []string{}},
	}
	v := NewConfigValidator(cfg)
	if err := v.validateCleanup(); err == nil {
		t.Error("expected error for empty cleanup run")
	}
}

func TestValidateCleanup_NegativeTimeout(t *testing.T) {
	cfg := &Config{
		FileName:  "test.yaml",
		Bootstrap: &BootstrapConfig{Run: []string{"echo hi"}},
		Cleanup:   &CleanupConfig{Run: []string{"echo bye"}, Timeout: -1},
	}
	v := NewConfigValidator(cfg)
	if err := v.validateCleanup(); err == nil {
		t.Error("expected error for negative cleanup timeout")
	}
}

func TestValidateCleanup_Valid(t *testing.T) {
	cfg := &Config{
		FileName:  "test.yaml",
		Bootstrap: &BootstrapConfig{Run: []string{"echo hi"}},
		Cleanup:   &CleanupConfig{Run: []string{"echo bye"}, Timeout: 5},
	}
	v := NewConfigValidator(cfg)
	if err := v.validateCleanup(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateRemoteProvider_Nil(t *testing.T) {
	cfg := &Config{FileName: "test.yaml"}
	v := NewConfigValidator(cfg)
	if err := v.validateRemoteProvider(); err != nil {
		t.Errorf("unexpected error for nil remote provider: %v", err)
	}
}

func TestValidateRemoteProvider_EmptyUrl(t *testing.T) {
	cfg := &Config{
		FileName:       "test.yaml",
		RemoteProvider: &RemoteProvider{Url: "", ProjectId: 1, Token: "tok"},
	}
	v := NewConfigValidator(cfg)
	if err := v.validateRemoteProvider(); err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestValidateRemoteProvider_EmptyProjectId(t *testing.T) {
	cfg := &Config{
		FileName:       "test.yaml",
		RemoteProvider: &RemoteProvider{Url: "gitlab.com", ProjectId: 0, Token: "tok"},
	}
	v := NewConfigValidator(cfg)
	if err := v.validateRemoteProvider(); err == nil {
		t.Error("expected error for zero project ID")
	}
}

func TestValidateRemoteProvider_EmptyToken(t *testing.T) {
	cfg := &Config{
		FileName:       "test.yaml",
		RemoteProvider: &RemoteProvider{Url: "gitlab.com", ProjectId: 1, Token: ""},
	}
	v := NewConfigValidator(cfg)
	if err := v.validateRemoteProvider(); err == nil {
		t.Error("expected error for empty token")
	}
}

func TestValidateJob_MissingStage(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{Name: "test", Image: "alpine", Script: []string{"echo hi"}}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for missing stage")
	}
}

func TestValidateJob_UndefinedStage(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{Name: "test", Stage: "deploy", Image: "alpine", Script: []string{"echo hi"}}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for undefined stage")
	}
}

func TestValidateJob_MissingScript(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{Name: "test", Stage: "build", Image: "alpine", Script: []string{}}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for missing script")
	}
}

func TestValidateJob_MissingImage(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{Name: "test", Stage: "build", Script: []string{"echo hi"}}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for missing image")
	}
}

func TestValidateJob_JobBootstrapEmptyRun(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{
		Name: "test", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
		JobBootstrap: &JobBootstrapConfig{Run: []string{}},
	}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for empty job_bootstrap run")
	}
}

func TestValidateJob_JobBootstrapNegativeTimeout(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{
		Name: "test", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
		JobBootstrap: &JobBootstrapConfig{Run: []string{"echo setup"}, Timeout: -1},
	}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for negative job_bootstrap timeout")
	}
}

func TestValidateJob_JobBootstrapValid(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{
		Name: "test", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
		JobBootstrap: &JobBootstrapConfig{Run: []string{"echo setup"}, Timeout: 5},
	}
	if err := v.validateJob(job); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateJob_JobCleanupRequiresBootstrap(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{
		Name: "test", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
		JobCleanup: &JobCleanupConfig{Run: []string{"echo teardown"}},
	}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for job_cleanup without job_bootstrap")
	}
}

func TestValidateJob_JobCleanupEmptyRun(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{
		Name: "test", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
		JobBootstrap: &JobBootstrapConfig{Run: []string{"echo setup"}},
		JobCleanup:   &JobCleanupConfig{Run: []string{}},
	}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for empty job_cleanup run")
	}
}

func TestValidateJob_JobCleanupNegativeTimeout(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{
		Name: "test", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
		JobBootstrap: &JobBootstrapConfig{Run: []string{"echo setup"}},
		JobCleanup:   &JobCleanupConfig{Run: []string{"echo teardown"}, Timeout: -1},
	}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for negative job_cleanup timeout")
	}
}

func TestValidateJob_JobCleanupValid(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{
		Name: "test", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
		JobBootstrap: &JobBootstrapConfig{Run: []string{"echo setup"}},
		JobCleanup:   &JobCleanupConfig{Run: []string{"echo teardown"}, Timeout: 5},
	}
	if err := v.validateJob(job); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateJob_CacheMissingKey(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{
		Name: "test", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
		Cache: &CacheConfig{Paths: []string{"/tmp"}},
	}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for cache without key")
	}
}

func TestValidateJob_CacheEmptyPaths(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{
		Name: "test", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
		Cache: &CacheConfig{Key: "k", Paths: []string{}},
	}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for cache with empty paths")
	}
}

func TestValidateJob_CacheEmptyPathEntry(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{
		Name: "test", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
		Cache: &CacheConfig{Key: "k", Paths: []string{"/tmp", ""}},
	}
	if err := v.validateJob(job); err == nil {
		t.Error("expected error for cache with empty path entry")
	}
}

func TestValidateJob_Valid(t *testing.T) {
	cfg := &Config{FileName: "test.yaml", Stages: []string{"build"}}
	v := NewConfigValidator(cfg)
	job := &JobConfig{
		Name: "test", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
		Cache: &CacheConfig{Key: "deps", Paths: []string{"/tmp"}},
	}
	if err := v.validateJob(job); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_RejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "no stages",
			cfg: &Config{
				FileName: "test.yaml",
				Stages:   []string{},
				Jobs: []JobConfig{
					{Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo hi"}},
				},
			},
		},
		{
			name: "job references undefined stage",
			cfg: &Config{
				FileName: "test.yaml",
				Stages:   []string{"build"},
				Jobs: []JobConfig{
					{Name: "Deploy", Stage: "deploy", Image: "alpine", Script: []string{"echo hi"}},
				},
			},
		},
		{
			name: "job missing image",
			cfg: &Config{
				FileName: "test.yaml",
				Stages:   []string{"build"},
				Jobs: []JobConfig{
					{Name: "Build", Stage: "build", Script: []string{"echo hi"}},
				},
			},
		},
		{
			name: "job missing script",
			cfg: &Config{
				FileName: "test.yaml",
				Stages:   []string{"build"},
				Jobs: []JobConfig{
					{Name: "Build", Stage: "build", Image: "alpine", Script: []string{}},
				},
			},
		},
		{
			name: "bootstrap with empty run",
			cfg: &Config{
				FileName:  "test.yaml",
				Stages:    []string{"build"},
				Bootstrap: &BootstrapConfig{Run: []string{}},
				Jobs: []JobConfig{
					{Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo hi"}},
				},
			},
		},
		{
			name: "cleanup without bootstrap",
			cfg: &Config{
				FileName: "test.yaml",
				Stages:   []string{"build"},
				Cleanup:  &CleanupConfig{Run: []string{"echo bye"}},
				Jobs: []JobConfig{
					{Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo hi"}},
				},
			},
		},
		{
			name: "bootstrap negative timeout",
			cfg: &Config{
				FileName:  "test.yaml",
				Stages:    []string{"build"},
				Bootstrap: &BootstrapConfig{Run: []string{"echo hi"}, Timeout: -1},
				Jobs: []JobConfig{
					{Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo hi"}},
				},
			},
		},
		{
			name: "remote provider missing url",
			cfg: &Config{
				FileName:       "test.yaml",
				Stages:         []string{"build"},
				RemoteProvider: &RemoteProvider{Url: "", ProjectId: 1, Token: "tok"},
				Jobs: []JobConfig{
					{Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo hi"}},
				},
			},
		},
		{
			name: "job bootstrap with empty run",
			cfg: &Config{
				FileName: "test.yaml",
				Stages:   []string{"build"},
				Jobs: []JobConfig{
					{
						Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
						JobBootstrap: &JobBootstrapConfig{Run: []string{}},
					},
				},
			},
		},
		{
			name: "job bootstrap negative timeout",
			cfg: &Config{
				FileName: "test.yaml",
				Stages:   []string{"build"},
				Jobs: []JobConfig{
					{
						Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
						JobBootstrap: &JobBootstrapConfig{Run: []string{"echo setup"}, Timeout: -1},
					},
				},
			},
		},
		{
			name: "job cleanup without job bootstrap",
			cfg: &Config{
				FileName: "test.yaml",
				Stages:   []string{"build"},
				Jobs: []JobConfig{
					{
						Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
						JobCleanup: &JobCleanupConfig{Run: []string{"echo teardown"}},
					},
				},
			},
		},
		{
			name: "job cleanup with empty run",
			cfg: &Config{
				FileName: "test.yaml",
				Stages:   []string{"build"},
				Jobs: []JobConfig{
					{
						Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
						JobBootstrap: &JobBootstrapConfig{Run: []string{"echo setup"}},
						JobCleanup:   &JobCleanupConfig{Run: []string{}},
					},
				},
			},
		},
		{
			name: "job cleanup negative timeout",
			cfg: &Config{
				FileName: "test.yaml",
				Stages:   []string{"build"},
				Jobs: []JobConfig{
					{
						Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
						JobBootstrap: &JobBootstrapConfig{Run: []string{"echo setup"}},
						JobCleanup:   &JobCleanupConfig{Run: []string{"echo teardown"}, Timeout: -1},
					},
				},
			},
		},
		{
			name: "cache missing key",
			cfg: &Config{
				FileName: "test.yaml",
				Stages:   []string{"build"},
				Jobs: []JobConfig{
					{
						Name: "Build", Stage: "build", Image: "alpine", Script: []string{"echo hi"},
						Cache: &CacheConfig{Paths: []string{"/tmp"}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateConfig(tt.cfg); err == nil {
				t.Errorf("expected validation to reject config, but it passed")
			}
		})
	}
}

func TestValidate_FullPipeline(t *testing.T) {
	cfg := &Config{
		FileName: "test.yaml",
		Stages:   []string{"build", "test"},
		Bootstrap: &BootstrapConfig{
			Run:     []string{"docker compose up -d"},
			Timeout: 5,
		},
		Cleanup: &CleanupConfig{
			Run:     []string{"docker compose down"},
			Timeout: 5,
		},
		Jobs: []JobConfig{
			{Name: "Build", Stage: "build", Image: "golang:1.21", Script: []string{"go build"}},
			{Name: "Test", Stage: "test", Image: "golang:1.21", Script: []string{"go test ./..."}},
		},
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_FullPipelineWithJobBootstrapCleanup(t *testing.T) {
	cfg := &Config{
		FileName: "test.yaml",
		Stages:   []string{"build", "test"},
		Bootstrap: &BootstrapConfig{
			Run:     []string{"docker compose up -d"},
			Timeout: 5,
		},
		Cleanup: &CleanupConfig{
			Run:     []string{"docker compose down"},
			Timeout: 5,
		},
		Jobs: []JobConfig{
			{
				Name: "Build", Stage: "build", Image: "golang:1.21", Script: []string{"go build"},
				JobBootstrap: &JobBootstrapConfig{Run: []string{"echo setup"}, Timeout: 3},
				JobCleanup:   &JobCleanupConfig{Run: []string{"echo teardown"}, Timeout: 2},
			},
			{Name: "Test", Stage: "test", Image: "golang:1.21", Script: []string{"go test ./..."}},
		},
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
