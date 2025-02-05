package config

import (
	"fmt"
	"slices"
)

func ValidateConfig(cfg Config) error {
	steps := cfg.Stages
	blocks := cfg.Blocks
	if len(steps) == 0 {
		return fmt.Errorf("[YAML] %s config file has no steps defined. "+
			"Please add at least one step."+
			"\nExample:\n\nsteps:\n  - foo <- step name goes here\n", cfg.FileName)
	}
	for item := range blocks {
		if !slices.Contains(steps, blocks[item].Stage) {
			return fmt.Errorf(
				"[YAML] %s block uses undefined step: \"%s\"! Available steps are: %v",
				item, blocks[item].Stage, steps,
			)
		}
		if len(blocks[item].Script) == 0 {
			return fmt.Errorf(
				"[YAML] %s block uses a \"script\" field, but no scripts were defined. "+
					"Please add at least one script."+
					"\nExample:\n\nsteps:\n  - echo \"Hello World!\" <- script code goes here\n", item,
			)
		}
	}
	return nil
}
