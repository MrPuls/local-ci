package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

const templatePrefix = "."

func isTemplate(name string) bool {
	return strings.HasPrefix(name, templatePrefix)
}

// resolveAllExtends walks every non-template job in cfg, applies its
// `extends:` chain (recursively, with cycle detection), and removes templates
// from the final job list.
func resolveAllExtends(cfg *Config) error {
	registry := make(map[string]*JobConfig, len(cfg.Jobs))
	for i := range cfg.Jobs {
		registry[cfg.Jobs[i].Name] = &cfg.Jobs[i]
	}

	for i := range cfg.Jobs {
		if isTemplate(cfg.Jobs[i].Name) {
			continue
		}
		resolved, err := resolveJob(cfg.Jobs[i], registry, nil)
		if err != nil {
			return err
		}
		if resolved.Workdir == "" {
			resolved.Workdir = "/"
		}
		cfg.Jobs[i] = resolved
	}

	nonTemplates := cfg.Jobs[:0]
	for _, j := range cfg.Jobs {
		if isTemplate(j.Name) {
			continue
		}
		nonTemplates = append(nonTemplates, j)
	}
	cfg.Jobs = nonTemplates
	return nil
}

// resolveJob returns a new JobConfig with all templates in its Extends chain
// merged in (left-to-right). The consuming job's own fields override any
// template fields on conflict. Cycles are rejected.
func resolveJob(job JobConfig, registry map[string]*JobConfig, seen []string) (JobConfig, error) {
	if slices.Contains(seen, job.Name) {
		return JobConfig{}, fmt.Errorf("circular extends: %q already in chain %v", job.Name, seen)
	}
	if len(job.Extends) == 0 {
		return job, nil
	}
	seen = append(seen, job.Name)

	var merged JobConfig
	for _, parentName := range job.Extends {
		parent, ok := registry[parentName]
		if !ok {
			return JobConfig{}, fmt.Errorf("job %q extends unknown template %q", job.Name, parentName)
		}
		resolvedParent, err := resolveJob(*parent, registry, seen)
		if err != nil {
			return JobConfig{}, err
		}
		merged = mergeJobs(merged, resolvedParent)
	}
	merged = mergeJobs(merged, job)
	merged.Extends = nil
	return merged, nil
}

// mergeJobs returns base with overlay's fields applied. Non-empty scalars,
// non-nil pointers, and non-empty lists from overlay replace base. The
// Variables map is deep-merged with overlay winning on key conflicts; a copy
// is made so base's map is not mutated.
func mergeJobs(base, overlay JobConfig) JobConfig {
	out := base

	if base.Variables != nil {
		copied := make(map[string]string, len(base.Variables))
		maps.Copy(copied, base.Variables)
		out.Variables = copied
	}
	if overlay.Name != "" {
		out.Name = overlay.Name
	}
	if overlay.Image != "" {
		out.Image = overlay.Image
	}
	if len(overlay.Script) > 0 {
		out.Script = overlay.Script
	}
	if overlay.Stage != "" {
		out.Stage = overlay.Stage
	}
	if overlay.Workdir != "" {
		out.Workdir = overlay.Workdir
	}
	if len(overlay.Variables) > 0 {
		if out.Variables == nil {
			out.Variables = make(map[string]string, len(overlay.Variables))
		}
		maps.Copy(out.Variables, overlay.Variables)
	}
	if overlay.Cache != nil {
		out.Cache = overlay.Cache
	}
	if overlay.Network != nil {
		out.Network = overlay.Network
	}
	if overlay.JobBootstrap != nil {
		out.JobBootstrap = overlay.JobBootstrap
	}
	if overlay.JobCleanup != nil {
		out.JobCleanup = overlay.JobCleanup
	}
	if overlay.Parallel != nil {
		out.Parallel = overlay.Parallel
	}
	if len(overlay.Matrix) > 0 {
		out.Matrix = overlay.Matrix
	}
	return out
}
