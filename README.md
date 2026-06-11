# Local CI

Local CI is a tool that allows you to run CI/CD pipelines locally using Docker containers. It helps developers test and debug their CI pipelines without pushing to remote repositories.

## Features

- Run CI pipeline jobs locally using Docker
- Stages-based execution similar to GitLab CI
- Configuration using YAML format
- Global and job-specific environment variables
- Working directory customization
- Cache support for dependencies and build artifacts
- Automatic file copying with .gitignore support
- Real-time log streaming from containers
- Automatic container cleanup
- Stage-based pipeline execution
- Job-based pipeline execution
- Parallel job execution (all jobs at once, or per-stage) with a live status board
- Per-job `parallel: true` keyword for detaching individual jobs from the sequential chain
- Matrix builds: parametrize a job with `matrix:` to fan it out across variable combinations
- Services: sidecar containers (databases, caches) on a private per-job network with readiness gates — `services: [postgres:16]`
- Artifacts: pass build outputs from one job to the next with `artifacts: paths: [...]`
- DAG scheduling: `needs: [job]` starts a job the moment its dependencies pass instead of waiting for the stage barrier
- Per-job `timeout:` and `retry:` for slow or flaky jobs
- Templates and includes: factor common config into `.dot-prefixed` templates and pull shared files with `include:`
- Stage placeholders: splice an included file's stages into the main pipeline at a chosen position
- GitLab utils
- Bootstrap scripts
- Cleanup scripts (companion to bootstrap)
- Per-job bootstrap and cleanup scripts
- Web UI: live pipeline graph, ANSI-colored searchable logs, run control (env vars, per-stage/per-job runs, failed-job re-run), desktop notifications, and a built-in YAML editor with live lint — served from a single binary with `local-ci ui`
- Run history with git context (branch + commit per run), plus per-job duration trends and flaky-job detection; inspect from the UI or `local-ci runs` / `local-ci log`
- Debug shell: `local-ci shell <job>` drops you into the job's exact container environment — image, variables, workspace, cache mounts, and running services
- Watch mode: `local-ci run --watch` re-runs the pipeline on every file change
- Config linting (`local-ci validate`) and a `.gitlab-ci.yml` importer (`local-ci import gitlab`)
- Claude Code agent plugin

## Installation

### Prerequisites

- Docker must be installed and running
- Your user must have permissions to access the Docker daemon
- Verify installation with `docker --version`

### Homebrew (macOS / Linux)
```bash
brew install --cask MrPuls/local-ci/local-ci
```
Shell completions for bash, zsh, and fish are installed automatically.

### Go Install

Requires Go 1.26 or later ([download](https://golang.org/dl/)). The web UI build is
committed, so the installed binary includes `local-ci ui` with no extra tooling.
```bash
go install github.com/MrPuls/local-ci/cmd/local-ci@latest
```

### Manual

Download the latest binary for your platform (Linux, macOS, or Windows) from the
[releases page](https://github.com/MrPuls/local-ci/releases). The archives also
carry completion scripts (`completions/`) for bash, zsh, fish, and PowerShell;
or generate them on the fly with `local-ci completion <shell>`.

### Shell completions

Completions are context-aware: `local-ci shell <TAB>` lists the jobs defined in
your config, `-j`/`-s` complete job and stage names, `local-ci log <TAB>`
completes recent run ids (with their status), and `-c` offers the discovered
config files. Homebrew installs them for you; otherwise see
`local-ci completion --help`.

### Build from source

A `Makefile` wraps the common tasks (run `make help` for the full list). Building
the UI needs [Bun](https://bun.sh); the Go binary needs Go 1.26+.
```bash
make build      # build the web UI + the local-ci binary into ./bin
make install    # build the UI and `go install` the binary
```
After changing anything under `web/`, run `make web` and commit the regenerated
`internal/web/dist` so the embedded UI stays in sync.


## Usage

### Check Installation

```bash
local-ci --version
```

### Run a Pipeline

```bash
# Run using default .local-ci.yaml in current directory
local-ci run

# Run with a specific config file using --config/-c flag
local-ci run --config my-pipeline.yaml

# Run a specific job using --job/-j
local-ci run --job JobName

# Or to run multiple jobs
local-ci run --job jobName1,JobName2

# Run jobs from a specific stage using --stage/-s
local-ci run --stage stageName

# Or use multiple stages
local-ci run --stage stageName1,stageName2

# Run all jobs in parallel with --parallel/-p
local-ci run --parallel

# Or run stages in order, with jobs inside each stage in parallel
local-ci run --parallel-stages

# Clone/update the repository and run it's local-ci.yaml with --remote/-r
local-ci run --remote <repository_url>

# Pass additional environment variables with --env/-e
local-ci run --env NEW_VAR=var_value,SECOND_VAR=new_value
```

When you don't pass `-c/--config`, `run` scans the working directory for config
files — the canonical `.local-ci.yaml`/`.local-ci.yml` plus any
`<name>.local-ci.yaml` / `<name>-local-ci.yaml` / `<name>_local-ci.yaml`
variant — and asks which one to load (Enter picks the first). Non-interactive
sessions never block: a single discovered file is used as-is, anything else
falls back to `.local-ci.yaml`.

### Developer loop

```bash
# Re-run the pipeline on every file change (Ctrl-C to stop)
local-ci run --watch

# Drop into a job's exact container environment to debug it from the inside:
# same image, variables, workdir, cache mounts, copied workspace — and its
# services running on the job network
local-ci shell Test

# Lint the config without running anything (exit 1 when invalid — git-hook friendly)
local-ci validate

# Convert an existing GitLab CI config (prints notes for anything that can't carry over)
local-ci import gitlab
```

### Web UI

`local-ci ui` serves the whole web app — its UI **and** API — from this single
binary, then opens it in your browser. No separate dev server or token to manage.

```bash
# Serve the UI for the project in the current directory and open a browser
local-ci ui

# Bind a specific port and don't auto-open a browser
local-ci ui --port 8080 --no-open

# Use a non-default config file
local-ci ui --config my-pipeline.yaml
```

It binds loopback only (`127.0.0.1`) and shows the configured pipeline as a live
graph: trigger and cancel runs, watch job status update in real time, stream
logs, and browse run history — all backed by the same engine as `local-ci run`.

On load the UI scans the project directory for config files (same patterns as
`run`) and asks which one to drive the session; switch any time via the `FILE:`
chip in the top bar. The **EDITOR** tab is a built-in YAML editor with syntax
highlighting, live lint (undefined stages, unknown `needs` targets, missing
fields — flagged before you save), validation feedback, a diff preview before
discarding edits, and a live pipeline graph — `Ctrl+S` writes to disk.

The pipeline view exposes the full trigger surface: pick a run mode, set env
vars, run a single stage from its column header, or a single job from the
inspector (failed jobs get a one-click re-run). Logs render ANSI colors and
are searchable. History shows each run's git context (`branch@sha`) and a
JOB_TRENDS panel: per-job duration sparklines, pass rates, and an
INTERMITTENT flag for jobs that flip between pass and fail. The bell in the
top bar enables desktop notifications when a run finishes in a hidden tab.

> For **frontend development** against a hot-reloading dev server, use
> `local-ci serve` (the API-only backend) together with the Vite dev server —
> see [web/README.md](web/README.md).

### Inspecting past runs

Every run (CLI or UI) is recorded to a local history store.

```bash
# List recent runs for the current project (newest first)
local-ci runs

# List runs across all projects, capping the count
local-ci runs --all --limit 50

# Show one run's per-job breakdown
local-ci runs <run-id>

# Print a recorded run's logs — all jobs, or one job
local-ci log <run-id>
local-ci log <run-id> --job Build
local-ci log <run-id> --job pipeline   # run-level diagnostics
```

### Command reference

| Command | Description | Flags |
|---|---|---|
| `run` | Run the pipeline | `-c/--config`, `-j/--job`, `-s/--stage`, `-r/--remote`, `-e/--env`, `-p/--parallel`, `--parallel-stages`, `-w/--watch`, `--no-record` |
| `shell <job>` | Open an interactive shell in the job's container environment (services included) | `-c/--config`, `-v/--verbose` |
| `validate [file]` | Lint a config without running it (exit 1 when invalid) | `-c/--config` |
| `import gitlab [file]` | Convert a `.gitlab-ci.yml` into a local-ci config | `-o/--output` (default `.local-ci.yaml`, `-` for stdout), `--force` |
| `runs [run-id]` | List recorded runs, or show one run's details | `-a/--all`, `-n/--limit` (default 20) |
| `log <run-id>` | Print a recorded run's logs | `-j/--job` (use `pipeline` for diagnostics) |
| `ui` | Serve the embedded web UI **and** API from one binary, then open a browser | `--host` (default `127.0.0.1`), `--port` (default `4123`), `-c/--config`, `--no-open` |
| `serve` | Run the API-only backend (for the web dev server or a future desktop shell) | `--host` (default `127.0.0.1`), `--port` (default ephemeral), `--token` (default random), `-c/--config` |

Global: `local-ci --version`, `local-ci --help`, `local-ci <command> --help`.

## Quick Start

1. Start Docker

2. Create a `.local-ci.yaml` file in your project root:

```yaml
stages:
  - build
  - test

variables:
  GLOBAL_VAR: global_value

Build:
  stage: build
  image: golang:1.21
  variables:
    GO_FLAGS: "-v"
  script:
    - echo "Building application..."
    - go build $GO_FLAGS
  cache:
    key: go-build-cache
    paths:
      - .go/
      - build/

Test:
  stage: test
  image: golang:1.21
  script:
    - echo "Testing with global var: $GLOBAL_VAR"
    - go test ./...
  cache:
    key: go-build-cache
    paths:
      - .go/
```

3. Run the pipeline:

```bash
local-ci run
```

## YAML Configuration

### Basic Structure

```yaml
# Define stages and their order
stages:
  - build
  - test
  - deploy

# Global variables (available to all jobs)
variables:
  GLOBAL_KEY: global_value

# Job definitions
job_name:
  stage: build    # Must match one of the defined stages
  image: image_name
  workdir: /path   # Optional
  variables:       # Job-specific variables (override globals)
    KEY: value
  script:
    - command1
    - command2
  cache:           # Optional cache configuration
    key: cache-key
    paths:
      - path/to/cache
      - another/path
```

## Claude Code Agent

Local CI ships as a [Claude Code](https://claude.ai/code) plugin. Once installed, Claude Code can autonomously write pipeline configs, run them, and validate your code in real Docker environments.

Install the plugin

```bash
/plugin marketplace add MrPuls/local-ci

/plugin install local-ci@MrPuls-local-ci
```

See the [Technical Reference](docs/tech-reference.md#claude-code-agent) for details.

## Documentation

- [YAML Configuration Reference](docs/yaml-reference.md)
- [Technical Reference](docs/tech-reference.md)
- [Claude Code Agent](docs/agent.md)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[MIT License](LICENSE)
