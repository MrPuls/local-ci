package terminal

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/MrPuls/local-ci/internal/engine"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// statusBoard renders a live, repainting list of job statuses to a terminal.
// Once Start is called it owns the cursor region it draws, so nothing else
// should write to the same output until Stop returns. It is a direct port of
// the previous app.StatusBoard, driven by the terminal sink.
type statusBoard struct {
	mu     sync.Mutex
	order  []string
	states map[string]engine.JobState
	out    io.Writer
	stop   chan struct{}
	done   chan struct{}
}

func newStatusBoard(jobNames []string, out io.Writer) *statusBoard {
	states := make(map[string]engine.JobState, len(jobNames))
	for _, n := range jobNames {
		states[n] = engine.StatePending
	}
	return &statusBoard{
		order:  jobNames,
		states: states,
		out:    out,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (b *statusBoard) Update(name string, state engine.JobState) {
	b.mu.Lock()
	if _, ok := b.states[name]; ok {
		b.states[name] = state
	}
	b.mu.Unlock()
}

// Start spawns the renderer goroutine, which repaints on a fixed interval
// until Stop is called.
func (b *statusBoard) Start() {
	go func() {
		defer close(b.done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		frame := 0
		b.render(frame, false)
		for {
			select {
			case <-b.stop:
				b.render(frame, true)
				return
			case <-ticker.C:
				frame++
				b.render(frame, true)
			}
		}
	}()
}

// Stop ends the renderer and blocks until the final frame is painted.
func (b *statusBoard) Stop() {
	close(b.stop)
	<-b.done
}

func (b *statusBoard) render(frame int, repaint bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if repaint && len(b.order) > 0 {
		fmt.Fprintf(b.out, "\033[%dA", len(b.order))
	}

	spin := spinnerFrames[frame%len(spinnerFrames)]
	for _, name := range b.order {
		var sym, label string
		switch b.states[name] {
		case engine.StatePending:
			sym, label = "·", "pending"
		case engine.StateRunning:
			sym, label = spin, "running"
		case engine.StatePassed:
			sym, label = "✓", "passed"
		case engine.StateFailed:
			sym, label = "✗", "failed"
		}
		fmt.Fprintf(b.out, "\033[2K  %s  %-24s  (%s)\n", sym, name, label)
	}
}
