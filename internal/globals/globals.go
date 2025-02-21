package globals

import "github.com/MrPuls/local-ci/internal/config"

type Globals interface {
	GetStages() []string
	GetGlobalVariables() map[string]string
}

type globalsConfig struct {
	config config.Config
}

func (g globalsConfig) GetStages() []string { return g.config.Stages }

func (g globalsConfig) GetGlobalVariables() map[string]string { return g.config.GlobalVariables }

func NewConfigGlobals(cfg config.Config) Globals {
	return &globalsConfig{
		config: cfg,
	}
}
