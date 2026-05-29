package runmanager

import (
	"sync"

	"github.com/MrPuls/local-ci/internal/engine"
)

const subBuffer = 1024

type subscriber struct {
	ch     chan engine.Event
	closed bool
}

// hub fans one run's events out to live subscribers and is itself an
// engine.Sink. Delivery is non-blocking: a subscriber whose buffer fills is
// dropped (channel closed), so a slow SSE client can never stall the engine —
// it reconnects and replays from events.ndjson instead.
type hub struct {
	mu     sync.Mutex
	subs   map[*subscriber]struct{}
	closed bool
}

func newHub() *hub {
	return &hub{subs: make(map[*subscriber]struct{})}
}

// Subscribe registers a live subscriber, returning its channel and an
// unsubscribe func. It returns a nil channel if the run has already finished.
func (h *hub) Subscribe() (<-chan engine.Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, func() {}
	}
	s := &subscriber{ch: make(chan engine.Event, subBuffer)}
	h.subs[s] = struct{}{}
	return s.ch, func() { h.drop(s) }
}

func (h *hub) drop(s *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeLocked(s)
}

// removeLocked deletes s and closes its channel. Caller holds h.mu.
func (h *hub) removeLocked(s *subscriber) {
	if _, ok := h.subs[s]; !ok {
		return
	}
	delete(h.subs, s)
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
}

// Emit delivers an event to every subscriber without blocking.
func (h *hub) Emit(e engine.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		select {
		case s.ch <- e:
		default:
			// Buffer full: drop the laggard.
			h.removeLocked(s)
		}
	}
}

// Close ends the hub: closes all subscriber channels (signaling stream end) and
// rejects future subscribers. Buffered events are still drained by readers
// before they observe the close.
func (h *hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for s := range h.subs {
		h.removeLocked(s)
	}
}
