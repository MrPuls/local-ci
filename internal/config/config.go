package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type JobConfig struct {
	Image     string            `yaml:"image"`
	Script    []string          `yaml:"script"`
	Stage     string            `yaml:"stage"`
	Workdir   string            `yaml:"workdir,omitempty"`
	Variables map[string]string `yaml:"variables,omitempty"`
	Cache     *CacheConfig      `yaml:"cache,omitempty"`
}

type Config struct {
	FileName        string
	Stages          []string             `yaml:"stages"`
	Jobs            map[string]JobConfig `yaml:",inline"`
	GlobalVariables map[string]string    `yaml:"variables,omitempty"`
}

type CacheConfig struct {
	Key   string   `yaml:"key"`
	Paths []string `yaml:"paths"`
}

func (c *Config) ParseConfig(file string) error {
	yamlFile, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return fmt.Errorf(
			"error reading config file, please make sure that all stages are correctly defined\n %w", err)
	}
	c.FileName = file
	return nil
}
