// Package eventlog is a sink that appends the engine's event stream to a
// per-run events.ndjson file (one JSON-encoded WireEvent per line). It is the
// durable, replayable source the server's SSE endpoint reads — including for
// runs that have already finished.
package eventlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MrPuls/local-ci/internal/engine"
)

// Sink writes events to <runDir>/events.ndjson.
type Sink struct {
	f   *os.File
	enc *json.Encoder
}

// New opens (truncating) the run's event log. On failure it returns a no-op
// sink and warns to stderr; persistence must never break a run.
func New(runDir string) *Sink {
	f, err := os.Create(filepath.Join(runDir, "events.ndjson"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "local-ci: eventlog: create events.ndjson: %v\n", err)
		return &Sink{}
	}
	return &Sink{f: f, enc: json.NewEncoder(f)}
}

func (s *Sink) Emit(e engine.Event) {
	if s.enc == nil {
		return
	}
	// Encode writes a single compact line plus a trailing newline.
	if err := s.enc.Encode(engine.ToWire(e)); err != nil {
		fmt.Fprintf(os.Stderr, "local-ci: eventlog: write: %v\n", err)
	}
	if e.Type == engine.RunFinished {
		s.f.Close()
		s.enc = nil
	}
}
