package config

import (
	"fmt"
	"slices"
	"strings"
)

type ConfigValidator struct {
	cfg *Config
}

func NewConfigValidator(cfg *Config) *ConfigValidator {
	return &ConfigValidator{cfg: cfg}
}

func ValidateConfig(cfg *Config) error {
	return NewConfigValidator(cfg).Validate()
}

func (v *ConfigValidator) Validate() error {
	checks := []func() error{
		v.validateRemoteProvider,
		v.validateBootstrap,
		v.validateCleanup,
		v.validateStages,
		v.validateJobs,
		v.validateNeeds,
	}

	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}

	return nil
}

func (v *ConfigValidator) validateRemoteProvider() error {
	if v.cfg.RemoteProvider == nil {
		return nil
	}
	if v.cfg.RemoteProvider.Url == "" {
		return fmt.Errorf("[YAML] %s config file has an empty remote provider's url. Either specify a valid url or remove the 'remote_provider' field.", v.cfg.FileName)
	}
	if v.cfg.RemoteProvider.ProjectId == 0 {
		return fmt.Errorf("[YAML] %s config file has an empty remote provider's project_id. Either specify a valid project_id or remove the 'remote_provider' field.", v.cfg.FileName)
	}
	if v.cfg.RemoteProvider.Token == "" {
		return fmt.Errorf("[YAML] %s config file has an empty remote provider's token. Either specify a valid token or remove the 'remote_provider' field.", v.cfg.FileName)
	}
	return nil
}

func (v *ConfigValidator) validateBootstrap() error {
	if v.cfg.Bootstrap == nil {
		return nil
	}
	if len(v.cfg.Bootstrap.Run) == 0 {
		return fmt.Errorf("[YAML] %s config file has an empty bootstrap 'run' field. "+
			"Please add at least one script to run.", v.cfg.FileName)
	}
	if v.cfg.Bootstrap.Timeout < 0 {
		return fmt.Errorf("[YAML] %s config file has a negative bootstrap 'timeout' field. "+
			"Please set a positive timeout value.", v.cfg.FileName)
	}
	return nil
}

func (v *ConfigValidator) validateCleanup() error {
	if v.cfg.Cleanup == nil {
		return nil
	}
	if v.cfg.Bootstrap == nil {
		return fmt.Errorf("[YAML] %s config file has a cleanup block without a bootstrap block. "+
			"Cleanup requires bootstrap to be defined.", v.cfg.FileName)
	}
	if len(v.cfg.Cleanup.Run) == 0 {
		return fmt.Errorf("[YAML] %s config file has an empty cleanup 'run' field. "+
			"Please add at least one script to run.", v.cfg.FileName)
	}
	if v.cfg.Cleanup.Timeout < 0 {
		return fmt.Errorf("[YAML] %s config file has a negative cleanup 'timeout' field. "+
			"Please set a positive timeout value.", v.cfg.FileName)
	}
	return nil
}

func (v *ConfigValidator) validateStages() error {
	if len(v.cfg.Stages) == 0 {
		return fmt.Errorf("[YAML] %s config file has no stages defined: %v. "+
			"Please add at least one stage."+
			"\nExample:\n\nstages:\n  - foo <- stage name goes here\n", v.cfg.FileName, v.cfg.Stages)
	}
	return nil
}

func (v *ConfigValidator) validateJobs() error {
	for _, job := range v.cfg.Jobs {
		if err := v.validateJob(&job); err != nil {
			return err
		}
	}
	return nil
}

func (v *ConfigValidator) validateJob(job *JobConfig) error {
	if job.Stage == "" {
		return fmt.Errorf("[YAML] Stage is empty or undefined in block \"%s\"", job.Name)
	}
	if !slices.Contains(v.cfg.Stages, job.Stage) {
		return fmt.Errorf(
			"[YAML] \"%s\" block uses undefined step: \"%s\"! Available stages are: %v",
			job.Name, job.Stage, v.cfg.Stages,
		)
	}
	if len(job.Script) == 0 {
		return fmt.Errorf(
			"[YAML] \"%s\" block has no scripts defined. "+
				"Please make sure that you are using the 'script' keyword\n"+
				"If you do, please add at least one script."+
				"\nExample:\n\nscript:\n  - echo \"Hello World!\" <- script code goes here\n", job.Name,
		)
	}
	if job.Image == "" {
		return fmt.Errorf("[YAML] Image is empty or undefined in block \"%s\"\n", job.Name)
	}
	if job.JobBootstrap != nil {
		if len(job.JobBootstrap.Run) == 0 {
			return fmt.Errorf("[YAML] \"%s\" block has an empty job_bootstrap 'run' field. "+
				"Please add at least one script to run.", job.Name)
		}
		if job.JobBootstrap.Timeout < 0 {
			return fmt.Errorf("[YAML] \"%s\" block has a negative job_bootstrap 'timeout' field. "+
				"Please set a positive timeout value.", job.Name)
		}
	}
	if job.JobCleanup != nil {
		if job.JobBootstrap == nil {
			return fmt.Errorf("[YAML] \"%s\" block has a job_cleanup without a job_bootstrap. "+
				"Job cleanup requires job bootstrap to be defined.", job.Name)
		}
		if len(job.JobCleanup.Run) == 0 {
			return fmt.Errorf("[YAML] \"%s\" block has an empty job_cleanup 'run' field. "+
				"Please add at least one script to run.", job.Name)
		}
		if job.JobCleanup.Timeout < 0 {
			return fmt.Errorf("[YAML] \"%s\" block has a negative job_cleanup 'timeout' field. "+
				"Please set a positive timeout value.", job.Name)
		}
	}
	if job.Cache != nil {
		if job.Cache.Key == "" {
			return fmt.Errorf("[YAML] Cache must have a key defined in block \"%s\"\n", job.Name)
		}
		if len(job.Cache.Paths) == 0 {
			return fmt.Errorf("[YAML] Cache must have at least one path defined in block \"%s\"\n", job.Name)
		}
		if slices.Contains(job.Cache.Paths, "") {
			return fmt.Errorf("[YAML] Cache can't include an empty path in block \"%s\"\n", job.Name)
		}
	}
	if job.Timeout < 0 {
		return fmt.Errorf("[YAML] \"%s\" block has a negative 'timeout'. "+
			"Use a duration like \"90s\" or \"10m\".", job.Name)
	}
	if job.Retry < 0 || job.Retry > 10 {
		return fmt.Errorf("[YAML] \"%s\" block has 'retry: %d'; retry must be between 0 and 10.",
			job.Name, job.Retry)
	}
	if err := v.validateServices(job); err != nil {
		return err
	}
	if job.Artifacts != nil {
		if len(job.Artifacts.Paths) == 0 {
			return fmt.Errorf("[YAML] \"%s\" block has an artifacts section with no 'paths'. "+
				"Please list at least one path.", job.Name)
		}
		for _, p := range job.Artifacts.Paths {
			if p == "" {
				return fmt.Errorf("[YAML] \"%s\" block has an empty artifacts path.", job.Name)
			}
			if strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
				return fmt.Errorf("[YAML] \"%s\" artifacts path %q must be relative to the job's workdir "+
					"and must not contain '..'.", job.Name, p)
			}
		}
	}
	return nil
}

func (v *ConfigValidator) validateServices(job *JobConfig) error {
	if len(job.Services) == 0 {
		return nil
	}
	if job.Network != nil && job.Network.HostMode {
		return fmt.Errorf("[YAML] \"%s\" block combines 'services' with 'network.host_mode'. "+
			"Services run on a per-job network, which host mode bypasses — remove one of the two.", job.Name)
	}
	aliases := make(map[string]bool, len(job.Services))
	for i, svc := range job.Services {
		if svc.Image == "" {
			return fmt.Errorf("[YAML] \"%s\" block: service %d has no image.", job.Name, i+1)
		}
		alias := svc.EffectiveAlias()
		if alias == "" || !matrixSafeRe.MatchString(alias) {
			return fmt.Errorf("[YAML] \"%s\" block: service %q resolves to invalid alias %q "+
				"(allowed: a-zA-Z0-9_.-); set an explicit 'alias'.", job.Name, svc.Image, alias)
		}
		if aliases[alias] {
			return fmt.Errorf("[YAML] \"%s\" block: duplicate service alias %q; "+
				"set distinct 'alias' values.", job.Name, alias)
		}
		aliases[alias] = true
		if svc.Ready != nil && svc.Ready.Timeout < 0 {
			return fmt.Errorf("[YAML] \"%s\" block: service %q has a negative ready timeout.", job.Name, alias)
		}
	}
	return nil
}

// validateNeeds checks the needs graph as a whole: every reference must name
// an existing job (not itself) in the same or an earlier stage, and the graph
// must be acyclic.
func (v *ConfigValidator) validateNeeds() error {
	byName := make(map[string]*JobConfig, len(v.cfg.Jobs))
	for i := range v.cfg.Jobs {
		byName[v.cfg.Jobs[i].Name] = &v.cfg.Jobs[i]
	}
	stageIdx := make(map[string]int, len(v.cfg.Stages))
	for i, s := range v.cfg.Stages {
		stageIdx[s] = i
	}

	for _, job := range v.cfg.Jobs {
		for _, need := range job.Needs {
			dep, ok := byName[need]
			if !ok {
				return fmt.Errorf("[YAML] \"%s\" block needs unknown job %q.", job.Name, need)
			}
			if need == job.Name {
				return fmt.Errorf("[YAML] \"%s\" block needs itself.", job.Name)
			}
			if stageIdx[dep.Stage] > stageIdx[job.Stage] {
				return fmt.Errorf("[YAML] \"%s\" (stage %q) needs %q from later stage %q; "+
					"needs may only reference the same or an earlier stage.", job.Name, job.Stage, need, dep.Stage)
			}
		}
	}

	// Cycle detection over the needs edges (DFS, three colors).
	const (
		white = iota
		grey
		black
	)
	color := make(map[string]int, len(v.cfg.Jobs))
	var visit func(name string) error
	visit = func(name string) error {
		color[name] = grey
		for _, need := range byName[name].Needs {
			switch color[need] {
			case grey:
				return fmt.Errorf("[YAML] 'needs' cycle detected involving %q and %q.", name, need)
			case white:
				if err := visit(need); err != nil {
					return err
				}
			}
		}
		color[name] = black
		return nil
	}
	for name := range byName {
		if color[name] == white {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}
