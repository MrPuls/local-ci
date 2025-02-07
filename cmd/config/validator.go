package config

import (
	"fmt"
	"slices"
)

func ValidateConfig(cfg Config) error {
	stages := cfg.Stages
	blocks := cfg.Blocks

	if len(stages) == 0 {
		return fmt.Errorf("[YAML] %s config file has no stages defined. "+
			"Please add at least one stage."+
			"\nExample:\n\nstages:\n  - foo <- stage name goes here\n", cfg.FileName)
	}

	for item := range blocks {
		if blocks[item].Stage == "" {
			return fmt.Errorf("[YAML] Stage is empty or undefined in block \"%s\"", item)
		}
		if !slices.Contains(stages, blocks[item].Stage) {
			return fmt.Errorf(
				"[YAML] \"%s\" block uses undefined step: \"%s\"! Available stages are: %v",
				item, blocks[item].Stage, stages,
			)
		}
		if len(blocks[item].Script) == 0 {
			return fmt.Errorf(
				"[YAML] \"%s\" block uses a \"script\" field, but no scripts were defined. "+
					"Please add at least one script."+
					"\nExample:\n\nstages:\n  - echo \"Hello World!\" <- script code goes here\n", item,
			)
		}
		if blocks[item].Image == "" {
			return fmt.Errorf("[YAML] Image is empty or undefined in block \"%s\"\n", item)
		}
	}
	return nil
}
