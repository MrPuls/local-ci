# local-ci web UI

A Svelte SPA for local-ci. It talks to a running `local-ci serve` over its HTTP
API. This is the reusable UI core; the Tauri desktop shell (later) just packages
this same app and supplies the sidecar URL + token at runtime.

Phase 3a is read-only run history: run list → run detail → per-job logs.

## Develop

Prerequisites: Node 18+ and a package manager (`corepack enable` provides npm/pnpm).
Rust/Tauri are **not** needed for the browser SPA.

1. Start the backend in a project that has recorded runs:

   ```sh
   cd /path/to/your/project
   local-ci serve --port 4123 --token dev
   ```

2. Point the SPA at it and run the dev server:

   ```sh
   cd web
   cp .env.example .env        # set VITE_LCI_PORT / VITE_LCI_TOKEN to match step 1
   npm install
   npm run dev                 # open the printed http://localhost:5173
   ```

   The Vite dev server proxies `/api/*` to the backend and injects the bearer
   token, so the browser stays same-origin (no CORS) and the token never lives
   in client code.

3. Type-check:

   ```sh
   npm run check               # svelte-check
   ```

## How it connects

`src/lib/api.ts` calls **relative** `/api/...` URLs against `VITE_LCI_BASE`
(empty in dev/Tauri = same-origin). In browser dev the Vite proxy
(`vite.config.ts`) forwards to `127.0.0.1:$VITE_LCI_PORT` with the token. The
Tauri shell will later set `VITE_LCI_BASE` (+ token handling) to the sidecar.

Types in `src/lib/types.ts` mirror the server JSON (`internal/server/server.go`,
`internal/server/config.go`, `internal/engine/wire.go`) — keep them in sync.
