package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/MrPuls/local-ci/internal/engine"
	"github.com/MrPuls/local-ci/internal/store"
)

// maxEventLine bounds a single events.ndjson line (a log chunk can be large).
const maxEventLine = 8 << 20 // 8 MiB

// handleEvents streams a run's events as SSE. It replays from events.ndjson and,
// if the run is still active, continues live from the manager hub. Subscribing
// before reading the file (and deduping on Seq) makes the handoff gap-free; a
// Last-Event-ID lets a reconnecting client resume after the last seq it saw.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	// Subscribe to live first so events emitted while we replay the file are
	// buffered and not lost.
	liveCh, unsub, active := s.manager.Subscribe(id)
	if active {
		defer unsub()
	} else if _, err := s.store.GetRun(id); errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	afterSeq := lastEventID(r)
	maxSeq, err := s.replayEventFile(w, flusher, id, afterSeq)
	if err != nil {
		return
	}

	if !active {
		return // finished run: the file was the whole stream
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case e, open := <-liveCh:
			if !open {
				return // run finished (or this slow client was dropped)
			}
			if e.Seq <= maxSeq {
				continue // already delivered from the file
			}
			data, mErr := json.Marshal(engine.ToWire(e))
			if mErr != nil {
				continue
			}
			if _, wErr := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", e.Seq, data); wErr != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// replayEventFile writes every events.ndjson line with seq > afterSeq as an SSE
// frame and returns the highest seq seen. A missing file (run just started)
// yields afterSeq with no error.
func (s *Server) replayEventFile(w http.ResponseWriter, flusher http.Flusher, runID string, afterSeq uint64) (uint64, error) {
	path := filepath.Join(s.store.RunDir(runID), "events.ndjson")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return afterSeq, nil
		}
		return afterSeq, err
	}
	defer f.Close()

	maxSeq := afterSeq
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), maxEventLine)
	for sc.Scan() {
		line := sc.Bytes()
		var probe struct {
			Seq uint64 `json:"seq"`
		}
		if json.Unmarshal(line, &probe) != nil || probe.Seq <= afterSeq {
			continue
		}
		if _, err := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", probe.Seq, line); err != nil {
			return maxSeq, err
		}
		if probe.Seq > maxSeq {
			maxSeq = probe.Seq
		}
	}
	flusher.Flush()
	return maxSeq, sc.Err()
}

// lastEventID reads the resume point from the SSE Last-Event-ID header (set by
// EventSource on reconnect) or a ?lastEventId= query fallback.
func lastEventID(r *http.Request) uint64 {
	v := r.Header.Get("Last-Event-ID")
	if v == "" {
		v = r.URL.Query().Get("lastEventId")
	}
	n, _ := strconv.ParseUint(v, 10, 64)
	return n
}
