package engine

import "time"

// WireEvent is the JSON representation of an Event, shared by the event-log
// sink and the server's SSE stream so both emit an identical, self-describing
// shape (string-named enums, type-specific fields only). It is the contract the
// UI consumes.
type WireEvent struct {
	Seq         uint64    `json:"seq"`
	Type        string    `json:"type"`
	Time        time.Time `json:"time"`
	RunID       string    `json:"runId,omitempty"`
	Job         string    `json:"job,omitempty"`
	Stage       string    `json:"stage,omitempty"`
	Exec        string    `json:"exec,omitempty"`
	GroupID     string    `json:"groupId,omitempty"`
	GroupKind   string    `json:"groupKind,omitempty"`
	GroupLabel  string    `json:"groupLabel,omitempty"`
	Mode        string    `json:"mode,omitempty"`
	HasMatrix   bool      `json:"hasMatrix,omitempty"`
	HasDetached bool      `json:"hasDetached,omitempty"`
	Order       []string  `json:"order,omitempty"`
	ConfigPath  string    `json:"configPath,omitempty"`
	ProjectPath string    `json:"projectPath,omitempty"`
	Commit      string    `json:"commit,omitempty"`
	Branch      string    `json:"branch,omitempty"`
	ExitCode    int       `json:"exitCode,omitempty"`
	DurationMs  int64     `json:"durationMs,omitempty"`
	Err         string    `json:"err,omitempty"`
	Stream      string    `json:"stream,omitempty"`
	Data        string    `json:"data,omitempty"`
}

// ToWire converts an Event to its JSON wire form, populating only the fields
// meaningful for the event's type.
func ToWire(e Event) WireEvent {
	w := WireEvent{
		Seq:   e.Seq,
		Type:  eventTypeString(e.Type),
		Time:  e.Time,
		RunID: e.RunID,
	}
	switch e.Type {
	case RunStarted:
		w.Mode = modeString(e.Mode)
		w.HasMatrix = e.HasMatrix
		w.HasDetached = e.HasDetached
		w.Order = e.Order
		w.ConfigPath = e.ConfigPath
		w.ProjectPath = e.ProjectPath
		w.Commit = e.Commit
		w.Branch = e.Branch
	case RunFinished:
		w.DurationMs = e.Duration.Milliseconds()
		w.Err = e.Err
	case GroupStarted, GroupFinished:
		w.GroupKind = groupKindString(e.GroupKind)
		w.GroupLabel = e.GroupLabel
		w.GroupID = e.GroupID
		w.Order = e.Order
	case JobStarted:
		w.Job = e.Job
		w.Stage = e.Stage
		w.Exec = execKindString(e.Exec)
		w.GroupID = e.GroupID
	case JobFinished:
		w.Job = e.Job
		w.Stage = e.Stage
		w.Exec = execKindString(e.Exec)
		w.GroupID = e.GroupID
		w.ExitCode = e.ExitCode
		w.DurationMs = e.Duration.Milliseconds()
		w.Err = e.Err
	case LogLine:
		w.Job = e.Job
		w.Exec = execKindString(e.Exec)
		w.Stream = streamString(e.Stream)
		w.Data = string(e.Data)
	case Diagnostic:
		w.Data = string(e.Data)
	}
	return w
}

func eventTypeString(t EventType) string {
	switch t {
	case RunStarted:
		return "run_started"
	case RunFinished:
		return "run_finished"
	case GroupStarted:
		return "group_started"
	case GroupFinished:
		return "group_finished"
	case JobStarted:
		return "job_started"
	case JobFinished:
		return "job_finished"
	case LogLine:
		return "log_line"
	case Diagnostic:
		return "diagnostic"
	default:
		return "unknown"
	}
}

func modeString(m RunMode) string {
	switch m {
	case ModeParallel:
		return "parallel"
	case ModeParallelStages:
		return "parallel-stages"
	default:
		return "sequential"
	}
}

func execKindString(k ExecKind) string {
	switch k {
	case Concurrent:
		return "concurrent"
	case Detached:
		return "detached"
	default:
		return "standalone"
	}
}

func groupKindString(k GroupKind) string {
	switch k {
	case GroupStage:
		return "stage"
	case GroupMatrix:
		return "matrix"
	default:
		return "parallel"
	}
}

func streamString(s StreamKind) string {
	if s == StreamStderr {
		return "stderr"
	}
	return "stdout"
}
