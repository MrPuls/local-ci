# local-ci web UI

A Vue 3 + TypeScript SPA for local-ci, built with **Bun** + **Vite**. It talks to
a running `local-ci serve` over its HTTP/SSE API and renders the CRT-terminal
design system. This is the reusable UI core; a later Tauri desktop shell would
package this same app and supply the sidecar URL + token at runtime.

Surfaces:

- **Pipeline view** (`/`, `/runs/:id`) — the configured DAG (stages → jobs from
  `GET /api/config`) with live status overlaid from a run, a job inspector, an
  attachable log feed, and run control (trigger with a mode, cancel). Live and
  finished runs are driven by the same SSE stream.
- **Run history** (`/history`) — past runs from `GET /api/runs`; click a run to
  open it in the pipeline view.
- **Tweaks** — phosphor theme (amber/cyan/mono) + CRT effect toggles, persisted
  to `localStorage`.

## Develop

Prerequisites: [Bun](https://bun.sh) (`bun --version`). No npm/Node required.

1. Start the backend in a project that has a `.local-ci.yaml`:

   ```sh
   cd /path/to/your/project
   local-ci serve --port 4123 --token dev
   ```

2. Point the SPA at it and run the dev server:

   ```sh
   cd web
   cp .env.example .env        # set VITE_LCI_PORT / VITE_LCI_TOKEN to match step 1
   bun install
   bun run dev                 # open the printed http://localhost:5173
   ```

   The Vite dev server proxies `/api/*` to the backend and injects the bearer
   token (including on the SSE stream, which can't set headers), so the browser
   stays same-origin (no CORS) and the token never lives in client code.

3. Type-check / build / test:

   ```sh
   bun run typecheck           # vue-tsc
   bun run build               # type-check + production bundle into dist/
   bun run test                # vitest (harness ready; add specs under test/)
   ```

## How it connects

`src/lib/api.ts` calls **relative** `/api/...` URLs against `VITE_LCI_BASE`
(empty in dev/Tauri = same-origin). In browser dev the Vite proxy
(`vite.config.ts`) forwards to `127.0.0.1:$VITE_LCI_PORT` with the token. A
Tauri shell would later set `VITE_LCI_BASE` (+ token handling) to the sidecar.

Types in `src/lib/types.ts` mirror the server JSON (`internal/server/server.go`,
`internal/server/config.go`, `internal/engine/wire.go`) — keep them in sync.

## Layout

```
src/
  lib/          contract types, api client, SSE event folding, formatting,
                status mapping, and the config↔run pipeline merge (pure TS)
  composables/  shared reactive state (settings, toast, health, config, runs,
                live SSE run, ticking clock, run-status bus)
  components/   CRT primitives + panels (graph, inspector, log feed, top/status
                bars, tweaks, toast)
  views/        PipelineView, HistoryView (routed)
  styles/       tokens.css + crt.css (from local-ci-design) + app.css additions
```

## Degraded fields (frontend-only against the current API)

The server's `GET /api/config` exposes a lean graph: job name, stage, image,
`parallel`, `variantCount`, plus stages/includes. The design also depicts
per-job script/variables/rules/artifacts, a host bootstrap phase, and git
branch/commit — **none of which the API returns**. Those are intentionally
omitted or shown as "not exposed by the API"; everything rendered is real run,
job, log, and config-graph data. Surfacing the rest is a backend change for a
later slice.

## Testability

Interactive elements carry stable `data-test-id` attributes (e.g.
`run-pipeline`, `cancel-run`, `mode-select`, `job-card-<name>`, `inspector`,
`log-tab-<job>`, `theme-<name>`, `toggle-<fx>`, `history-row-<id>`) for unit and
end-to-end tests.
