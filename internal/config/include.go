package config

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v4"
)

// loadConfigWithIncludes reads the file at absPath, recursively loads any
// `include:`d files relative to absPath's directory, and merges the resulting
// configs with main-wins semantics.
func loadConfigWithIncludes(absPath string, visited map[string]bool) (*Config, error) {
	if visited[absPath] {
		return nil, fmt.Errorf("circular include: %s already in load chain", absPath)
	}
	visited[absPath] = true

	yamlFile, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read include %q: %w", absPath, err)
	}
	cfg := &Config{FileName: absPath}
	if err := yaml.Unmarshal(yamlFile, cfg); err != nil {
		return nil, fmt.Errorf("parse include %q: %w", absPath, err)
	}

	baseDir := filepath.Dir(absPath)
	for _, inc := range cfg.Include {
		childAbs := resolveIncludePath(inc, baseDir)
		child, err := loadConfigWithIncludes(childAbs, visited)
		if err != nil {
			return nil, err
		}
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
