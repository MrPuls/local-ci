package engine

// RunMode selects how the prepared jobs are scheduled.
type RunMode int

const (
	ModeSequential RunMode = iota
	ModeParallel
	ModeParallelStages
)

// Spec describes a single pipeline run request. It is the engine's input,
// built by a front-end (the CLI today, the server later) from its own flags or
// API payload.
type Spec struct {
	ConfigFile string
	JobNames   []string
	Stages     []string
	Remote     string
	Env        []string
	Mode       RunMode
}
