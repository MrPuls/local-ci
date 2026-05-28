package config

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v4"
)

// loadConfigWithIncludes reads the file at absPath, recursively loads any
// `include:`d files relative to absPath's directory, and merges the resulting
// configs. The including file wins over all of its includes; among multiple
// includes, later entries win over earlier ones.
//
// inProgress tracks the files on the current load path so true cycles are
// rejected while diamond includes (the same file pulled in via two different
// branches) are allowed.
//
// directStages is non-nil only for the top-level (main config) call. When set,
// each direct include's resolved stages are recorded into it so the main
// file's stage placeholders can be expanded afterward. Recursive calls pass
// nil, which also marks them as non-main: a non-main file is not allowed to
// declare stage placeholders.
func loadConfigWithIncludes(absPath string, inProgress map[string]bool, directStages map[string][]stageSource) (*Config, error) {
	if inProgress[absPath] {
		return nil, fmt.Errorf("circular include: %s already in load chain", absPath)
	}
	inProgress[absPath] = true
	defer delete(inProgress, absPath)

	yamlFile, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read include %q: %w", absPath, err)
	}
	cfg := &Config{FileName: absPath}
	if err := yaml.Unmarshal(yamlFile, cfg); err != nil {
		return nil, fmt.Errorf("parse include %q: %w", absPath, err)
	}

	if directStages == nil && hasStagePlaceholder(cfg.Stages) {
		return nil, fmt.Errorf("stage placeholders are only supported in the main config file, found in %q", absPath)
	}

	baseDir := filepath.Dir(absPath)
	// Merge later includes first. mergeConfigInto keeps whatever the
	// accumulator already has, so processing back-to-front makes a later
	// include win over an earlier one on conflict, while the including file
	// (already populated in cfg) still wins over every include.
	for i := len(cfg.Include) - 1; i >= 0; i-- {
		childAbs := resolveIncludePath(cfg.Include[i], baseDir)
		child, err := loadConfigWithIncludes(childAbs, inProgress, nil)
		if err != nil {
			return nil, err
		}
		recordDirectStages(directStages, childAbs, child.Stages)
		mergeConfigInto(cfg, child)
	}
	cfg.Include = nil
	return cfg, nil
}

func resolveIncludePath(path, baseDir string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}

// mergeConfigInto adds fields from `from` into `into` only when `into` does
// not already have them. Maps are merged with `into` winning on key conflicts.
// Jobs are appended by name — a job already present in `into` is not
// overwritten.
func mergeConfigInto(into, from *Config) {
	if len(into.Stages) == 0 {
		into.Stages = from.Stages
	}
	if from.GlobalVariables != nil {
		if into.GlobalVariables == nil {
			into.GlobalVariables = map[string]string{}
		}
		for k, v := range from.GlobalVariables {
			if _, ok := into.GlobalVariables[k]; !ok {
				into.GlobalVariables[k] = v
			}
		}
	}
	if into.RemoteProvider == nil {
		into.RemoteProvider = from.RemoteProvider
	}
	if into.Bootstrap == nil {
		into.Bootstrap = from.Bootstrap
	}
	if into.Cleanup == nil {
		into.Cleanup = from.Cleanup
	}

	existing := make(map[string]bool, len(into.Jobs))
	for _, j := range into.Jobs {
		existing[j.Name] = true
	}
	for _, j := range from.Jobs {
		if existing[j.Name] {
			continue
		}
		into.Jobs = append(into.Jobs, j)
		existing[j.Name] = true
	}
}
