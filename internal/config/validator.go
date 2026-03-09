package config

import (
	"fmt"
	"slices"

	"github.com/MrPuls/local-ci/internal/integrations/gitlab"
)

func ValidateConfig(cfg *Config) error {
	stages := cfg.Stages
	blocks := cfg.Jobs

	if cfg.RemoteProvider != nil {
		if cfg.RemoteProvider.Url == "" {
			return fmt.Errorf("[YAML] %s config file has an empty remote provider's url. Either specify a valid url or remove the 'remote_provider' field.", cfg.FileName)
		}
		if cfg.RemoteProvider.ProjectId == 0 {
			return fmt.Errorf("[YAML] %s config file has an empty remote provider's project_id. Either specify a valid project_id or remove the 'remote_provider' field.", cfg.FileName)
		}
		if cfg.RemoteProvider.Token == "" {
			return fmt.Errorf("[YAML] %s config file has an empty remote provider's token. Either specify a valid token or remove the 'remote_provider' field.", cfg.FileName)
		}
		options := gitlab.GitlabOptions{
			Url:       cfg.RemoteProvider.Url,
			Token:     cfg.RemoteProvider.Token,
			ProjectId: cfg.RemoteProvider.ProjectId,
		}
		gtl := gitlab.NewGitLabUtil(&options)
		vars := gtl.GetRemoteVariables()
		if vars == nil {
			return fmt.Errorf("[YAML] %s config file has an invalid remote provider configuration. "+
				"Please check the 'remote_provider' field. [Details: Failed to fetch variables]", cfg.FileName)
		}
	}

	if len(stages) == 0 {
		return fmt.Errorf("[YAML] %s config file has no stages defined: %v. "+
			"Please add at least one stage."+
			"\nExample:\n\nstages:\n  - foo <- stage name goes here\n", cfg.FileName, cfg.Stages)
	}

	for _, v := range blocks {
		if v.Stage == "" {
			return fmt.Errorf("[YAML] Stage is empty or undefined in block \"%s\"", v.Name)
		}
		if !slices.Contains(stages, v.Stage) {
			return fmt.Errorf(
				"[YAML] \"%s\" block uses undefined step: \"%s\"! Available stages are: %v",
				v.Name, v.Stage, stages,
			)
		}
		if len(v.Script) == 0 {
			return fmt.Errorf(
				"[YAML] \"%s\" block has no scripts defined. "+
					"Please make sure that you are using the 'script' keyword\n"+
					"If you do, please add at least one script."+
					"\nExample:\n\nscript:\n  - echo \"Hello World!\" <- script code goes here\n", v.Name,
			)
		}
		if v.Image == "" {
			return fmt.Errorf("[YAML] Image is empty or undefined in block \"%s\"\n", v.Name)
		}
		if v.Cache != nil {
			if v.Cache.Key == "" {
				return fmt.Errorf("[YAML] Cache must have a key defined in block \"%s\"\n", v.Name)
			}
			if len(v.Cache.Paths) == 0 {
				return fmt.Errorf("[YAML] Cache must have at least one path defined in block \"%s\"\n", v.Name)
			}
			if slices.Contains(v.Cache.Paths, "") {
				return fmt.Errorf("[YAML] Cache can't include an empty path in block \"%s\"\n", v.Name)
			}
		}
	}
	return nil
}
