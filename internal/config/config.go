package config

import (
	"fmt"
	"log"
	"os"
	"slices"

	"go.yaml.in/yaml/v4"
)

type NetworkConfig struct {
	HostAccess bool `yaml:"host_access,omitempty"`
	HostMode   bool `yaml:"host_mode,omitempty"`
}

type CacheConfig struct {
	Key   string   `yaml:"key"`
	Paths []string `yaml:"paths"`
}

type RemoteProvider struct {
	Url       string `yaml:"url"`
	ProjectId int    `yaml:"project_id"`
	Token     string `yaml:"access_token"`
}

type BootstrapConfig struct {
	Run     []string `yaml:"run"`
	Timeout int      `yaml:"timeout,omitempty"`
}

type CleanupConfig struct {
	Run     []string `yaml:"run"`
	Timeout int      `yaml:"timeout,omitempty"`
}

type JobConfig struct {
	Name      string            `yaml:"-"`
	Image     string            `yaml:"image"`
	Script    []string          `yaml:"script"`
	Stage     string            `yaml:"stage"`
	Workdir   string            `yaml:"workdir,omitempty"`
	Variables map[string]string `yaml:"variables,omitempty"`
	Cache     *CacheConfig      `yaml:"cache,omitempty"`
	Network   *NetworkConfig    `yaml:"network,omitempty"`
}

type Config struct {
	FileName        string
	Stages          []string          `yaml:"stages"`
	Jobs            []JobConfig       `yaml:"-"`
	GlobalVariables map[string]string `yaml:"variables,omitempty"`
	RemoteProvider  *RemoteProvider   `yaml:"remote_provider,omitempty"`
	CLIVariables    map[string]string `yaml:"-"`
	Bootstrap       *BootstrapConfig  `yaml:"bootstrap,omitempty"`
	Cleanup         *CleanupConfig    `yaml:"cleanup,omitempty"`
}

func NewConfig(file string) *Config {
	return &Config{
		FileName: file,
	}
}

func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	type Alias Config
	nonJobFields := []string{"stages", "bootstrap", "cleanup", "variables", "remote_provider"}
	alias := (*Alias)(c) // to avoid recursion

	var raw map[string]any
	if err := node.Decode(&raw); err != nil {
		return err
	}

	if stages, ok := raw["stages"].([]any); ok {
		for _, stage := range stages {
			alias.Stages = append(alias.Stages, stage.(string))
		}
	}

	if bootstrap, ok := raw["bootstrap"].(map[string]any); ok {
		alias.Bootstrap = &BootstrapConfig{
			Timeout: 0,
			Run:     []string{},
		}

		if timeout, ok := bootstrap["timeout"]; ok {
			alias.Bootstrap.Timeout = timeout.(int)
		}

		for _, cmd := range bootstrap["run"].([]any) {
			alias.Bootstrap.Run = append(alias.Bootstrap.Run, cmd.(string))
		}
	}

	if cleanup, ok := raw["cleanup"].(map[string]any); ok {
		alias.Cleanup = &CleanupConfig{
			Timeout: 0,
			Run:     []string{},
		}

		if timeout, ok := cleanup["timeout"]; ok {
			alias.Cleanup.Timeout = timeout.(int)
		}

		for _, cmd := range cleanup["run"].([]any) {
			alias.Cleanup.Run = append(alias.Cleanup.Run, cmd.(string))
		}
	}

	if variables, ok := raw["variables"].(map[string]any); ok {
		log.Println("Collecting global variables...")
		alias.GlobalVariables = make(map[string]string)
		for k, v := range variables {
			alias.GlobalVariables[k] = v.(string)
		}
	}

	if provider, ok := raw["remote_provider"].(map[string]any); ok {
		log.Println("Remote provider found, variables will be fetched from GitLab project...")
		alias.RemoteProvider = &RemoteProvider{
			Url:       provider["url"].(string),
			ProjectId: provider["project_id"].(int),
			Token:     provider["access_token"].(string),
		}
	}

	if node.Kind == yaml.MappingNode {
		log.Println("Collecting jobs...")
		for i := 0; i < len(node.Content); i += 2 { // to iterate over keys only
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			var key string
			if err := keyNode.Decode(&key); err != nil {
				continue
			}

			if slices.Contains(nonJobFields, key) {
				continue
			}

			var job JobConfig
			if err := valueNode.Decode(&job); err != nil {
				return fmt.Errorf("failed to decode job %s: %w", key, err)
			}

			job.Name = key

			// local variables take precedence
			if job.Variables == nil && len(alias.GlobalVariables) > 0 {
				job.Variables = make(map[string]string)
			}
			for k, v := range alias.GlobalVariables {
				if _, ok := job.Variables[k]; !ok {
					job.Variables[k] = v
				}
			}
			// Workdir default value setup
			if job.Workdir == "" {
				job.Workdir = "/"
			}
			alias.Jobs = append(alias.Jobs, job)
		}
	}

	return nil
}

func (c *Config) LoadConfig() error {
	yamlFile, err := os.ReadFile(c.FileName)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return fmt.Errorf(
			"error reading config file, please make sure that all stages are correctly defined\n %w", err)
	}
	return nil
}
