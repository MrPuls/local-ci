package config

import (
	"fmt"
	"strconv"
	"time"

	"go.yaml.in/yaml/v4"
)

// Duration is a YAML-friendly wrapper over time.Duration. It accepts either a
// Go duration string ("90s", "10m", "1h30m") or a bare integer, which is read
// as seconds.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("duration must be a string like \"10m\" or integer seconds")
	}
	if n, err := strconv.Atoi(node.Value); err == nil {
		*d = Duration(time.Duration(n) * time.Second)
		return nil
	}
	v, err := time.ParseDuration(node.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q (use \"90s\", \"10m\", \"1h30m\" or integer seconds)", node.Value)
	}
	*d = Duration(v)
	return nil
}

func (d Duration) Std() time.Duration { return time.Duration(d) }
