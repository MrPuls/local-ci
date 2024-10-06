package config

import (
	"gopkg.in/yaml.v3"
	"os"
)

type StepConfig struct {
	Image  string   `yaml:"image"`
	Script []string `yaml:"script"`
	Step   string   `yaml:"step"`
}

type Config struct {
	Steps  []string              `yaml:"steps"`
	Blocks map[string]StepConfig `yaml:",inline,omitempty"`
}

func (c *Config) GetConfig() error {
	yamlFile, err := os.ReadFile("foo.yaml")
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		return err
	}
	return nil
}
