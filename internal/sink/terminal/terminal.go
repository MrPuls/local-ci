// Package terminal renders the engine's event stream to a terminal, exactly
// reproducing the output the CLI produced before the engine/sink split:
// streamed logs for sequential jobs, a repainting status board for concurrent
// groups, "[detached]" status lines, and per-job log files under
// .local-ci/logs/<timestamp>/.
package terminal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/MrPuls/local-ci/internal/engine"
	"github.com/MrPuls/local-ci/internal/integrations/fs"
)

// TerminalSink consumes engine events and owns all terminal presentation. Its
// Emit is only ever called from Bus.Emit (serialized), so it carries no locks
// of its own; the status board manages its own goroutine and lock.
type TerminalSink struct {
	stdout io.Writer
	stderr io.Writer

	// makeLogDir is the run-log-directory factory; overridable in tests.
	makeLogDir func() (string, error)

	mode        engine.RunMode
	hasMatrix   bool
	hasDetached bool

	logDir    string
	logDirErr error

	jobFiles    map[string]*os.File
	board       *statusBoard
	boardActive bool
	pipelineLog *os.File
}

// New returns a TerminalSink writing job output and chrome to stdout and
// diagnostics to stderr (the destinations the CLI used).
func New(stdout, stderr io.Writer) *TerminalSink {
	return &TerminalSink{
		stdout:     stdout,
		stderr:     stderr,
		makeLogDir: fs.MakeRunLogDir,
		jobFiles:   make(map[string]*os.File),
	}
}

func (s *TerminalSink) Emit(e engine.Event) {
	switch e.Type {
	case engine.RunStarted:
		s.onRunStarted(e)
	case engine.GroupStarted:
		s.onGroupStarted(e)
	case engine.GroupFinished:
		s.onGroupFinished(e)
	case engine.JobStarted:
		s.onJobStarted(e)
	case engine.JobFinished:
		s.onJobFinished(e)
	case engine.LogLine:
		s.onLogLine(e)
	case engine.Diagnostic:
		s.onDiagnostic(e)
	case engine.RunFinished:
		s.onRunFinished(e)
	}
}

func (s *TerminalSink) onRunStarted(e engine.Event) {
	s.mode = e.Mode
	s.hasMatrix = e.HasMatrix
	s.hasDetached = e.HasDetached

	switch {
	case e.Mode == engine.ModeParallel:
		fmt.Fprintf(s.stdout, "Running %d jobs in parallel, logs in %s\n", len(e.Order), s.ensureLogDir())
	case e.Mode == engine.ModeParallelStages:
		fmt.Fprintf(s.stdout, "Running jobs by stage in parallel, logs in %s\n", s.ensureLogDir())
	case e.Mode == engine.ModeSequential && e.HasDetached:
		// Detached takes precedence: the previous runSequentialWithDetached
		// path printed only this header even when matrix variants were present.
		fmt.Fprintf(s.stdout, "Detached jobs will log to %s\n", s.ensureLogDir())
	case e.Mode == engine.ModeSequential && e.HasMatrix:
		fmt.Fprintf(s.stdout, "Matrix variants will log to %s\n", s.ensureLogDir())
	}
}

func (s *TerminalSink) onGroupStarted(e engine.Event) {
	switch e.GroupKind {
	case engine.GroupStage:
		fmt.Fprintf(s.stdout, "\nStage: %s\n", e.GroupLabel)
	case engine.GroupMatrix:
		fmt.Fprintf(s.stdout, "\nMatrix [%s]:\n", e.GroupLabel)
	}
	s.ensurePipelineLog()
	s.board = newStatusBoard(e.Order, s.stdout)
	s.boardActive = true
	s.board.Start()
}

func (s *TerminalSink) onGroupFinished(engine.Event) {
	if s.board != nil {
		s.board.Stop()
		s.board = nil
	}
	s.boardActive = false
}

func (s *TerminalSink) onJobStarted(e engine.Event) {
	switch e.Exec {
	case engine.Detached:
		s.openJobFile(e.Job)
		fmt.Fprintf(s.stdout, "[detached] %s: started → %s\n", e.Job, s.jobLogPath(e.Job))
	case engine.Concurrent:
		s.openJobFile(e.Job)
		if s.board != nil {
			s.board.Update(e.Job, engine.StateRunning)
		}
	}
}

func (s *TerminalSink) onJobFinished(e engine.Event) {
	if e.Exec == engine.Concurrent && s.board != nil {
		if e.Err == "" {
			s.board.Update(e.Job, engine.StatePassed)
		} else {
			s.board.Update(e.Job, engine.StateFailed)
		}
	}
	if e.Exec == engine.Detached {
		if e.Err == "" {
			fmt.Fprintf(s.stdout, "[detached] %s: passed\n", e.Job)
		} else {
			fmt.Fprintf(s.stdout, "[detached] %s: failed (see %s)\n", e.Job, s.jobLogPath(e.Job))
		}
	}
	if f, ok := s.jobFiles[e.Job]; ok {
		f.Close()
		delete(s.jobFiles, e.Job)
	}
}

func (s *TerminalSink) onLogLine(e engine.Event) {
	switch e.Exec {
	case engine.Standalone:
		// Standalone job output and run-level notices stream to stdout.
		s.stdout.Write(e.Data)
	case engine.Concurrent, engine.Detached:
		if f := s.openJobFile(e.Job); f != nil {
			f.Write(e.Data)
		}
	}
}

func (s *TerminalSink) onDiagnostic(e engine.Event) {
	if s.boardActive {
		if f := s.ensurePipelineLog(); f != nil {
			f.Write(e.Data)
			return
		}
	}
	s.stderr.Write(e.Data)
}

func (s *TerminalSink) onRunFinished(engine.Event) {
	if s.board != nil {
		s.board.Stop()
		s.board = nil
	}
	for name, f := range s.jobFiles {
		f.Close()
		delete(s.jobFiles, name)
	}
	if s.pipelineLog != nil {
		s.pipelineLog.Close()
		s.pipelineLog = nil
	}
}

func (s *TerminalSink) ensureLogDir() string {
	if s.logDir != "" || s.logDirErr != nil {
		return s.logDir
	}
	dir, err := s.makeLogDir()
	if err != nil {
		s.logDirErr = err
		fmt.Fprintf(s.stderr, "failed to create run log dir: %v\n", err)
		return ""
	}
	s.logDir = dir
	return dir
}

func (s *TerminalSink) jobLogPath(job string) string {
	if s.logDir == "" {
		return job + ".log"
	}
	return filepath.Join(s.logDir, job+".log")
}

func (s *TerminalSink) openJobFile(job string) *os.File {
	if f, ok := s.jobFiles[job]; ok {
		return f
	}
	if s.ensureLogDir() == "" {
		return nil
	}
	f, err := os.Create(s.jobLogPath(job))
	if err != nil {
		fmt.Fprintf(s.stderr, "failed to create log file for %s: %v\n", job, err)
		return nil
	}
	s.jobFiles[job] = f
	return f
}

func (s *TerminalSink) ensurePipelineLog() *os.File {
	if s.pipelineLog != nil {
		return s.pipelineLog
	}
	if s.ensureLogDir() == "" {
		return nil
	}
	f, err := os.Create(filepath.Join(s.logDir, "pipeline.log"))
	if err != nil {
		fmt.Fprintf(s.stderr, "failed to create pipeline.log: %v\n", err)
		return nil
	}
	s.pipelineLog = f
	return f
}
