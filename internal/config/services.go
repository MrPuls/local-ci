package config

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"
)

// ServiceConfig declares a sidecar container that runs for the duration of a
// job (a database, a cache, ...). The engine starts it on a per-job network
// before the job's script and tears it down after, so the job reaches it at
// its alias (e.g. postgres://db:5432). Unlike job_bootstrap — host-side
// commands that run to completion — a service is an engine-managed daemon
// with its own image, lifecycle, and readiness gate.
type ServiceConfig struct {
	Image     string            `yaml:"image"`
	Alias     string            `yaml:"alias,omitempty"`
	Variables map[string]string `yaml:"variables,omitempty"`
	Ready     *ServiceReady     `yaml:"ready,omitempty"`
}

// ServiceReady gates the job's start on the service being usable. With a
// command, the engine execs it inside the service container until it exits 0.
// Without one, it waits for the image's HEALTHCHECK (when defined) or for the
// container to be running.
type ServiceReady struct {
	Command string   `yaml:"command,omitempty"`
	Timeout Duration `yaml:"timeout,omitempty"`
}

// UnmarshalYAML accepts either the full mapping form or the string shorthand
// `- postgres:16`, which sets the image and derives the alias.
func (s *ServiceConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		s.Image = node.Value
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("service entry must be an image string or a mapping with an 'image' field")
	}
	type alias ServiceConfig // drop methods to avoid recursion
	return node.Decode((*alias)(s))
}

// EffectiveAlias is the network hostname the job reaches the service at: the
// explicit alias, or the image's last path segment without tag/digest
// (registry.example.com/group/postgres:16 → "postgres").
func (s *ServiceConfig) EffectiveAlias() string {
	if s.Alias != "" {
		return s.Alias
	}
	name := s.Image
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.IndexAny(name, ":@"); i >= 0 {
		name = name[:i]
	}
	return name
}
