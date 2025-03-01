package config

import (
	"fmt"
	"slices"
)

func ValidateConfig(cfg *Config, jobName string) error {
	stages := cfg.Stages
	blocks := cfg.Jobs

	if len(stages) == 0 {
		return fmt.Errorf("[YAML] %s config file has no stages defined. "+
			"Please add at least one stage."+
			"\nExample:\n\nstages:\n  - foo <- stage name goes here\n", cfg.FileName)
	}
	if jobName != "" {
		if _, ok := blocks[jobName]; !ok {
			return fmt.Errorf("[YAML] %s config file has no job named %s. ", cfg.FileName, jobName)
		}
	}

	for k, v := range blocks {
		if v.Stage == "" {
			return fmt.Errorf("[YAML] Stage is empty or undefined in block \"%s\"", k)
		}
		if !slices.Contains(stages, v.Stage) {
			return fmt.Errorf(
				"[YAML] \"%s\" block uses undefined step: \"%s\"! Available stages are: %v",
				k, v.Stage, stages,
			)
		}
		if len(v.Script) == 0 {
			return fmt.Errorf(
				"[YAML] \"%s\" block uses a \"script\" field, but no scripts were defined. "+
					"Please add at least one script."+
					"\nExample:\n\nstages:\n  - echo \"Hello World!\" <- script code goes here\n", k,
			)
		}
		if v.Image == "" {
			return fmt.Errorf("[YAML] Image is empty or undefined in block \"%s\"\n", k)
		}
		if v.Cache != nil {
			if v.Cache.Key == "" {
				return fmt.Errorf("[YAML] Cache must have a key defined in block \"%s\"\n", k)
			}
			if len(v.Cache.Paths) == 0 {
				return fmt.Errorf("[YAML] Cache must have at least one path defined in block \"%s\"\n", k)
			}
			if slices.Contains(v.Cache.Paths, "") {
				return fmt.Errorf("[YAML] Cache can't include an empty path in block \"%s\"\n", k)
			}
		}
	}
	return nil
}
