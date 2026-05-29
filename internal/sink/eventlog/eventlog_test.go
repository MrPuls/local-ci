package eventlog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/MrPuls/local-ci/internal/engine"
)

func TestEventLogWritesNdjsonInOrder(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	events := []engine.Event{
		{Type: engine.RunStarted, RunID: "r1", Seq: 1, Mode: engine.ModeSequential},
		{Type: engine.JobStarted, RunID: "r1", Seq: 2, Job: "a", Exec: engine.Standalone},
		{Type: engine.LogLine, RunID: "r1", Seq: 3, Job: "a", Exec: engine.Standalone, Data: []byte("hello\n")},
		{Type: engine.JobFinished, RunID: "r1", Seq: 4, Job: "a", Exec: engine.Standalone},
		{Type: engine.RunFinished, RunID: "r1", Seq: 5},
	}
	for _, e := range events {
		s.Emit(e)
	}

	f, err := os.Open(filepath.Join(dir, "events.ndjson"))
	if err != nil {
		t.Fatalf("open ndjson: %v", err)
	}
	defer f.Close()

	var got []engine.WireEvent
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var w engine.WireEvent
		if err := json.Unmarshal(sc.Bytes(), &w); err != nil {
			t.Fatalf("unmarshal line %q: %v", sc.Text(), err)
		}
		got = append(got, w)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(got) != len(events) {
		t.Fatalf("got %d lines, want %d", len(got), len(events))
	}
	for i, w := range got {
		if w.Seq != uint64(i+1) {
			t.Errorf("line %d seq = %d, want %d", i, w.Seq, i+1)
		}
	}
	if got[0].Type != "run_started" || got[2].Type != "log_line" || got[2].Data != "hello\n" {
		t.Errorf("unexpected decoded events: %+v", got)
	}

	// After RunFinished the sink should have closed the file (no further writes).
	s.Emit(engine.Event{Type: engine.Diagnostic, Data: []byte("late")})
}
