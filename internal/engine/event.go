package engine

import (
	"log"
	"sync"
	"time"
)

// EventType identifies what an Event reports. The engine emits a stream of
// these; sinks decide how to present or persist them.
type EventType int

const (
	RunStarted    EventType = iota // Mode, HasMatrix, HasDetached, Order
	RunFinished                    // Duration, Err
	GroupStarted                   // a concurrent barrier: parallel run / stage / matrix group (GroupKind, GroupLabel, Order)
	GroupFinished                  // GroupKind, GroupLabel
	JobStarted                     // Job, Stage, Exec, GroupID
	JobFinished                    // ExitCode, Duration, Err (also drives the status board)
	LogLine                        // Stream, Data (a chunk of job output)
	Diagnostic                     // Data (a fully-formatted internal log line)
)

// ExecKind reports how a job is being run. It is an execution fact, not a
// presentation hint: the engine says whether a job runs alone, as part of a
// concurrent barrier, or detached, and the sink maps that to streaming vs a
// board vs inline status lines.
type ExecKind int

const (
	Standalone ExecKind = iota // sequential job, streamed on its own
	Concurrent                 // member of a concurrent barrier (a group)
	Detached                   // a `parallel: true` job detached from the chain
)

// GroupKind distinguishes the three flavors of concurrent barrier so the sink
// can render the right header.
type GroupKind int

const (
	GroupParallelAll GroupKind = iota // whole-run parallel mode (no per-group header)
	GroupStage                        // a stage in parallel-stages mode ("Stage: X")
	GroupMatrix                       // a matrix group inside a sequential run ("Matrix [X]")
)

// JobState is the lifecycle state shown on the status board.
type JobState int

const (
	StatePending JobState = iota
	StateRunning
	StatePassed
	StateFailed
)

// StreamKind identifies which output stream a LogLine came from. Today the
// engine merges everything into Stdout (matching the previous behavior);
// the field exists so later phases can split streams without a schema change.
type StreamKind int

const (
	StreamStdout StreamKind = iota
	StreamStderr
)

// Event is a single item in the engine's output stream. Only the fields
// relevant to a given Type are populated.
type Event struct {
	Type  EventType
	Time  time.Time
	RunID string

	// Job-scoped events.
	Job   string
	Stage string
	Exec  ExecKind

	// Group-scoped events.
	GroupID    string
	GroupKind  GroupKind
	GroupLabel string

	// RunStarted.
	Mode        RunMode
	HasMatrix   bool
	HasDetached bool
	ConfigPath  string
	ProjectPath string

	// RunStarted / GroupStarted: job ordering for the board.
	Order []string

	// JobFinished / RunFinished.
	ExitCode int
	Duration time.Duration
	Err      string

	// LogLine / Diagnostic.
	Stream StreamKind
	Data   []byte
}

// Sink consumes events. Implementations must not call back into the Bus they
// are attached to (Emit holds the bus lock while fanning out).
type Sink interface {
	Emit(Event)
}

// Bus fans an event out to every attached sink. Emit is safe to call from
// multiple goroutines (parallel jobs emit concurrently); fan-out is serialized
// so sinks observe a single consistent ordering.
type Bus struct {
	mu    sync.Mutex
	sinks []Sink
}

func NewBus(sinks ...Sink) *Bus {
	return &Bus{sinks: sinks}
}

func (b *Bus) Emit(e Event) {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.sinks {
		s.Emit(e)
	}
}

// logWriter turns each Write into a LogLine event. io.Copy reuses its buffer,
// so the payload is copied before it leaves the writer.
type logWriter struct {
	bus     *Bus
	runID   string
	job     string
	stage   string
	exec    ExecKind
	groupID string
}

func (w *logWriter) Write(p []byte) (int, error) {
	data := make([]byte, len(p))
	copy(data, p)
	w.bus.Emit(Event{
		Type:    LogLine,
		RunID:   w.runID,
		Job:     w.job,
		Stage:   w.stage,
		Exec:    w.exec,
		GroupID: w.groupID,
		Stream:  StreamStdout,
		Data:    data,
	})
	return len(p), nil
}

// diagWriter turns each Write into a Diagnostic event. It is meant to back a
// *log.Logger so the bytes carry the same LstdFlags formatting the engine used
// to write to the global logger.
type diagWriter struct {
	bus   *Bus
	runID string
	job   string
}

func (w *diagWriter) Write(p []byte) (int, error) {
	data := make([]byte, len(p))
	copy(data, p)
	w.bus.Emit(Event{
		Type:  Diagnostic,
		RunID: w.runID,
		Job:   w.job,
		Data:  data,
	})
	return len(p), nil
}

// newDiagLogger builds a logger whose lines are emitted as Diagnostic events.
// LstdFlags matches the format the engine previously produced via the global
// log package.
func newDiagLogger(bus *Bus, runID string) *log.Logger {
	return log.New(&diagWriter{bus: bus, runID: runID}, "", log.LstdFlags)
}
