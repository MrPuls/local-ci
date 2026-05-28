package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

const stagePlaceholderPrefix = "."

// stageSource records an included file and the stages it declares, used to
// resolve stage placeholders in the main config's stage list.
type stageSource struct {
	path   string
	stages []string
}

func hasStagePlaceholder(stages []string) bool {
	for _, s := range stages {
		if strings.HasPrefix(s, stagePlaceholderPrefix) {
			return true
		}
	}
	return false
}

// recordDirectStages registers an included file's resolved stages under both
// its basename (without extension) and its full filename, so a stage
// placeholder can reference either form. Duplicate registrations of the same
// file path are ignored. A nil dst (non-main load) is a no-op.
func recordDirectStages(dst map[string][]stageSource, childAbs string, stages []string) {
	if dst == nil {
		return
	}
	src := stageSource{path: childAbs, stages: stages}
	filename := filepath.Base(childAbs)
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	appendStageSource(dst, base, src)
	if filename != base {
		appendStageSource(dst, filename, src)
	}
}

func appendStageSource(dst map[string][]stageSource, key string, src stageSource) {
	for _, s := range dst[key] {
		if s.path == src.path {
			return
		}
	}
	dst[key] = append(dst[key], src)
}

// expandStagePlaceholders replaces each `.name` entry in the stage list with
// the stages declared by the included file matching `name`, splicing them in
// at the placeholder's position. Real stage names pass through unchanged and
// the result is de-duplicated keeping first occurrence. It is a no-op when the
// list contains no placeholders.
func expandStagePlaceholders(stages []string, directStages map[string][]stageSource) ([]string, error) {
	if !hasStagePlaceholder(stages) {
		return stages, nil
	}

	var out []string
	seen := make(map[string]bool)
	add := func(name string) {
		if !seen[name] {
			out = append(out, name)
			seen[name] = true
		}
	}

	for _, s := range stages {
		if !strings.HasPrefix(s, stagePlaceholderPrefix) {
			add(s)
			continue
		}
		ref := strings.TrimPrefix(s, stagePlaceholderPrefix)
		sources := directStages[ref]
		switch len(sources) {
		case 0:
			return nil, fmt.Errorf("stage placeholder %q references no included file named %q", s, ref)
		case 1:
			for _, st := range sources[0].stages {
				add(st)
			}
		default:
			paths := make([]string, len(sources))
			for i, src := range sources {
				paths[i] = src.path
			}
			if strings.HasSuffix(ref, ".yaml") || strings.HasSuffix(ref, ".yml") {
				return nil, fmt.Errorf(
					"stage placeholder %q is ambiguous: %d included files resolve to that name in different directories (%v); rename one to disambiguate",
					s, len(sources), paths)
			}
			return nil, fmt.Errorf(
				"stage placeholder %q is ambiguous: %d included files are named %q (%v); set the full file name (e.g. %q) to disambiguate",
				s, len(sources), ref, paths, s+".yaml")
		}
	}
	return out, nil
}
