package config

import (
	"fmt"
	"slices"
)

func ValidateConfig(cfg Config) error {
	steps := cfg.Steps
	blocks := cfg.Blocks
	if len(steps) == 0 {
		return fmt.Errorf("[YAML] %s config file has no steps defined. "+
			"Please add at least one step."+
			"\nExample:\n\nsteps:\n  - foo <- step name goes here\n", cfg.FileName)
	}
	for item := range blocks {
		if !slices.Contains(steps, blocks[item].Step) {
			return fmt.Errorf(
				"[YAML] %s block uses undefined step: \"%s\"! Available steps are: %v",
				item, blocks[item].Step, steps,
			)
		}
		if len(blocks[item].Script) == 0 {
			return fmt.Errorf(
				"[YAML] %s block does not have any scripts defined. "+
					"Please add at least one script."+
					"\nExample:\n\nsteps:\n  - echo \"Hello World!\" <- script code goes here\n", item,
			)
		}
	}
	return nil
}
