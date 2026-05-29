package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MrPuls/local-ci/internal/engine"
	"github.com/MrPuls/local-ci/internal/runmanager"
	"github.com/MrPuls/local-ci/internal/store"
)

const testToken = "secret"

func newTestServer(t *testing.T, runFn runmanager.RunFunc) (*httptest.Server, *runmanager.Manager, *store.Store, string) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	mgr := runmanager.New(st)
	if runFn != nil {
		mgr.SetRunFunc(runFn)
	}
	root := t.TempDir()
	configPath := filepath.Join(root, ".local-ci.yaml")
	ts := httptest.NewServer(New(st, mgr, testToken, "test", configPath).Handler())
	t.Cleanup(func() {
		ts.Close()
		st.Close()
	})
	return ts, mgr, st, root
}

func do(t *testing.T, method, url, body string) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

func fullRun(_ context.Context, runID string, _ engine.Spec, bus *engine.Bus) error {
	bus.Emit(engine.Event{Type: engine.RunStarted, RunID: runID, Mode: engine.ModeSequential, ProjectPath: "/p", ConfigPath: "c"})
	bus.Emit(engine.Event{Type: engine.JobStarted, RunID: runID, Job: "a", Stage: "build", Exec: engine.Standalone})
	bus.Emit(engine.Event{Type: engine.LogLine, RunID: runID, Job: "a", Exec: engine.Standalone, Data: []byte("hi\n")})
	bus.Emit(engine.Event{Type: engine.JobFinished, RunID: runID, Job: "a", Exec: engine.Standalone, Duration: time.Second})
	bus.Emit(engine.Event{Type: engine.RunFinished, RunID: runID, Duration: time.Second})
	return nil
}

func TestAuth(t *testing.T) {
	ts, _, _, _ := newTestServer(t, nil)

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no token: status = %d, want 401", resp.StatusCode)
	}

	resp = do(t, "GET", ts.URL+"/api/health", "")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("with token: status = %d, want 200", resp.StatusCode)
	}
}

func TestTriggerRecordsAndHistory(t *testing.T) {
	ts, mgr, _, _ := newTestServer(t, fullRun)

	resp := do(t, "POST", ts.URL+"/api/runs", `{"mode":"sequential"}`)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("trigger status = %d, want 202", resp.StatusCode)
	}
	var trig struct {
		ID string `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&trig)
	resp.Body.Close()
	if trig.ID == "" {
		t.Fatal("trigger returned empty id")
	}

	waitFinished(t, mgr, trig.ID)

	resp = do(t, "GET", ts.URL+"/api/runs/"+trig.ID, "")
	defer resp.Body.Close()
	var run runJSON
	json.NewDecoder(resp.Body).Decode(&run)
	if run.Status != store.StatusPassed {
		t.Errorf("run status = %q, want passed", run.Status)
	}
	if len(run.Jobs) != 1 || run.Jobs[0].Name != "a" {
		t.Errorf("jobs = %+v, want one job 'a'", run.Jobs)
	}

	listResp := do(t, "GET", ts.URL+"/api/runs?all=true", "")
	defer listResp.Body.Close()
	var list struct {
		Runs []runJSON `json:"runs"`
	}
	json.NewDecoder(listResp.Body).Decode(&list)
	if len(list.Runs) != 1 || list.Runs[0].ID != trig.ID {
		t.Errorf("history = %+v, want the triggered run", list.Runs)
	}
}

func TestSSEReplayFinishedRun(t *testing.T) {
	ts, mgr, _, _ := newTestServer(t, fullRun)
	resp := do(t, "POST", ts.URL+"/api/runs", `{"mode":"sequential"}`)
	var trig struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&trig)
	resp.Body.Close()
	waitFinished(t, mgr, trig.ID)

	ev := do(t, "GET", ts.URL+"/api/runs/"+trig.ID+"/events", "")
	defer ev.Body.Close()
	body, _ := io.ReadAll(ev.Body)
	text := string(body)

	for _, want := range []string{"run_started", "job_started", "log_line", "run_finished"} {
		if !strings.Contains(text, want) {
			t.Errorf("SSE replay missing %q in:\n%s", want, text)
		}
	}
	if !strings.Contains(text, "id: 1\n") {
		t.Errorf("SSE frames missing id lines:\n%s", text)
	}
}

func TestCancel(t *testing.T) {
	started := make(chan struct{})
	ts, _, _, _ := newTestServer(t, func(ctx context.Context, runID string, _ engine.Spec, bus *engine.Bus) error {
		bus.Emit(engine.Event{Type: engine.RunStarted, RunID: runID})
		close(started)
		<-ctx.Done()
		bus.Emit(engine.Event{Type: engine.RunFinished, RunID: runID, Err: "cancelled"})
		return ctx.Err()
	})

	resp := do(t, "POST", ts.URL+"/api/runs", `{}`)
	var trig struct{ ID string }
	json.NewDecoder(resp.Body).Decode(&trig)
	resp.Body.Close()
	<-started

	c := do(t, "POST", ts.URL+"/api/runs/"+trig.ID+"/cancel", "")
	c.Body.Close()
	if c.StatusCode != http.StatusOK {
		t.Errorf("cancel status = %d, want 200", c.StatusCode)
	}

	u := do(t, "POST", ts.URL+"/api/runs/nope/cancel", "")
	u.Body.Close()
	if u.StatusCode != http.StatusNotFound {
		t.Errorf("cancel unknown status = %d, want 404", u.StatusCode)
	}
}

func TestConfigGraph(t *testing.T) {
	ts, _, _, root := newTestServer(t, nil)
	cfgPath := filepath.Join(root, ".local-ci.yaml")
	os.WriteFile(cfgPath, []byte(`stages:
  - build
  - test
Build:
  stage: build
  image: alpine:3.19
  script:
    - echo hi
Test:
  stage: test
  image: alpine:3.19
  script:
    - echo bye
`), 0o644)

	// The config endpoint operates on the server's fixed project config; no
	// path is accepted from the request.
	resp := do(t, "GET", ts.URL+"/api/config", "")
	defer resp.Body.Close()
	var g configGraph
	json.NewDecoder(resp.Body).Decode(&g)
	if !g.Valid {
		t.Fatalf("graph invalid: %v", g.Errors)
	}
	if len(g.Stages) != 2 || g.Stages[0] != "build" {
		t.Errorf("stages = %v", g.Stages)
	}
	if len(g.Jobs) != 2 {
		t.Errorf("jobs = %+v, want 2", g.Jobs)
	}
}

func TestPathTraversalRejected(t *testing.T) {
	ts, _, _, _ := newTestServer(t, nil)

	// The only request-supplied path components are the run id and job name;
	// both are validated as single components, so traversal attempts are 400.
	logCases := []string{
		"/api/runs/good/log?job=" + url.QueryEscape("../../../../etc/passwd"),
		"/api/runs/" + url.QueryEscape("a..b") + "/log?job=build",
	}
	for _, p := range logCases {
		resp := do(t, "GET", ts.URL+p, "")
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("log %q: status = %d, want 400", p, resp.StatusCode)
		}
	}

	ev := do(t, "GET", ts.URL+"/api/runs/"+url.QueryEscape("a..b")+"/events", "")
	ev.Body.Close()
	if ev.StatusCode != http.StatusBadRequest {
		t.Errorf("events traversal id: status = %d, want 400", ev.StatusCode)
	}
}

func waitFinished(t *testing.T, mgr *runmanager.Manager, id string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for mgr.Active(id) {
		if time.Now().After(deadline) {
			t.Fatal("run did not finish in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
