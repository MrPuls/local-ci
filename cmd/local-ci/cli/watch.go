package cli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/MrPuls/local-ci/internal/archive"
	"github.com/fsnotify/fsnotify"
)

// debounceWindow batches the burst of events a single save produces (editors
// write temp files, rename, chmod) into one re-run trigger.
const debounceWindow = 400 * time.Millisecond

// projectWatcher watches the project tree recursively, honoring the same
// ignore rules as the workspace tar, and coalesces raw fsnotify events into
// single "something changed" signals on C.
type projectWatcher struct {
	w       *fsnotify.Watcher
	ignored func(name string) bool
	root    string
	// C receives the path of (one of) the changed file(s). Buffered with one
	// slot: changes that land while a pipeline is running keep exactly one
	// pending trigger instead of piling up.
	C chan string
}

func newProjectWatcher(root string) (*projectWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	pw := &projectWatcher{
		w:       w,
		ignored: archive.IgnoreMatcher(root),
		root:    root,
		C:       make(chan string, 1),
	}
	if err := pw.addRecursive(root); err != nil {
		w.Close()
		return nil, err
	}
	go pw.loop()
	return pw, nil
}

func (pw *projectWatcher) Close() error { return pw.w.Close() }

func (pw *projectWatcher) addRecursive(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil //nolint:nilerr // unreadable subtrees are just not watched
		}
		if path != pw.root && pw.ignored(d.Name()) {
			return filepath.SkipDir
		}
		return pw.w.Add(path)
	})
}

func (pw *projectWatcher) loop() {
	for ev := range pw.w.Events {
		if pw.ignored(filepath.Base(ev.Name)) {
			continue
		}
		// New directories must be picked up so files created inside them
		// keep triggering.
		if ev.Has(fsnotify.Create) {
			if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
				_ = pw.addRecursive(ev.Name)
			}
		}
		if ev.Has(fsnotify.Chmod) {
			continue
		}
		select {
		case pw.C <- ev.Name:
		default: // a trigger is already pending
		}
	}
}

// watchLoop runs the pipeline, then blocks until project files change and
// runs it again, until ctx is cancelled (Ctrl-C). Failures don't stop the
// loop — the next save gets a fresh run.
func watchLoop(ctx context.Context, runOnce func(context.Context) error) error {
	pw, err := newProjectWatcher(".")
	if err != nil {
		return fmt.Errorf("start file watcher: %w", err)
	}
	defer pw.Close()

	for {
		if err := runOnce(ctx); err != nil {
			fmt.Printf("\nPipeline failed: %v\n", err)
		}
		if ctx.Err() != nil {
			return nil
		}
		fmt.Println("\n[watch] Waiting for changes... (Ctrl-C to stop)")

		var changed string
		select {
		case <-ctx.Done():
			return nil
		case changed = <-pw.C:
		}
		// Debounce the save burst, keeping the latest name for the banner.
		timer := time.NewTimer(debounceWindow)
	drain:
		for {
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case changed = <-pw.C:
				timer.Reset(debounceWindow)
			case <-timer.C:
				break drain
			}
		}
		fmt.Printf("[watch] Change detected: %s — re-running\n\n", changed)
	}
}
