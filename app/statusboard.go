package app

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type JobState int

const (
	StatePending JobState = iota
	StateRunning
	StatePassed
	StateFailed
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StatusBoard renders a live, repainting list of job statuses to a terminal.
// Once Start is called it owns the cursor region it draws, so nothing else
// should write to the same output until Stop returns.
type StatusBoard struct {
	mu     sync.Mutex
	order  []string
	states map[string]JobState
	out    io.Writer
	stop   chan struct{}
	done   chan struct{}
}

func NewStatusBoard(jobNames []string, out io.Writer) *StatusBoard {
	states := make(map[string]JobState, len(jobNames))
	for _, n := range jobNames {
		states[n] = StatePending
	}
	return &StatusBoard{
		order:  jobNames,
		states: states,
		out:    out,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (b *StatusBoard) Update(name string, state JobState) {
	b.mu.Lock()
	if _, ok := b.states[name]; ok {
		b.states[name] = state
	}
	b.mu.Unlock()
}

// Start spawns the renderer goroutine, which repaints on a fixed interval
// until Stop is called.
func (b *StatusBoard) Start() {
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
func (b *StatusBoard) Stop() {
	close(b.stop)
	<-b.done
}

func (b *StatusBoard) render(frame int, repaint bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if repaint && len(b.order) > 0 {
		fmt.Fprintf(b.out, "\033[%dA", len(b.order))
	}

	spin := spinnerFrames[frame%len(spinnerFrames)]
	for _, name := range b.order {
		var sym, label string
		switch b.states[name] {
		case StatePending:
			sym, label = "·", "pending"
		case StateRunning:
			sym, label = spin, "running"
		case StatePassed:
			sym, label = "✓", "passed"
		case StateFailed:
			sym, label = "✗", "failed"
		}
		fmt.Fprintf(b.out, "\033[2K  %s  %-24s  (%s)\n", sym, name, label)
	}
}
