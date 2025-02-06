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
		if blocks[item].Image == "" {
			return fmt.Errorf("[YAML] %s block uses a \"image\" field, but no images were defined\n", item)
		}
	}
	// TODO: split into different funcs probably, since it could become kinda long, especially with all the error text.
	// 	Also write some tests for that.
	//		Also would be nice to validate the field only in case if its present,
	//		can be done with if gVar, ok := cfg.GlobalVariables; ok {}
	return nil
}
