package terminal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrPuls/local-ci/internal/engine"
)

func newTestSink(t *testing.T) (*TerminalSink, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	dir := t.TempDir()
	var out, errb bytes.Buffer
	s := New(&out, &errb)
	s.makeLogDir = func() (string, error) { return dir, nil }
	return s, &out, &errb, dir
}

func TestSequentialStreamsToStdout(t *testing.T) {
	s, out, errb, _ := newTestSink(t)
	s.Emit(engine.Event{Type: engine.RunStarted, Mode: engine.ModeSequential, Order: []string{"a"}})
	s.Emit(engine.Event{Type: engine.JobStarted, Job: "a", Exec: engine.Standalone})
	s.Emit(engine.Event{Type: engine.LogLine, Job: "a", Exec: engine.Standalone, Data: []byte("hello\n")})
	s.Emit(engine.Event{Type: engine.JobFinished, Job: "a", Exec: engine.Standalone})
	s.Emit(engine.Event{Type: engine.RunFinished})

	if out.String() != "hello\n" {
		t.Errorf("stdout = %q, want %q", out.String(), "hello\n")
	}
	if errb.String() != "" {
		t.Errorf("stderr = %q, want empty", errb.String())
	}
}

func TestDetachedStatusLinesAndFile(t *testing.T) {
	s, out, _, dir := newTestSink(t)
	s.Emit(engine.Event{Type: engine.RunStarted, Mode: engine.ModeSequential, HasDetached: true, Order: []string{"d"}})
	s.Emit(engine.Event{Type: engine.JobStarted, Job: "d", Exec: engine.Detached})
	s.Emit(engine.Event{Type: engine.LogLine, Job: "d", Exec: engine.Detached, Data: []byte("dout")})
	s.Emit(engine.Event{Type: engine.JobFinished, Job: "d", Exec: engine.Detached})
	s.Emit(engine.Event{Type: engine.RunFinished})

	dlog := filepath.Join(dir, "d.log")
	want := fmt.Sprintf("Detached jobs will log to %s\n[detached] d: started → %s\n[detached] d: passed\n", dir, dlog)
	if out.String() != want {
		t.Errorf("stdout =\n%q\nwant\n%q", out.String(), want)
	}
	got, _ := os.ReadFile(dlog)
	if string(got) != "dout" {
		t.Errorf("d.log = %q, want %q", got, "dout")
	}
}

func TestDetachedFailureMessage(t *testing.T) {
	s, out, _, dir := newTestSink(t)
	s.Emit(engine.Event{Type: engine.RunStarted, Mode: engine.ModeSequential, HasDetached: true, Order: []string{"d"}})
	s.Emit(engine.Event{Type: engine.JobStarted, Job: "d", Exec: engine.Detached})
	s.Emit(engine.Event{Type: engine.JobFinished, Job: "d", Exec: engine.Detached, Err: "boom"})

	dlog := filepath.Join(dir, "d.log")
	if want := fmt.Sprintf("[detached] d: failed (see %s)\n", dlog); !strings.Contains(out.String(), want) {
		t.Errorf("stdout %q missing %q", out.String(), want)
	}
}

func TestDiagnosticsRouting(t *testing.T) {
	s, _, errb, dir := newTestSink(t)
	s.Emit(engine.Event{Type: engine.Diagnostic, Data: []byte("diag1\n")})
	s.Emit(engine.Event{Type: engine.GroupStarted, GroupKind: engine.GroupParallelAll, Order: []string{"a"}})
	s.Emit(engine.Event{Type: engine.Diagnostic, Data: []byte("diag2\n")})
	s.Emit(engine.Event{Type: engine.GroupFinished, GroupKind: engine.GroupParallelAll})
	s.Emit(engine.Event{Type: engine.Diagnostic, Data: []byte("diag3\n")})
	s.Emit(engine.Event{Type: engine.RunFinished})

	if errb.String() != "diag1\ndiag3\n" {
		t.Errorf("stderr = %q, want %q", errb.String(), "diag1\ndiag3\n")
	}
	plog, _ := os.ReadFile(filepath.Join(dir, "pipeline.log"))
	if string(plog) != "diag2\n" {
		t.Errorf("pipeline.log = %q, want %q", plog, "diag2\n")
	}
}

func TestParallelHeaderAndConcurrentFiles(t *testing.T) {
	s, out, _, dir := newTestSink(t)
	s.Emit(engine.Event{Type: engine.RunStarted, Mode: engine.ModeParallel, Order: []string{"a", "b"}})
	s.Emit(engine.Event{Type: engine.GroupStarted, GroupKind: engine.GroupParallelAll, Order: []string{"a", "b"}})
	s.Emit(engine.Event{Type: engine.JobStarted, Job: "a", Exec: engine.Concurrent})
	s.Emit(engine.Event{Type: engine.JobStarted, Job: "b", Exec: engine.Concurrent})
	s.Emit(engine.Event{Type: engine.LogLine, Job: "a", Exec: engine.Concurrent, Data: []byte("aout")})
	s.Emit(engine.Event{Type: engine.LogLine, Job: "b", Exec: engine.Concurrent, Data: []byte("bout")})
	s.Emit(engine.Event{Type: engine.JobFinished, Job: "a", Exec: engine.Concurrent})
	s.Emit(engine.Event{Type: engine.JobFinished, Job: "b", Exec: engine.Concurrent})
	s.Emit(engine.Event{Type: engine.GroupFinished, GroupKind: engine.GroupParallelAll})
	s.Emit(engine.Event{Type: engine.RunFinished})

	if want := fmt.Sprintf("Running 2 jobs in parallel, logs in %s\n", dir); !strings.Contains(out.String(), want) {
		t.Errorf("stdout %q missing header %q", out.String(), want)
	}
	a, _ := os.ReadFile(filepath.Join(dir, "a.log"))
	b, _ := os.ReadFile(filepath.Join(dir, "b.log"))
	if string(a) != "aout" || string(b) != "bout" {
		t.Errorf("a.log=%q b.log=%q, want aout/bout", a, b)
	}
}

func TestStageAndMatrixHeaders(t *testing.T) {
	s, out, _, _ := newTestSink(t)
	s.Emit(engine.Event{Type: engine.RunStarted, Mode: engine.ModeParallelStages, Order: []string{"a"}})
	s.Emit(engine.Event{Type: engine.GroupStarted, GroupKind: engine.GroupStage, GroupLabel: "build", Order: []string{"a"}})
	s.Emit(engine.Event{Type: engine.GroupFinished, GroupKind: engine.GroupStage, GroupLabel: "build"})
	s.Emit(engine.Event{Type: engine.RunFinished})
	if !strings.Contains(out.String(), "\nStage: build\n") {
		t.Errorf("stdout %q missing stage header", out.String())
	}

	s2, out2, _, _ := newTestSink(t)
	s2.Emit(engine.Event{Type: engine.RunStarted, Mode: engine.ModeSequential, HasMatrix: true, Order: []string{"m_V.1"}})
	s2.Emit(engine.Event{Type: engine.GroupStarted, GroupKind: engine.GroupMatrix, GroupLabel: "m", Order: []string{"m_V.1"}})
	s2.Emit(engine.Event{Type: engine.GroupFinished, GroupKind: engine.GroupMatrix, GroupLabel: "m"})
	s2.Emit(engine.Event{Type: engine.RunFinished})
	if !strings.Contains(out2.String(), "\nMatrix [m]:\n") {
		t.Errorf("stdout %q missing matrix header", out2.String())
	}
}

func TestBoardRender(t *testing.T) {
	var buf bytes.Buffer
	b := newStatusBoard([]string{"a", "b"}, &buf)
	b.states["a"] = engine.StateRunning
	b.states["b"] = engine.StatePassed
	b.render(0, false)

	want := fmt.Sprintf("\033[2K  %s  %-24s  (%s)\n", spinnerFrames[0], "a", "running") +
		fmt.Sprintf("\033[2K  %s  %-24s  (%s)\n", "✓", "b", "passed")
	if buf.String() != want {
		t.Errorf("render =\n%q\nwant\n%q", buf.String(), want)
	}
}
