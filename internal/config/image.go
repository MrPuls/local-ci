package config

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"
)

// ImageConfig accepts an image name or a mapping with an explicit pull policy.
// The default is to reuse an image already present in the Docker daemon.
type ImageConfig struct {
	Name      string `yaml:"name"`
	ForcePull bool   `yaml:"force_pull,omitempty"`
}

func (i *ImageConfig) UnmarshalYAML(node *yaml.Node) error {
	var decoded ImageConfig
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Tag != "!!str" {
			return fmt.Errorf("image must be a string or a mapping with a 'name' field")
		}
		decoded.Name = node.Value
	case yaml.MappingNode:
		type plain ImageConfig
		if err := node.Decode((*plain)(&decoded)); err != nil {
			return fmt.Errorf("image: %w", err)
		}
	default:
		return fmt.Errorf("image must be a string or a mapping with a 'name' field")
	}
	if strings.TrimSpace(decoded.Name) == "" {
		return fmt.Errorf("image name must not be empty")
	}
	*i = decoded
	return nil
}

func (i ImageConfig) MarshalYAML() (any, error) {
	if !i.ForcePull {
		return i.Name, nil
	}
	type plain ImageConfig
	return plain(i), nil
}
