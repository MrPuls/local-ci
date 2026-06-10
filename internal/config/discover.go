package config

import (
	"os"
	"sort"
	"strings"
)

// CanonicalConfigName is the default config file the CLI and server look for
// when nothing else is selected.
const CanonicalConfigName = ".local-ci.yaml"

// IsConfigFileName reports whether name looks like a local-ci config file:
// the canonical ".local-ci.yaml"/".local-ci.yml", a bare "local-ci.yaml", or
// any "<prefix>.local-ci.yaml" / "<prefix>-local-ci.yaml" /
// "<prefix>_local-ci.yaml" variant (and the .yml spellings). The separator
// requirement keeps unrelated names like "nonlocal-ci.yaml" out.
func IsConfigFileName(name string) bool {
	lower := strings.ToLower(name)
	var base string
	switch {
	case strings.HasSuffix(lower, ".yaml"):
		base = strings.TrimSuffix(lower, ".yaml")
	case strings.HasSuffix(lower, ".yml"):
		base = strings.TrimSuffix(lower, ".yml")
	default:
		return false
	}
	if base == "local-ci" || base == ".local-ci" {
		return true
	}
	prefix, ok := strings.CutSuffix(base, "local-ci")
	if !ok || prefix == "" {
		return false
	}
	switch prefix[len(prefix)-1] {
	case '.', '-', '_':
		return true
	}
	return false
}

// DiscoverConfigs lists the local-ci config files present in dir (basenames
// only, not full paths). The canonical ".local-ci.yaml"/".local-ci.yml" sorts
// first; the rest follow case-insensitively. A missing or unreadable dir is
// reported as an error.
func DiscoverConfigs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !IsConfigFileName(e.Name()) {
			continue
		}
		names = append(names, e.Name())
	}
	canonical := func(n string) bool {
		l := strings.ToLower(n)
		return l == ".local-ci.yaml" || l == ".local-ci.yml"
	}
	sort.SliceStable(names, func(i, j int) bool {
		ci, cj := canonical(names[i]), canonical(names[j])
		if ci != cj {
			return ci
		}
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names, nil
}
