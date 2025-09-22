package config

import (
	"fmt"
	"os"

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
}

func NewConfig(file string) *Config {
	return &Config{
		FileName: file,
	}
}

func (c *Config) UnmarshalYAML(node *yaml.Node) error {
	type Alias Config
	alias := (*Alias)(c) // to avoid recursion

	var raw map[string]interface{}
	if err := node.Decode(&raw); err != nil {
		return err
	}

	if stages, ok := raw["stages"].([]interface{}); ok {
		for _, stage := range stages {
			alias.Stages = append(alias.Stages, stage.(string))
		}
	}

	if variables, ok := raw["variables"].(map[string]interface{}); ok {
		alias.GlobalVariables = make(map[string]string)
		for k, v := range variables {
			alias.GlobalVariables[k] = v.(string)
		}
	}

	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 { // to iterate over keys only
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			var key string
			if err := keyNode.Decode(&key); err != nil {
				continue
			}

			if key == "stages" || key == "variables" {
				continue
			}

			var job JobConfig
			if err := valueNode.Decode(&job); err != nil {
				return fmt.Errorf("failed to decode job %s: %w", key, err)
			}

			job.Name = key

			// local variables take precedence
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
