package globals

import "github.com/MrPuls/local-ci/internal/config"

// Stages defines the interface for accessing pipeline stages
type Stages interface {
	GetStages() []string
}

// Variables defines the interface for accessing global variables
type Variables interface {
	GetGlobalVariables() map[string]string
}

// configStages implements the Stages interface
type configStages struct {
	config *config.Config
}

// configVariables implements the Variables interface
type configVariables struct {
	config *config.Config
}

func (g *configStages) GetStages() []string {
	return g.config.Stages
}

func (g *configVariables) GetGlobalVariables() map[string]string {
	return g.config.GlobalVariables
}

// NewStages creates a new Stages implementation
func NewStages(cfg *config.Config) Stages {
	return &configStages{
		config: cfg,
	}
}

// NewVariables creates a new Variables implementation
func NewVariables(cfg *config.Config) Variables {
	return &configVariables{
		config: cfg,
	}
}
